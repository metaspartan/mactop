package app

/*
#include <sys/types.h>
#include <sys/sysctl.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
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
	cpuInfoDict := getCPUInfo()
	
	// Use authoritative core counts from BuildCoreLabels which matches the gauge
	// and accurately cross-references IORegistry with sysctl perflevels.
	_, eCount, pCount, sCount, _ := BuildCoreLabels()

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

func getCPUInfo() map[string]string {
	out, err := exec.Command("sysctl", "machdep.cpu").Output()
	if err != nil {
		stderrLogger.Fatalf("failed to execute getCPUInfo() sysctl command: %v", err)
	}
	cpuInfo := string(out)
	cpuInfoLines := strings.Split(cpuInfo, "\n")
	dataFields := []string{"machdep.cpu.brand_string", "machdep.cpu.core_count"}
	cpuInfoDict := make(map[string]string)
	for _, line := range cpuInfoLines {
		for _, field := range dataFields {
			if strings.Contains(line, field) {
				value := strings.TrimSpace(strings.Split(line, ":")[1])
				cpuInfoDict[field] = value
			}
		}
	}
	return cpuInfoDict
}

// getPerfLevelCores dynamically queries sysctl hw.perflevel* to discover
// core types and counts. Returns a map: "E" -> count, "P" -> count, "S" -> count.
// Works across all M-series chips without hardcoding perflevel indices.
func getPerfLevelCores() map[string]int {
	result := map[string]int{"E": 0, "P": 0, "S": 0}

	// Get number of performance levels
	nperfCmd := exec.Command("sysctl", "-n", "hw.nperflevels")
	nperfCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	nperfOut, err := nperfCmd.Output()
	if err != nil {
		// Fallback to legacy 2-level query
		return getPerfLevelCoresLegacy()
	}
	nperflevels, _ := strconv.Atoi(strings.TrimSpace(string(nperfOut)))
	if nperflevels == 0 {
		return getPerfLevelCoresLegacy()
	}

	// Query each perflevel for its name and core count
	for i := 0; i < nperflevels; i++ {
		nameKey := fmt.Sprintf("hw.perflevel%d.name", i)
		cpuKey := fmt.Sprintf("hw.perflevel%d.logicalcpu", i)

		cmd := exec.Command("sysctl", "-n", nameKey, cpuKey)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		out, err := cmd.Output()
		if err != nil {
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) < 2 {
			continue
		}

		name := strings.TrimSpace(lines[0])
		count, _ := strconv.Atoi(strings.TrimSpace(lines[1]))

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

	cmd := exec.Command("sysctl", "hw.perflevel0.logicalcpu", "hw.perflevel1.logicalcpu")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.Output()
	if err != nil {
		return result
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "hw.perflevel0.logicalcpu") {
			val, _ := strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
			result["P"] = val
		} else if strings.Contains(line, "hw.perflevel1.logicalcpu") {
			val, _ := strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
			result["E"] = val
		}
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

func getThermalStateString() (string, bool) {
	name := C.CString("machdep.xcpm.cpu_thermal_level")
	defer C.free(unsafe.Pointer(name))

	var val int32
	size := C.size_t(unsafe.Sizeof(val))

	if C.sysctlbyname(name, unsafe.Pointer(&val), &size, nil, 0) != 0 {
		return "Normal", false
	}

	switch val {
	case 0:
		return "Normal", false
	case 1:
		return "Fair", true
	case 2:
		return "Serious", true
	case 3:
		return "Critical", true
	default:
		return "Normal", false
	}
}
