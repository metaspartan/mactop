// Copyright (c) 2024-2026 Carsen Klock under MIT License
// overlay.go - Go wrappers for native macOS floating overlay HUD
package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

typedef struct {
    double cpu_percent;
    double gpu_percent;
    double ane_percent;
    int gpu_freq_mhz;
    unsigned long long mem_used_bytes;
    unsigned long long mem_total_bytes;
    unsigned long long swap_used_bytes;
    unsigned long long swap_total_bytes;
    double total_watts;
    double package_watts;
    double cpu_watts;
    double gpu_watts;
    double ane_watts;
    double dram_watts;
    double soc_temp;
    double cpu_temp;
    double gpu_temp;
    char thermal_state[32];
    int thermal_level; // 0=nominal, 1=fair, 2=serious, 3=critical
    char model_name[128];
    int gpu_core_count;
    int e_core_count;
    int p_core_count;
    int s_core_count;
    int ecluster_freq_mhz;
    double ecluster_active;
    int pcluster_freq_mhz;
    double pcluster_active;
    int scluster_freq_mhz;
    double scluster_active;
    double net_in_bytes_per_sec;
    double net_out_bytes_per_sec;
    double disk_read_kb_per_sec;
    double disk_write_kb_per_sec;
    double tflops_fp32;
    char rdma_status[64];
    double dram_bw_combined_gbs;
    int fan_count;
    int fan_rpm[4];
    char fan_name[4][32];
} overlay_metrics_t;

typedef struct {
    int show_cpu;
    int show_gpu;
    int show_ane;
    int show_memory;
    int show_power;
    int show_temps;
    int show_thermals;
    int show_fans;
    int show_bandwidth;
    int show_network;
    int show_gpu_freq;
    double opacity;
    char collapsed_sections[256]; // comma-separated ordered section names for collapsed mode
    char expanded_order[512];     // comma-separated ordered section names for expanded mode

    char label_fps[32];
    char label_frame[32];
    char label_cpu[32];
    char label_gpu[32];
    char label_ane[32];
    char label_memory[32];
    char label_swap[32];
    char label_power[32];
    char label_bandwidth[64];
    char label_gpu_freq[64];
    char label_temps[32];
    char label_thermal[32];
    char label_fans[32];
    char label_network[32];
} overlay_config_t;

int initOverlay(void);
void setOverlayConfig(overlay_config_t *cfg);
void updateOverlayMetrics(overlay_metrics_t *m);
void runOverlayLoop(void);
void cleanupOverlay(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/metaspartan/mactop/v2/internal/i18n"
)

// Overlay worker state
var (
	overlayMetricsEncoder *json.Encoder
	overlayWorkerCmd      *exec.Cmd
	overlayWorkerStdin    io.WriteCloser
	overlayMu             sync.Mutex
	overlayLastRestart    time.Time
)

