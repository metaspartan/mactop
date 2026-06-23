package app

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/ps/IOPowerSources.h>
#include <IOKit/ps/IOPSKeys.h>
#include <stdlib.h>
#include <string.h>

// Reads Amperage (mA, signed) and Voltage (mV) from the AppleSmartBattery IOKit
// registry entry, which is more reliably populated than the IOPowerSources dict.
// Returns the product in milliwatts (positive = charging, negative = discharging),
// or 0 if the service or properties are unavailable.
static int mactop_battery_power_mw(void) {
    io_service_t svc = IOServiceGetMatchingService(kIOMainPortDefault,
                           IOServiceNameMatching("AppleSmartBattery"));
    if (!svc) return 0;

    int result = 0;
    CFNumberRef ampRef  = IORegistryEntryCreateCFProperty(svc, CFSTR("Amperage"),
                                                          kCFAllocatorDefault, 0);
    CFNumberRef voltRef = IORegistryEntryCreateCFProperty(svc, CFSTR("Voltage"),
                                                          kCFAllocatorDefault, 0);
    if (ampRef && voltRef) {
        int64_t amp_ma = 0;
        int32_t volt_mv = 0;
        CFNumberGetValue(ampRef,  kCFNumberSInt64Type, &amp_ma);
        CFNumberGetValue(voltRef, kCFNumberSInt32Type, &volt_mv);
        // mA * mV = µW; divide by 1000 to get mW
        result = (int)((amp_ma * (int64_t)volt_mv) / 1000LL);
    }
    if (ampRef)  CFRelease(ampRef);
    if (voltRef) CFRelease(voltRef);
    IOObjectRelease(svc);
    return result;
}

// Returns 1 if a battery power source is present, fills out values via pointers.
// percent = -1 when not available. charging = 1 if charging, 0 otherwise.
// power_mw is set to instantaneous power in milliwatts (positive = charging, negative = discharging).
// state buffer should be at least 32 bytes.
static int mactop_get_battery(int *percent, int *charging, char *state, int state_len, int *power_mw) {
    *percent = -1;
    *charging = 0;
    *power_mw = 0;
    if (state && state_len > 0) state[0] = '\0';

    CFTypeRef info = IOPSCopyPowerSourcesInfo();
    if (!info) return 0;
    CFArrayRef list = IOPSCopyPowerSourcesList(info);
    if (!list) {
        CFRelease(info);
        return 0;
    }

    int found = 0;
    CFIndex count = CFArrayGetCount(list);
    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef ps = IOPSGetPowerSourceDescription(info, CFArrayGetValueAtIndex(list, i));
        if (!ps) continue;

        CFStringRef type = CFDictionaryGetValue(ps, CFSTR(kIOPSTypeKey));
        if (!type || CFStringCompare(type, CFSTR(kIOPSInternalBatteryType), 0) != kCFCompareEqualTo) {
            continue;
        }

        CFNumberRef cap = CFDictionaryGetValue(ps, CFSTR(kIOPSCurrentCapacityKey));
        CFNumberRef max = CFDictionaryGetValue(ps, CFSTR(kIOPSMaxCapacityKey));
        if (cap && max) {
            int c = 0, m = 0;
            CFNumberGetValue(cap, kCFNumberIntType, &c);
            CFNumberGetValue(max, kCFNumberIntType, &m);
            if (m > 0) *percent = (c * 100) / m;
        }

        CFBooleanRef isCharging = CFDictionaryGetValue(ps, CFSTR(kIOPSIsChargingKey));
        if (isCharging && CFBooleanGetValue(isCharging)) *charging = 1;

        CFStringRef st = CFDictionaryGetValue(ps, CFSTR(kIOPSPowerSourceStateKey));
        if (st && state && state_len > 0) {
            CFStringGetCString(st, state, state_len, kCFStringEncodingUTF8);
        }

        found = 1;
        break;
    }

    CFRelease(list);
    CFRelease(info);

    if (found)
        *power_mw = mactop_battery_power_mw();

    return found;
}
*/
import "C"

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/metaspartan/mactop/v2/internal/i18n"
)

