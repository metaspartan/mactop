package app

/*
#include <sys/types.h>
#include <sys/sysctl.h>
#include <stdlib.h>
*/
import "C"

import (
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
	coreCountsDict := getCoreCounts()
	var eCoreCounts, pCoreCounts int
	if val, ok := coreCountsDict["hw.perflevel1.logicalcpu"]; ok {
		eCoreCounts = val
	}
	if val, ok := coreCountsDict["hw.perflevel0.logicalcpu"]; ok {
		pCoreCounts = val
	}

	coreCount, _ := strconv.Atoi(cpuInfoDict["machdep.cpu.core_count"])
	gpuCoreCountStr := getGPUCores()
	gpuCoreCount, _ := strconv.Atoi(gpuCoreCountStr)
	if gpuCoreCount == 0 && gpuCoreCountStr != "?" {
	}

	return SystemInfo{
		Name:         cpuInfoDict["machdep.cpu.brand_string"],
		CoreCount:    coreCount,
		ECoreCount:   eCoreCounts,
		PCoreCount:   pCoreCounts,
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

func getCoreCounts() map[string]int {
	cmd := exec.Command("sysctl", "hw.perflevel0.logicalcpu", "hw.perflevel1.logicalcpu")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.Output()
	if err != nil {
		stderrLogger.Fatalf("failed to execute getCoreCounts() sysctl command: %v", err)
	}
	coresInfo := string(out)
	coresInfoLines := strings.Split(coresInfo, "\n")
	dataFields := []string{"hw.perflevel0.logicalcpu", "hw.perflevel1.logicalcpu"}
	coresInfoDict := make(map[string]int)
	for _, line := range coresInfoLines {
		for _, field := range dataFields {
			if strings.Contains(line, field) {
				value, _ := strconv.Atoi(strings.TrimSpace(strings.Split(line, ":")[1]))
				coresInfoDict[field] = value
			}
		}
	}
	return coresInfoDict
}

func getGPUCores() string {
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