// applyOverlayConfig parses the --overlay section filter and applies the config
func applyOverlayConfig(sections string) {
	var ccfg C.overlay_config_t
	// Default: all sections enabled
	ccfg.show_cpu = 1
	ccfg.show_gpu = 1
	ccfg.show_ane = 1
	ccfg.show_memory = 1
	ccfg.show_power = 1
	ccfg.show_temps = 1
	ccfg.show_thermals = 1
	ccfg.show_fans = 1
	ccfg.show_bandwidth = 1
	ccfg.show_network = 1
	ccfg.show_gpu_freq = 1
	ccfg.opacity = 0.88

	// Apply opacity from CLI/env
	if overlayOpacity > 0 {
		if overlayOpacity < 0.15 {
			overlayOpacity = 0.15
		}
		if overlayOpacity > 1.0 {
			overlayOpacity = 1.0
		}
		ccfg.opacity = C.double(overlayOpacity)
	}

	// If sections are specified via CLI, disable all then enable only requested
	if sections != "" {
		ccfg.show_cpu = 0
		ccfg.show_gpu = 0
		ccfg.show_ane = 0
		ccfg.show_memory = 0
		ccfg.show_power = 0
		ccfg.show_temps = 0
		ccfg.show_thermals = 0
		ccfg.show_fans = 0
		ccfg.show_bandwidth = 0
		ccfg.show_network = 0
		ccfg.show_gpu_freq = 0

		for _, s := range strings.Split(sections, ",") {
			setOverlaySectionFlag(&ccfg, strings.TrimSpace(strings.ToLower(s)))
		}
	}

	// Apply ordered section lists from config or env
	collapsedStr := os.Getenv("MACTOP_OVERLAY_COLLAPSED")
	expandedStr := os.Getenv("MACTOP_OVERLAY_EXPANDED")
	if collapsedStr == "" {
		collapsedStr = strings.Join(overlayDefaultCollapsed, ",")
	}
	if expandedStr == "" {
		expandedStr = strings.Join(overlayDefaultExpanded, ",")
	}

	// Copy to C struct
	copyToCCharBuf(unsafe.Pointer(&ccfg.collapsed_sections), collapsedStr, 256)
	copyToCCharBuf(unsafe.Pointer(&ccfg.expanded_order), expandedStr, 512)

	// Populate localized section labels
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_fps), i18n.T("Overlay_FPS"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_frame), i18n.T("Overlay_FrameInfo"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_cpu), i18n.T("Overlay_CPU"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_gpu), i18n.T("Overlay_GPU"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_ane), i18n.T("Overlay_ANE"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_memory), i18n.T("Overlay_Memory"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_swap), i18n.T("Overlay_Swap"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_power), i18n.T("Overlay_Power"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_bandwidth), i18n.T("Overlay_Bandwidth"), 64)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_gpu_freq), i18n.T("Overlay_GPUFreq"), 64)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_temps), i18n.T("Overlay_Temps"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_thermal), i18n.T("Overlay_Thermal"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_fans), i18n.T("Overlay_Fans"), 32)
	copyToCCharBuf(unsafe.Pointer(&ccfg.label_network), i18n.T("Overlay_Network"), 32)

	C.setOverlayConfig(&ccfg)
}

// copyToCCharBuf copies a Go string into a C char buffer at the given pointer
func copyToCCharBuf(dst unsafe.Pointer, src string, maxLen int) {
	bytes := []byte(src)
	if len(bytes) >= maxLen {
		bytes = bytes[:maxLen-1]
	}
	p := unsafe.Slice((*C.char)(dst), maxLen)
	for i, b := range bytes {
		p[i] = C.char(b)
	}
	p[len(bytes)] = 0
}

// setOverlaySectionFlag is a helper to reduce cyclomatic complexity
func setOverlaySectionFlag(ccfg *C.overlay_config_t, section string) {
	switch section {
	case "cpu":
		ccfg.show_cpu = 1
	case "gpu":
		ccfg.show_gpu = 1
	case "ane", "npu":
		ccfg.show_ane = 1
	case "mem", "memory":
		ccfg.show_memory = 1
	case "power":
		ccfg.show_power = 1
	case "temp", "temps", "temperature":
		ccfg.show_temps = 1
	case "thermal", "thermals":
		ccfg.show_thermals = 1
	case "fan", "fans":
		ccfg.show_fans = 1
	case "bw", "bandwidth":
		ccfg.show_bandwidth = 1
	case "net", "network":
		ccfg.show_network = 1
	case "gpu-freq", "gpu_freq":
		ccfg.show_gpu_freq = 1
	}
}

// startOverlayWorker is the entry point for the child process (--overlay-worker).
// It reads JSON metrics from stdin and updates the overlay on the main thread.
func startOverlayWorker() {
	// NOTE: runtime.LockOSThread() is called in init() to ensure goroutine 1
	// stays on the main OS thread, which AppKit requires for NSWindow creation.

	// Apply section filtering and opacity from environment variables
	sections := os.Getenv("MACTOP_OVERLAY_SECTIONS")
	if opStr := os.Getenv("MACTOP_OVERLAY_OPACITY"); opStr != "" {
		if op, err := strconv.ParseFloat(opStr, 64); err == nil {
			overlayOpacity = op
		}
	}
	applyOverlayConfig(sections)

	// Initialize AppKit + overlay window
	if ret := C.initOverlay(); ret != 0 {
		fmt.Fprintf(os.Stderr, "Failed to initialize overlay worker: %d\n", int(ret))
		os.Exit(1)
	}

	// Decode JSON from stdin in a goroutine
	go func() {
		decoder := json.NewDecoder(os.Stdin)
		gcTicker := time.NewTicker(5 * time.Minute)
		defer gcTicker.Stop()
		for {
			select {
			case <-gcTicker.C:
				runtime.GC()
			default:
			}

			var payload MenuBarMetricsPayload
			if err := decoder.Decode(&payload); err != nil {
				// Parent died or pipe closed
				os.Exit(0)
				return
			}

			updateOverlayFromPayload(payload)
		}
	}()

	// Blocks on [NSApp run]
	C.runOverlayLoop()
	C.cleanupOverlay()
}

