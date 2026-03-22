// Copyright (c) 2024-2026 Carsen Klock under MIT License
// ioreport.go - Go wrappers for IOReport power/thermal metrics
package app

import (
	"fmt"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit -framework Foundation -framework CoreWLAN -lIOReport
#include <mach/mach_host.h>
#include <mach/processor_info.h>
#include <mach/mach_init.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <stdint.h>
#include <string.h>
#include <stdlib.h>

typedef struct IOReportSubscriptionRef* IOReportSubscriptionRef;

extern CFDictionaryRef IOReportCopyChannelsInGroup(CFStringRef group, CFStringRef subgroup, uint64_t a, uint64_t b, uint64_t c);
extern void IOReportMergeChannels(CFDictionaryRef a, CFDictionaryRef b, CFTypeRef unused);
extern IOReportSubscriptionRef IOReportCreateSubscription(void* a, CFMutableDictionaryRef channels, CFMutableDictionaryRef* out, uint64_t d, CFTypeRef e);
extern CFDictionaryRef IOReportCreateSamples(IOReportSubscriptionRef sub, CFMutableDictionaryRef channels, CFTypeRef unused);
extern CFDictionaryRef IOReportCreateSamplesDelta(CFDictionaryRef a, CFDictionaryRef b, CFTypeRef unused);
extern int64_t IOReportSimpleGetIntegerValue(CFDictionaryRef item, int32_t idx);
extern CFStringRef IOReportChannelGetGroup(CFDictionaryRef item);
extern CFStringRef IOReportChannelGetSubGroup(CFDictionaryRef item);
extern CFStringRef IOReportChannelGetChannelName(CFDictionaryRef item);
extern CFStringRef IOReportChannelGetUnitLabel(CFDictionaryRef item);
extern int32_t IOReportStateGetCount(CFDictionaryRef item);
extern CFStringRef IOReportStateGetNameForIndex(CFDictionaryRef item, int32_t idx);
extern int64_t IOReportStateGetResidency(CFDictionaryRef item, int32_t idx);

typedef void* IOHIDEventSystemClientRef;
typedef void* IOHIDServiceClientRef;
typedef void* IOHIDEventRef;

extern IOHIDEventSystemClientRef IOHIDEventSystemClientCreate(CFAllocatorRef allocator);
extern int IOHIDEventSystemClientSetMatching(IOHIDEventSystemClientRef client, CFDictionaryRef matching);
extern CFArrayRef IOHIDEventSystemClientCopyServices(IOHIDEventSystemClientRef client);
extern CFStringRef IOHIDServiceClientCopyProperty(IOHIDServiceClientRef service, CFStringRef key);
extern IOHIDEventRef IOHIDServiceClientCopyEvent(IOHIDServiceClientRef service, int64_t type, int32_t options, int64_t timeout);
extern double IOHIDEventGetFloatValue(IOHIDEventRef event, int64_t field);

typedef struct {
    char name[32];
    int actualRPM;
    int minRPM;
    int maxRPM;
    int targetRPM;
    int mode;
    int id;
} fan_info_t;

typedef struct {
    char key[5];
    char name[64];
    float value;
} temp_sensor_t;

typedef struct {
    double cpuPower;
    double gpuPower;
    double anePower;
    double dramPower;
    double gpuSramPower;
    double systemPower;
    int gpuFreqMHz;
    double gpuActive;
    double eClusterActive;
    double pClusterActive;
    double sClusterActive;
    int eClusterFreqMHz;
    int pClusterFreqMHz;
    int sClusterFreqMHz;
    float socTemp;
    float cpuTemp;
    float gpuTemp;
    int64_t dramReadBytes;
    int64_t dramWriteBytes;
    int fanCount;
    fan_info_t fans[8];
    int tempSensorCount;
    temp_sensor_t temps[128];
} PowerMetrics;

int initIOReport();
PowerMetrics samplePowerMetrics(int durationMs);
void cleanupIOReport();
int getThermalState();
extern void debugIOReport(void);
extern void printAllChannels(void);
extern void debugMonitorChannels(int durationMs);
extern void dumpAllSMCTemps(void);
extern void setExpectedCoreCounts(int eCores, int pCores, int sCores);
int setFanForceTest(int enabled);
int setFanMode(int fanIndex, int mode);
int setFanTarget(int fanIndex, int rpm);
int resetFansToAuto();

// Wi-Fi link info structure (defined in ioreport.m)
typedef struct {
    char interface_name[32];
    char phy_mode[32];
    char wifi_generation[16];
    int tx_rate_mbps;
    int is_connected;
} wifi_link_info_t;

int get_wifi_link_info(wifi_link_info_t *info);
*/
import "C"

// FanInfo represents a single system fan's state
type FanInfo struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ActualRPM int    `json:"actual_rpm"`
	MinRPM    int    `json:"min_rpm"`
	MaxRPM    int    `json:"max_rpm"`
	TargetRPM int    `json:"target_rpm"`
	Mode      int    `json:"mode"` // 0=auto, 1=forced
}

