package app

/*
#include <sys/types.h>
#include <sys/sysctl.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/metaspartan/mactop/v2/internal/i18n"
)

var (
	cachedSOCInfoResult SystemInfo
	socInfoOnce         sync.Once
)

type VolumeInfo struct {
	Name      string
	Total     float64
	Used      float64
	Available float64
	UsedPct   float64
}

func getVolumes() []VolumeInfo {
	var volumes []VolumeInfo
	partitions, err := GetNativePartitions(false)
	if err != nil {
		return volumes
	}

	excludedVolumes := map[string]bool{
		"/Volumes/Recovery":   true,
		"/Volumes/Preboot":    true,
		"/Volumes/VM":         true,
		"/Volumes/Update":     true,
		"/Volumes/xarts":      true,
		"/Volumes/iSCPreboot": true,
		"/Volumes/Hardware":   true,
	}

	seen := make(map[string]bool)
	for _, p := range partitions {
		if seen[p.Device] {
			continue
		}
		if !strings.HasPrefix(p.Mountpoint, "/Volumes/") && p.Mountpoint != "/" {
			continue
		}

		excluded := false
		for k := range excludedVolumes {
			if strings.Contains(p.Mountpoint, k) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		usage, err := GetNativeDiskUsage(p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		seen[p.Device] = true
		var name string
		if p.Mountpoint == "/" {
			name = "Mac HD"
		} else {
			name = strings.TrimPrefix(p.Mountpoint, "/Volumes/")
		}
		if len(name) > 12 {
			name = name[:12]
		}
		volumes = append(volumes, VolumeInfo{
			Name:      name,
			Total:     float64(usage.Total) / 1e9,
			Used:      float64(usage.Used) / 1e9,
			Available: float64(usage.Free) / 1e9,
			UsedPct:   usage.UsedPercent,
		})
	}
	return volumes
}

func getSOCInfo() SystemInfo {
	socInfoOnce.Do(func() {
		cachedSOCInfoResult = computeSOCInfo()
	})
	return cachedSOCInfoResult
}

func computeSOCInfo() SystemInfo {
	cpuInfoDict := getCPUInfo()

	// Use authoritative core counts from BuildCoreLabels which matches the gauge
	// and accurately cross-references IORegistry with sysctl perflevels.
	_, eCount, pCount, sCount, _ := BuildCoreLabels()

	// Fallback: if BuildCoreLabels failed (IORegistry unavailable), use sysctl directly
	if eCount == 0 && pCount == 0 && sCount == 0 {
		coreTiers := getPerfLevelCores()
		eCount = coreTiers["E"]
		pCount = coreTiers["P"]
		sCount = coreTiers["S"]
	}

	coreCount, _ := strconv.Atoi(cpuInfoDict["machdep.cpu.core_count"])
	gpuCoreCountStr := getGPUCores()
	gpuCoreCount, _ := strconv.Atoi(gpuCoreCountStr)
	if gpuCoreCount == 0 && gpuCoreCountStr != "?" {
	}

	return SystemInfo{
		Name:         cpuInfoDict["machdep.cpu.brand_string"],
		CoreCount:    coreCount,
		ECoreCount:   eCount,
		PCoreCount:   pCount,
		SCoreCount:   sCount,
		GPUCoreCount: gpuCoreCount,
	}
}

// sysctlStringByName reads a sysctl string value directly via the C API,
// avoiding the overhead of spawning an external process.
func sysctlStringByName(name string) (string, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var size C.size_t
	if C.sysctlbyname(cName, nil, &size, nil, 0) != 0 {
		return "", fmt.Errorf("sysctl size query failed for %s", name)
	}
	buf := C.malloc(size)
	defer C.free(buf)
	if C.sysctlbyname(cName, buf, &size, nil, 0) != 0 {
		return "", fmt.Errorf("sysctl value query failed for %s", name)
	}
	return C.GoString((*C.char)(buf)), nil
}

// sysctlIntByName reads a sysctl integer value directly via the C API.
func sysctlIntByName(name string) (int, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var val C.int
	size := C.size_t(unsafe.Sizeof(val))
	if C.sysctlbyname(cName, unsafe.Pointer(&val), &size, nil, 0) != 0 {
		return 0, fmt.Errorf("sysctl int query failed for %s", name)
	}
	return int(val), nil
}

func getCPUInfo() map[string]string {
	cpuInfoDict := make(map[string]string)

	brand, err := sysctlStringByName("machdep.cpu.brand_string")
	if err != nil {
		stderrLogger.Fatalf("failed to get CPU brand string: %v", err)
	}
	cpuInfoDict["machdep.cpu.brand_string"] = brand

	coreCount, err := sysctlIntByName("machdep.cpu.core_count")
	if err != nil {
		stderrLogger.Fatalf("failed to get CPU core count: %v", err)
	}
	cpuInfoDict["machdep.cpu.core_count"] = strconv.Itoa(coreCount)

	return cpuInfoDict
}

// getPerfLevelCores dynamically queries sysctl hw.perflevel* to discover
// core types and counts. Returns a map: "E" -> count, "P" -> count, "S" -> count.
// Works across all M-series chips without hardcoding perflevel indices.
func getPerfLevelCores() map[string]int {
	result := map[string]int{"E": 0, "P": 0, "S": 0}

	// Get number of performance levels via direct sysctl (no subprocess)
	nperflevels, err := sysctlIntByName("hw.nperflevels")
	if err != nil || nperflevels == 0 {
		return getPerfLevelCoresLegacy()
	}

	// Query each perflevel for its name and core count via direct sysctl
	for i := 0; i < nperflevels; i++ {
		name, err := sysctlStringByName(fmt.Sprintf("hw.perflevel%d.name", i))
		if err != nil {
			continue
		}
		count, err := sysctlIntByName(fmt.Sprintf("hw.perflevel%d.logicalcpu", i))
		if err != nil {
			continue
		}

		// Map perflevel names to core type letters
		switch {
		case strings.HasPrefix(name, "Super"):
			result["S"] += count
		case strings.HasPrefix(name, "Performance"):
			result["P"] += count
		case strings.HasPrefix(name, "Efficiency"):
			result["E"] += count
		default:
			// Unknown tier — treat as P-core for safety
			result["P"] += count
		}
	}

	return result
}

// getPerfLevelCoresLegacy is the fallback for systems without hw.nperflevels
func getPerfLevelCoresLegacy() map[string]int {
	result := map[string]int{"E": 0, "P": 0, "S": 0}

	pVal, err := sysctlIntByName("hw.perflevel0.logicalcpu")
	if err == nil {
		result["P"] = pVal
	}
	eVal, err := sysctlIntByName("hw.perflevel1.logicalcpu")
	if err == nil {
		result["E"] = eVal
	}
	return result
}

func getGPUCores() string {
	count := GetGPUCoreCountFast()
	if count > 0 {
		return strconv.Itoa(count)
	}

	data, err := GetGlobalProfilerData()
	if err != nil {
		stderrLogger.Printf("failed to get global profiler data: %v", err)
		return "?"
	}

	for _, display := range data.DisplayItems {
		if display.Cores != "" {
			return display.Cores
		}
	}
	return "?"
}

type thermalStateLevel int

const (
	thermalStateUnknown  thermalStateLevel = -1
	thermalStateNominal  thermalStateLevel = 0
	thermalStateFair     thermalStateLevel = 1
	thermalStateSerious  thermalStateLevel = 2
	thermalStateCritical thermalStateLevel = 3
)

func getThermalStateLevel() thermalStateLevel {
	name := C.CString("machdep.xcpm.cpu_thermal_level")
	defer C.free(unsafe.Pointer(name))

	var val int32
	size := C.size_t(unsafe.Sizeof(val))

	if C.sysctlbyname(name, unsafe.Pointer(&val), &size, nil, 0) != 0 {
		return thermalStateNominal
	}

	switch val {
	case 0:
		return thermalStateNominal
	case 1:
		return thermalStateFair
	case 2:
		return thermalStateSerious
	case 3:
		return thermalStateCritical
	default:
		return thermalStateUnknown
	}
}

func thermalStateString(level thermalStateLevel) string {
	switch level {
	case thermalStateNominal:
		return i18n.T("Metrics_ThermalNominal")
	case thermalStateFair:
		return i18n.T("Metrics_ThermalFair")
	case thermalStateSerious:
		return i18n.T("Metrics_ThermalSerious")
	case thermalStateCritical:
		return i18n.T("Metrics_ThermalCritical")
	default:
		return i18n.T("Metrics_ThermalUnknown")
	}
}

func thermalStateThrottled(level thermalStateLevel) bool {
	switch level {
	case thermalStateFair, thermalStateSerious, thermalStateCritical:
		return true
	default:
		return false
	}
}

func getThermalStateString() (string, bool) {
	level := getThermalStateLevel()
	return thermalStateString(level), thermalStateThrottled(level)
}