// updateOverlayFromPayload converts a Go payload to C struct and pushes it
func updateOverlayFromPayload(p MenuBarMetricsPayload) {
	var cm C.overlay_metrics_t

	cm.cpu_percent = C.double(p.CPUPercent)
	cm.gpu_percent = C.double(p.GPUMetrics.ActivePercent)

	anePct := p.CPUMetrics.ANEW / 8.0 * 100
	if anePct > 100 {
		anePct = 100
	}
	cm.ane_percent = C.double(anePct) // Power-based estimation

	cm.mem_used_bytes = C.ulonglong(p.MemMetrics.Used)
	cm.mem_total_bytes = C.ulonglong(p.MemMetrics.Total)
	cm.swap_used_bytes = C.ulonglong(p.MemMetrics.SwapUsed)
	cm.swap_total_bytes = C.ulonglong(p.MemMetrics.SwapTotal)

	cm.cpu_watts = C.double(p.CPUMetrics.CPUW)
	cm.gpu_watts = C.double(p.GPUMetrics.Power)
	cm.ane_watts = C.double(p.CPUMetrics.ANEW)
	cm.dram_watts = C.double(p.CPUMetrics.DRAMW)
	// PackageW is the correct total: max(componentSum, systemPower)
	cm.package_watts = C.double(p.CPUMetrics.PackageW)
	cm.total_watts = C.double(p.CPUMetrics.PackageW)

	cm.gpu_freq_mhz = C.int(p.GPUMetrics.FreqMHz)
	cm.cpu_temp = C.double(p.CPUMetrics.CPUTemp)
	cm.gpu_temp = C.double(p.GPUMetrics.Temp)

	// Clusters
	cm.ecluster_active = C.double(float64(p.CPUMetrics.EClusterActive))
	cm.ecluster_freq_mhz = C.int(p.CPUMetrics.EClusterFreqMHz)
	cm.pcluster_active = C.double(float64(p.CPUMetrics.PClusterActive))
	cm.pcluster_freq_mhz = C.int(p.CPUMetrics.PClusterFreqMHz)
	cm.scluster_active = C.double(float64(p.CPUMetrics.SClusterActive))
	cm.scluster_freq_mhz = C.int(p.CPUMetrics.SClusterFreqMHz)

	// Network/Disk
	cm.net_in_bytes_per_sec = C.double(p.NetDiskMetrics.InBytesPerSec)
	cm.net_out_bytes_per_sec = C.double(p.NetDiskMetrics.OutBytesPerSec)
	cm.disk_read_kb_per_sec = C.double(p.NetDiskMetrics.ReadKBytesPerSec)
	cm.disk_write_kb_per_sec = C.double(p.NetDiskMetrics.WriteKBytesPerSec)

	// TFLOPs / DRAM BW
	cm.tflops_fp32 = C.double(p.TFLOPs)
	cm.dram_bw_combined_gbs = C.double(p.CPUMetrics.DRAMBWCombined)

	// SysInfo
	cm.gpu_core_count = C.int(p.SysInfo.GPUCoreCount)
	cm.e_core_count = C.int(p.SysInfo.ECoreCount)
	cm.p_core_count = C.int(p.SysInfo.PCoreCount)
	cm.s_core_count = C.int(p.SysInfo.SCoreCount)

	// Model Name
	modelBytes := []byte(p.SysInfo.Name)
	if len(modelBytes) > 127 {
		modelBytes = modelBytes[:127]
	}
	for i, b := range modelBytes {
		cm.model_name[i] = C.char(b)
	}
	cm.model_name[len(modelBytes)] = 0

	// Thermal State
	thermalBytes := []byte(p.ThermalState)
	if len(thermalBytes) > 31 {
		thermalBytes = thermalBytes[:31]
	}
	for i, b := range thermalBytes {
		cm.thermal_state[i] = C.char(b)
	}
	cm.thermal_state[len(thermalBytes)] = 0
	cm.thermal_level = C.int(p.ThermalLevel)

	// RDMA Status
	rdmaBytes := []byte(p.RDMAStatus)
	if len(rdmaBytes) > 63 {
		rdmaBytes = rdmaBytes[:63]
	}
	for i, b := range rdmaBytes {
		cm.rdma_status[i] = C.char(b)
	}
	cm.rdma_status[len(rdmaBytes)] = 0

	// Fans
	fanCount := len(p.CPUMetrics.Fans)
	if fanCount > 4 {
		fanCount = 4
	}
	cm.fan_count = C.int(fanCount)
	for i := 0; i < fanCount; i++ {
		cm.fan_rpm[i] = C.int(p.CPUMetrics.Fans[i].ActualRPM)
		nameBytes := []byte(p.CPUMetrics.Fans[i].Name)
		if len(nameBytes) > 31 {
			nameBytes = nameBytes[:31]
		}
		for j, b := range nameBytes {
			cm.fan_name[i][j] = C.char(b)
		}
		cm.fan_name[i][len(nameBytes)] = 0
	}

	C.updateOverlayMetrics((*C.overlay_metrics_t)(unsafe.Pointer(&cm)))
}