// TempSensor represents a single temperature sensor reading
type TempSensor struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type SocMetrics struct {
	CPUPower        float64      `json:"cpu_power"`
	GPUPower        float64      `json:"gpu_power"`
	ANEPower        float64      `json:"ane_power"`
	DRAMPower       float64      `json:"dram_power"`
	GPUSRAMPower    float64      `json:"gpu_sram_power"`
	SystemPower     float64      `json:"system_power"`
	TotalPower      float64      `json:"total_power"`
	GPUFreqMHz      int32        `json:"gpu_freq_mhz"`
	GPUActive       float64      `json:"gpu_active"`
	EClusterActive  float64      `json:"e_cluster_active"`
	PClusterActive  float64      `json:"p_cluster_active"`
	SClusterActive  float64      `json:"s_cluster_active,omitempty"`
	EClusterFreqMHz int32        `json:"e_cluster_freq_mhz"`
	PClusterFreqMHz int32        `json:"p_cluster_freq_mhz"`
	SClusterFreqMHz int32        `json:"s_cluster_freq_mhz,omitempty"`
	SocTemp         float32      `json:"soc_temp"`
	CPUTemp         float32      `json:"cpu_temp"`
	GPUTemp         float32      `json:"gpu_temp"`
	DRAMReadBW      float64      `json:"dram_read_bw_gbs"`
	DRAMWriteBW     float64      `json:"dram_write_bw_gbs"`
	DRAMBWCombined  float64      `json:"dram_bw_combined_gbs"`
	Fans            []FanInfo    `json:"-"`
	TempSensors     []TempSensor `json:"-"`
}

func initSocMetrics() error {
	if ret := C.initIOReport(); ret != 0 {
		return fmt.Errorf("initIOReport failed with code %d", ret)
	}
	// Pass expected core counts to C for HID sensor validation.
	// HID per-core sensors are only used when count >= expected physical cores.
	sysInfo := getSOCInfo()
	C.setExpectedCoreCounts(C.int(sysInfo.ECoreCount), C.int(sysInfo.PCoreCount), C.int(sysInfo.SCoreCount))
	return nil
}

func sampleSocMetrics(durationMs int) SocMetrics {
	pm := C.samplePowerMetrics(C.int(durationMs))

	var dramReadBW, dramWriteBW, dramBWCombined float64
	if durationMs > 0 {
		dramReadBW = float64(pm.dramReadBytes) / float64(durationMs) * 1000.0 / 1e9
		dramWriteBW = float64(pm.dramWriteBytes) / float64(durationMs) * 1000.0 / 1e9
		dramBWCombined = float64(pm.dramReadBytes+pm.dramWriteBytes) / float64(durationMs) * 1000.0 / 1e9
	}

	// Convert fan data from C arrays to Go slices
	fans := make([]FanInfo, int(pm.fanCount))
	for i := 0; i < int(pm.fanCount) && i < 8; i++ {
		cf := pm.fans[i]
		fans[i] = FanInfo{
			ID:        int(cf.id),
			Name:      C.GoString(&cf.name[0]),
			ActualRPM: int(cf.actualRPM),
			MinRPM:    int(cf.minRPM),
			MaxRPM:    int(cf.maxRPM),
			TargetRPM: int(cf.targetRPM),
			Mode:      int(cf.mode),
		}
	}

	// Convert temp sensor data from C arrays to Go slices
	tempSensors := make([]TempSensor, int(pm.tempSensorCount))
	for i := 0; i < int(pm.tempSensorCount) && i < 128; i++ {
		ct := pm.temps[i]
		tempSensors[i] = TempSensor{
			Key:   C.GoString(&ct.key[0]),
			Name:  C.GoString(&ct.name[0]),
			Value: float64(ct.value),
		}
	}

	return SocMetrics{
		CPUPower:        float64(pm.cpuPower),
		GPUPower:        float64(pm.gpuPower),
		ANEPower:        float64(pm.anePower),
		DRAMPower:       float64(pm.dramPower),
		GPUSRAMPower:    float64(pm.gpuSramPower),
		SystemPower:     float64(pm.systemPower),
		TotalPower:      float64(pm.cpuPower) + float64(pm.gpuPower) + float64(pm.anePower) + float64(pm.dramPower) + float64(pm.gpuSramPower),
		GPUFreqMHz:      int32(pm.gpuFreqMHz),
		GPUActive:       float64(pm.gpuActive),
		EClusterActive:  float64(pm.eClusterActive),
		PClusterActive:  float64(pm.pClusterActive),
		SClusterActive:  float64(pm.sClusterActive),
		EClusterFreqMHz: int32(pm.eClusterFreqMHz),
		PClusterFreqMHz: int32(pm.pClusterFreqMHz),
		SClusterFreqMHz: int32(pm.sClusterFreqMHz),
		SocTemp:         float32(pm.socTemp),
		CPUTemp:         float32(pm.cpuTemp),
		GPUTemp:         float32(pm.gpuTemp),
		DRAMReadBW:      dramReadBW,
		DRAMWriteBW:     dramWriteBW,
		DRAMBWCombined:  dramBWCombined,
		Fans:            fans,
		TempSensors:     tempSensors,
	}
}