// BatteryInfo describes the current internal battery state.
//
// Present reports whether the host actually has an internal battery. Percent is
// a pointer so we can distinguish "no charge reading available" (nil — e.g. a
// battery is present but IOKit's capacity keys are momentarily missing) from a
// real 0%. Consumers should treat a nil/omitted Percent as "charge unknown",
// not as "no battery" (use Present for that).
type BatteryInfo struct {
	Present     bool    `json:"present" yaml:"present" xml:"Present" toon:"present"`
	Percent     *int    `json:"percent,omitempty" yaml:"percent,omitempty" xml:"Percent,omitempty" toon:"percent"`
	Charging    bool    `json:"charging" yaml:"charging" xml:"Charging" toon:"charging"`
	OnACPower   bool    `json:"on_ac_power" yaml:"on_ac_power" xml:"OnACPower" toon:"on_ac_power"`
	State       string  `json:"state" yaml:"state" xml:"State" toon:"state"`
	PowerWatts  float64 `json:"power_watts" yaml:"power_watts" xml:"PowerWatts" toon:"power_watts"`
}

var (
	hasBatteryOnce    sync.Once
	hasBatteryPresent bool
)

// GetBatteryInfo returns the current battery state, or Present=false if no battery.
func GetBatteryInfo() BatteryInfo {
	var percent C.int
	var charging C.int
	var powerMW C.int
	var stateBuf [32]C.char
	res := C.mactop_get_battery(&percent, &charging, &stateBuf[0], C.int(len(stateBuf)), &powerMW)
	if res == 0 {
		return BatteryInfo{}
	}
	state := C.GoString(&stateBuf[0])
	info := BatteryInfo{
		Present:    true,
		Charging:   charging == 1,
		OnACPower:  strings.EqualFold(state, "AC Power"),
		State:      state,
		PowerWatts: float64(powerMW) / 1000.0,
	}
	// The C layer leaves percent at -1 when IOKit capacity keys are missing.
	// Only attach a charge value when it's a real 0–100 reading; otherwise
	// leave Percent nil ("charge unknown") so it's never shown as -1.
	if p := int(percent); p >= 0 && p <= 100 {
		info.Percent = &p
	}
	return info
}

// HasBattery reports whether the host has an internal battery (MacBook/laptop).
// Cached after first call.
func HasBattery() bool {
	hasBatteryOnce.Do(func() {
		hasBatteryPresent = GetBatteryInfo().Present
	})
	return hasBatteryPresent
}

// Displayable reports whether this reading has a usable charge percentage.
// A host can have a battery (Present == true) while IOKit's capacity keys are
// momentarily missing, in which case the C layer leaves Percent at -1. Such a
// reading must not be rendered as a negative charge level, exported to
// Prometheus, or shown in the info panel (issue: battery shows -1%).
func (b BatteryInfo) Displayable() bool {
	return b.Present && b.Percent != nil
}

// batteryStateLabel returns the localized state string for a battery.
func batteryStateLabel(bat BatteryInfo) string {
	switch {
	case bat.Charging:
		return i18n.T("Info_BatteryCharging")
	case bat.OnACPower:
		return i18n.T("Info_BatteryAC")
	default:
		return i18n.T("Info_BatteryDischarging")
	}
}

// formatBatteryLine returns "Battery: 87% (charging, 45.2 W)" or empty string if no battery.
func formatBatteryLine() string {
	bat := GetBatteryInfo()
	if !bat.Displayable() {
		return ""
	}
	stateLabel := batteryStateLabel(bat)
	if bat.PowerWatts != 0 {
		absW := bat.PowerWatts
		if absW < 0 {
			absW = -absW
		}
		stateLabel = fmt.Sprintf("%s, %.1f W", stateLabel, absW)
	}
	return fmt.Sprintf("%s: %d%% (%s)", i18n.T("Info_Battery"), *bat.Percent, stateLabel)
}

// avoid "imported and not used" if a future build prunes battery usage
var _ = unsafe.Pointer(nil)
