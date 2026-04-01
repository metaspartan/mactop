// Copyright (c) 2024-2026 Carsen Klock under MIT License
// menubar.go - Go wrappers for native macOS menu bar status item
package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

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
} menubar_metrics_t;

typedef struct {
    int status_bar_width;
    int status_bar_height;
    int sparkline_width;
    int sparkline_height;
    int show_cpu;
    int show_gpu;
    int show_ane;
    int show_memory;
    int show_power;
    int show_percent;
    int font_size;
    int power_font_size;
    char cpu_color[8];
    char gpu_color[8];
    char ane_color[8];
    char mem_color[8];
} menubar_config_t;

int initMenuBar(void);
void setMenuBarConfig(menubar_config_t *cfg);
void updateMenuBarMetrics(menubar_metrics_t *m);
void runMenuBarLoop(void);
void cleanupMenuBar(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/metaspartan/mactop/v2/internal/i18n"
)

// MenuBarMetricsPayload is the JSON structure sent from the main process
// to the menubar worker process via stdin.
type MenuBarMetricsPayload struct {
	SysInfo        SystemInfo     `json:"sys_info"`
	CPUMetrics     CPUMetrics     `json:"cpu_metrics"`
	GPUMetrics     GPUMetrics     `json:"gpu_metrics"`
	NetDiskMetrics NetDiskMetrics `json:"net_disk_metrics"`
	MemMetrics     MemoryMetrics  `json:"mem_metrics"`
	TFLOPs         float64        `json:"tflops"`
	CPUPercent     float64        `json:"cpu_percent"`
	ThermalState   string         `json:"thermal_state"`
	RDMAStatus     string         `json:"rdma_status"`
	TotalPower     float64        `json:"total_power"`
}

// Cached values for TUI+menubar dual mode
var (
	cachedMenuBarSysInfo  SystemInfo
	cachedMenuBarTFLOPs   float64
	menubarMetricsEncoder *json.Encoder
	menubarWorkerCmd      *exec.Cmd
	menubarWorkerStdin    io.WriteCloser
	menubarMu             sync.Mutex
	menubarLastRestart    time.Time
)

func boolToInt(b *bool, defaultVal C.int) C.int {
	if b == nil {
		return defaultVal
	}
	if *b {
		return 1
	}
	return 0
}

func copyColorToCBuf(dst *[8]C.char, hex string) {
	b := []byte(hex)
	if len(b) > 7 {
		b = b[:7]
	}
	for i, c := range b {
		dst[i] = C.char(c)
	}
	dst[len(b)] = 0
}

func applyMenuBarConfig() {
	mbCfg := loadMenuBarConfig()
	var ccfg C.menubar_config_t
	ccfg.status_bar_width = C.int(mbCfg.StatusBarWidth)
	ccfg.status_bar_height = C.int(mbCfg.StatusBarHeight)
	ccfg.sparkline_width = C.int(mbCfg.SparklineWidth)
	ccfg.sparkline_height = C.int(mbCfg.SparklineHeight)
	ccfg.show_cpu = boolToInt(mbCfg.ShowCPU, 1)
	ccfg.show_gpu = boolToInt(mbCfg.ShowGPU, 1)
	ccfg.show_ane = boolToInt(mbCfg.ShowANE, 1)
	ccfg.show_memory = boolToInt(mbCfg.ShowMemory, 0)
	ccfg.show_power = boolToInt(mbCfg.ShowPower, 1)
	ccfg.show_percent = boolToInt(mbCfg.ShowPercent, 0)
	ccfg.font_size = C.int(mbCfg.FontSize)
	ccfg.power_font_size = C.int(mbCfg.PowerFontSize)
	copyColorToCBuf(&ccfg.cpu_color, mbCfg.CPUColor)
	copyColorToCBuf(&ccfg.gpu_color, mbCfg.GPUColor)
	copyColorToCBuf(&ccfg.ane_color, mbCfg.ANEColor)
	copyColorToCBuf(&ccfg.mem_color, mbCfg.MemColor)
	C.setMenuBarConfig(&ccfg)
}

// cBoolToPtr converts a C int (0/1) to a Go *bool
func cBoolToPtr(v C.int) *bool {
	b := v != 0
	return &b
}

// GoSaveMenuBarConfig is called from ObjC when settings change
//
//export GoSaveMenuBarConfig
func GoSaveMenuBarConfig(statusBarWidth, statusBarHeight, sparklineWidth, sparklineHeight, showCPU, showGPU, showANE, showMem, showPower, showPercent,
	fontSize, powerFontSize C.int,
	cpuHex, gpuHex, aneHex, memHex *C.char) {
	if currentConfig.MenuBar == nil {
		currentConfig.MenuBar = &MenuBarConfig{}
	}
	m := currentConfig.MenuBar

	if statusBarWidth > 0 {
		m.StatusBarWidth = int(statusBarWidth)
	}
	if statusBarHeight > 0 {
		m.StatusBarHeight = int(statusBarHeight)
	}
	if sparklineWidth > 0 {
		m.SparklineWidth = int(sparklineWidth)
	}
	if sparklineHeight > 0 {
		m.SparklineHeight = int(sparklineHeight)
	}
	m.ShowCPU = cBoolToPtr(showCPU)
	m.ShowGPU = cBoolToPtr(showGPU)
	m.ShowANE = cBoolToPtr(showANE)
	m.ShowMemory = cBoolToPtr(showMem)
	m.ShowPower = cBoolToPtr(showPower)
	m.ShowPercent = cBoolToPtr(showPercent)

	if fontSize > 0 {
		m.FontSize = int(fontSize)
	}
	if powerFontSize > 0 {
		m.PowerFontSize = int(powerFontSize)
	}

	if cpuHex != nil {
		m.CPUColor = C.GoString(cpuHex)
	}
	if gpuHex != nil {
		m.GPUColor = C.GoString(gpuHex)
	}
	if aneHex != nil {
		m.ANEColor = C.GoString(aneHex)
	}
	if memHex != nil {
		m.MemColor = C.GoString(memHex)
	}

	saveConfig()
}