func cleanupSocMetrics() {
	C.cleanupIOReport()
}

func getSocThermalState() int {
	return int(C.getThermalState())
}

// SetFanForceTest enables/disables SMC force test mode (required on M3/M4+ for manual fan control)
func SetFanForceTest(enabled bool) error {
	val := C.int(0)
	if enabled {
		val = C.int(1)
	}
	if C.setFanForceTest(val) != 0 {
		return fmt.Errorf("failed to set fan force test mode")
	}
	return nil
}

// SetFanMode sets a fan to auto (0) or forced/manual (1) mode
func SetFanMode(fanIndex, mode int) error {
	if C.setFanMode(C.int(fanIndex), C.int(mode)) != 0 {
		return fmt.Errorf("failed to set fan %d mode to %d", fanIndex, mode)
	}
	return nil
}

// SetFanTarget sets the target RPM for a fan (clamped to min/max by C layer)
func SetFanTarget(fanIndex, rpm int) error {
	if C.setFanTarget(C.int(fanIndex), C.int(rpm)) != 0 {
		return fmt.Errorf("failed to set fan %d target to %d RPM", fanIndex, rpm)
	}
	return nil
}

// ResetFansToAuto restores all fans to automatic control
func ResetFansToAuto() error {
	if C.resetFansToAuto() != 0 {
		return fmt.Errorf("failed to reset fans to auto")
	}
	return nil
}

// DebugIOReport prints all available IOReport channels and groups to stdout
func DebugIOReport() {
	C.debugIOReport()
}

// DumpAllSMCTemps prints all SMC temperature keys with raw values for diagnostics
func DumpAllSMCTemps() {
	C.dumpAllSMCTemps()
}

// WiFiLinkInfo represents Wi-Fi interface link information
type WiFiLinkInfo struct {
	InterfaceName  string // Interface name (en0, en1, etc.)
	PHYMode        string // "802.11n", "802.11ac", "802.11ax", etc.
	WiFiGeneration string // "Wi-Fi 4", "Wi-Fi 5", "Wi-Fi 6", etc.
	TxRateMbps     int    // Current transmit rate in Mbps
	IsConnected    bool   // True if associated to a network
}

// GetWiFiLinkInfo returns Wi-Fi link information
func GetWiFiLinkInfo() *WiFiLinkInfo {
	var info C.wifi_link_info_t
	ret := C.get_wifi_link_info(&info)
	if ret != 0 {
		return nil
	}
	return &WiFiLinkInfo{
		InterfaceName:  C.GoString(&info.interface_name[0]),
		PHYMode:        C.GoString(&info.phy_mode[0]),
		WiFiGeneration: C.GoString(&info.wifi_generation[0]),
		TxRateMbps:     int(info.tx_rate_mbps),
		IsConnected:    info.is_connected != 0,
	}
}