// startOverlayProcess spawns the worker process and sets up the pipe.
func startOverlayProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	cmd := exec.Command(exe, "--overlay-worker")
	// Pass section filter and config via environment variables
	overlayCfg := loadOverlayConfig()
	collapsedStr := strings.Join(overlayCfg.CollapsedSections, ",")
	expandedStr := strings.Join(overlayCfg.ExpandedOrder, ",")

	effectiveOpacity := overlayOpacity
	if overlayCfg.Opacity != nil && overlayOpacity == 0.88 {
		// Use config opacity only if CLI wasn't explicitly set
		effectiveOpacity = *overlayCfg.Opacity
	}

	cmd.Env = append(os.Environ(),
		"MACTOP_OVERLAY_SECTIONS="+overlaySections,
		fmt.Sprintf("MACTOP_OVERLAY_OPACITY=%.2f", effectiveOpacity),
		"MACTOP_OVERLAY_COLLAPSED="+collapsedStr,
		"MACTOP_OVERLAY_EXPANDED="+expandedStr,
		"MACTOP_LANG="+resolvedLanguage,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %v", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start overlay worker: %v", err)
	}

	overlayWorkerCmd = cmd
	overlayWorkerStdin = stdin
	overlayMetricsEncoder = json.NewEncoder(stdin)

	// Wait for worker in background
	go func() {
		cmd.Wait()
	}()

	return nil
}

// pushOverlayMetrics sends metrics to the overlay child process.
// Auto-restarts the worker with a 2s cooldown if the pipe breaks.
func pushOverlayMetrics(sm SocMetrics, cpuMetrics CPUMetrics, gpuMetrics GPUMetrics, netDisk NetDiskMetrics, sysInfo SystemInfo, maxFP32TFLOPs float64, cpuPercent float64, thermalState string, rdmaStatus string) {
	overlayMu.Lock()
	defer overlayMu.Unlock()

	if overlayMetricsEncoder == nil {
		return
	}

	payload := MenuBarMetricsPayload{
		SysInfo:        sysInfo,
		CPUMetrics:     cpuMetrics,
		GPUMetrics:     gpuMetrics,
		NetDiskMetrics: netDisk,
		MemMetrics:     getMemoryMetrics(),
		TFLOPs:         maxFP32TFLOPs,
		CPUPercent:     cpuPercent,
		ThermalState:   thermalState,
		ThermalLevel:   int(getThermalStateLevel()),
		RDMAStatus:     rdmaStatus,
		TotalPower:     sm.TotalPower,
	}

	if err := overlayMetricsEncoder.Encode(payload); err != nil {
		// Worker likely died — attempt restart with cooldown.
		// Clean up the broken encoder/pipe first.
		overlayMetricsEncoder = nil
		if overlayWorkerStdin != nil {
			overlayWorkerStdin.Close()
			overlayWorkerStdin = nil
		}
		if overlayWorkerCmd != nil && overlayWorkerCmd.Process != nil {
			overlayWorkerCmd.Process.Kill()
			overlayWorkerCmd = nil
		}

		if time.Since(overlayLastRestart) < 2*time.Second {
			return // Too soon, skip — next call will re-check cooldown
		}
		stderrLogger.Printf("Overlay worker pipe broken, restarting: %v\n", err)
		overlayLastRestart = time.Now()

		if restartErr := startOverlayProcess(); restartErr != nil {
			stderrLogger.Printf("Failed to restart overlay worker: %v\n", restartErr)
		}
	}
}