// GoI18nT provides direct translation lookup to Objective-C
//export GoI18nT
func GoI18nT(id *C.char) *C.char {
	goID := C.GoString(id)
	translated := i18n.T(goID)
	// Return malloc'd string (caller must free)
	return C.CString(translated)
}

// startMenuBarWorker is the entry point for the child process (--menubar-worker).
// It reads JSON metrics from stdin and updates the menu bar on the main thread.
func startMenuBarWorker() {
	runtime.LockOSThread()

	// Load config in the worker process so defaults/persistence works
	loadConfig()
	applyMenuBarConfig()

	// Initialize AppKit
	if ret := C.initMenuBar(); ret != 0 {
		fmt.Fprintf(os.Stderr, "Failed to initialize menu bar worker: %d\n", int(ret))
		os.Exit(1)
	}

	// We run the JSON decoder in a goroutine to feed updates.
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
				// If stdin closes (parent dies), exit the worker
				os.Exit(0)
				return
			}

			// Update cached values needed for logic
			cachedMenuBarSysInfo = payload.SysInfo
			cachedMenuBarTFLOPs = payload.TFLOPs

			updateMenuBarFromPayload(payload)
		}
	}()

	applyMenuBarConfig()
	// This blocks on [NSApp run]
	C.runMenuBarLoop()
	C.cleanupMenuBar()
}

// updateMenuBarFromPayload updates the C struct from the decoded JSON payload
func updateMenuBarFromPayload(p MenuBarMetricsPayload) {
	var cm C.menubar_metrics_t

	// CPU Loading
	cm.cpu_percent = C.double(p.CPUPercent)
	cm.gpu_percent = C.double(p.GPUMetrics.ActivePercent)

	anePct := p.CPUMetrics.ANEW / 8.0 * 100
	if anePct > 100 {
		anePct = 100
	}
	cm.ane_percent = C.double(anePct) // Power-based estimation (same as TUI)

	cm.mem_used_bytes = C.ulonglong(p.MemMetrics.Used)
	cm.mem_total_bytes = C.ulonglong(p.MemMetrics.Total)
	cm.swap_used_bytes = C.ulonglong(p.MemMetrics.SwapUsed)
	cm.swap_total_bytes = C.ulonglong(p.MemMetrics.SwapTotal)

	cm.cpu_watts = C.double(p.CPUMetrics.CPUW)
	cm.gpu_watts = C.double(p.GPUMetrics.Power)
	cm.ane_watts = C.double(p.CPUMetrics.ANEW)
	cm.dram_watts = C.double(p.CPUMetrics.DRAMW)
	cm.package_watts = C.double(p.TotalPower)
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

	// TFLOPs
	cm.tflops_fp32 = C.double(p.TFLOPs)

	// DRAM Bandwidth
	cm.dram_bw_combined_gbs = C.double(p.CPUMetrics.DRAMBWCombined)

	// SysInfo Mapping
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

	C.updateMenuBarMetrics((*C.menubar_metrics_t)(unsafe.Pointer(&cm)))
}

// startMenuBarProcess spawns the worker process and sets up the pipe.
func startMenuBarProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	cmd := exec.Command(exe, "--menubar-worker")
	cmd.Env = append(os.Environ(), "MACTOP_LANG="+cliLanguage)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %v", err)
	}

	menubarWorkerCmd = cmd
	menubarWorkerStdin = stdin
	menubarMetricsEncoder = json.NewEncoder(stdin)

	// Wait for worker in background
	go func() {
		cmd.Wait()
	}()

	return nil
}

// pushMenuBarMetricsToWorker sends metrics to the child process.
// If the worker has died, it automatically restarts with a 2s cooldown.
func pushMenuBarMetricsToWorker(sm SocMetrics, cpuMetrics CPUMetrics, gpuMetrics GPUMetrics, netDisk NetDiskMetrics, sysInfo SystemInfo, maxFP32TFLOPs float64, cpuPercent float64, thermalState string, rdmaStatus string) {
	menubarMu.Lock()
	defer menubarMu.Unlock()

	if menubarMetricsEncoder == nil {
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
		RDMAStatus:     rdmaStatus,
		TotalPower:     sm.TotalPower,
	}

	if err := menubarMetricsEncoder.Encode(payload); err != nil {
		// Worker likely died — attempt restart with cooldown
		if time.Since(menubarLastRestart) < 2*time.Second {
			return // Too soon, skip this cycle
		}
		stderrLogger.Printf("Menubar worker pipe broken, restarting: %v\n", err)
		menubarLastRestart = time.Now()

		// Clean up old resources
		if menubarWorkerStdin != nil {
			menubarWorkerStdin.Close()
		}
		if menubarWorkerCmd != nil && menubarWorkerCmd.Process != nil {
			menubarWorkerCmd.Process.Kill()
		}
		menubarMetricsEncoder = nil

		if restartErr := startMenuBarProcess(); restartErr != nil {
			stderrLogger.Printf("Failed to restart menubar worker: %v\n", restartErr)
		}
	}
}
