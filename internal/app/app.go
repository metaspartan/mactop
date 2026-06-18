// Copyright (c) 2024-2026 Carsen Klock under MIT License
// mactop is a simple terminal based Apple Silicon power monitor written in Go Lang! github.com/metaspartan/mactop
package app

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
	"github.com/metaspartan/mactop/v2/internal/i18n"
)

var (
	renderMutex   sync.Mutex
	menubarWorker bool // Hidden flag for the worker process
)

func setupUI() {
	appleSiliconModel := getSOCInfo()
	modelText, helpText, infoParagraph = w.NewParagraph(), w.NewParagraph(), w.NewParagraph()
	fanStatusPanel, fanTempPanel, fanControlPanel = w.NewParagraph(), w.NewParagraph(), w.NewParagraph()
	modelText.Title = i18n.T("TUI_AppleSilicon")
	helpText.Title = i18n.T("TUI_HelpMenu")
	infoParagraph.Text = i18n.T("TUI_Loading")
	fanStatusPanel.Title = i18n.T("TUI_Fans")
	fanStatusPanel.BorderRounded = true
	fanTempPanel.Title = i18n.T("TUI_Temperatures")
	fanTempPanel.BorderRounded = true
	fanControlPanel.Title = ""
	fanControlPanel.BorderRounded = true
	modelName := appleSiliconModel.Name
	if modelName == "" {
		modelName = i18n.T("TUI_UnknownModel")
	}

	cachedHostname, _ = os.Hostname()
	cachedCurrentUser = os.Getenv("USER")
	cachedShell = os.Getenv("SHELL")

	cachedKernelVersion, _ = sysctlStringByName("kern.osrelease")
	cachedOSVersion, _ = sysctlStringByName("kern.osproductversion")

	cachedModelName = modelName
	cachedSystemInfo = appleSiliconModel
	eCoreCount := appleSiliconModel.ECoreCount
	pCoreCount := appleSiliconModel.PCoreCount
	sCoreCount := appleSiliconModel.SCoreCount
	gpuCoreCount := appleSiliconModel.GPUCoreCount
	updateModelText()
	updateHelpText()
	stderrLogger.Printf("Model: %s\nE-Core Count: %d\nP-Core Count: %d\nS-Core Count: %d\nGPU Core Count: %d", modelName, eCoreCount, pCoreCount, sCoreCount, gpuCoreCount)

	processList = w.NewList()
	processList.Title = i18n.T("TUI_ProcessList")
	processList.TextStyle = ui.NewStyle(ui.ColorGreen)
	processList.WrapText = false
	processList.SelectedStyle = ui.NewStyle(ui.ColorBlack, ui.ColorGreen)
	processList.Rows = []string{}
	processList.SelectedRow = 0

	gauges := []*w.Gauge{
		w.NewGauge(), w.NewGauge(), w.NewGauge(), w.NewGauge(),
	}
	for _, gauge := range gauges {
		gauge.Percent = 0
	}
	cpuGauge, gpuGauge, memoryGauge, aneGauge = gauges[0], gauges[1], gauges[2], gauges[3]

	cpuGauge.Title = i18n.T("TUI_Loading")
	gpuGauge.Title = i18n.T("TUI_GPUUsage")
	memoryGauge.Title = i18n.T("TUI_MemoryUsage")
	aneGauge.Title = i18n.T("TUI_ANEUsage")

	PowerChart, NetworkInfo = w.NewParagraph(), w.NewParagraph()
	PowerChart.Title, NetworkInfo.Title = i18n.T("TUI_PowerUsage"), i18n.T("TUI_NetworkDisk")

	tbInfoParagraph = w.NewParagraph()
	tbInfoParagraph.Title = i18n.T("TUI_ThunderboltRDMA")
	tbInfoParagraph.Text = i18n.T("TUI_LoadingTB")
	go func() {
		description := GetThunderboltDescription()
		tbInfoMutex.Lock()
		tbDeviceInfo = description
		tbInfoMutex.Unlock()
	}()

	mainBlock = ui.NewBlock()
	mainBlock.BorderRounded = true
	mainBlock.TitleRight = " " + version + " "
	mainBlock.TitleAlignment = ui.AlignLeft
	mainBlock.TitleBottomLeft = fmt.Sprintf(i18n.T("TUI_LayoutInfo"), currentLayoutNum, totalLayouts, currentColorName)
	mainBlock.TitleBottom = i18n.T("TUI_InfoLayoutColorExit")
	mainBlock.TitleBottomAlignment = ui.AlignCenter
	mainBlock.TitleBottomRight = fmt.Sprintf(" -/+ %dms ", updateInterval)

	updateMainTitleWithHardware()

	termWidth, termHeight := ui.TerminalDimensions()
	UpdateCachedTerminalDimensions(termWidth, termHeight)
	// Use full terminal width for StepChart data buffers (old sparkline sizing used half)
	numPoints := max(termWidth,
		// Minimum buffer size
		500)

	powerValues = make([]float64, numPoints)
	gpuValues = make([]float64, numPoints)
	memoryUsedHistory = make([]float64, numPoints)
	swapUsedHistory = make([]float64, numPoints)
	cpuUsageHistory = make([]float64, numPoints)
	powerUsageHistory = make([]float64, numPoints)
	memBWReadHistory = make([]float64, numPoints)
	memBWWriteHistory = make([]float64, numPoints)
	aneUsageHistory = make([]float64, numPoints)
	aneCluster0History = make([]float64, numPoints)
	aneCluster1History = make([]float64, numPoints)
	dramReadHistory = make([]float64, numPoints)
	dramWriteHistory = make([]float64, numPoints)
	aneReadBwHistory = make([]float64, numPoints)
	aneWriteBwHistory = make([]float64, numPoints)

	// Per-block power histories for history_soc layout
	cpuPowerHistory = make([]float64, numPoints)
	gpuPowerHistory = make([]float64, numPoints)
	anePowerHistory = make([]float64, numPoints)
	dramPowerHistory = make([]float64, numPoints)

	// Peak + Average for SoC usage histories
	cpuPeakHistory = make([]float64, numPoints)
	gpuPeakHistory = make([]float64, numPoints)
	anePeakHistory = make([]float64, numPoints)
	bwPeakHistory = make([]float64, numPoints)

	// Effective GPU load history — recorded at sample time using the frequency of that moment.
	// This is the correct way so the graph doesn't jump when frequency changes.
	gpuEffectiveHistory = make([]float64, numPoints)

	// SSD read history for history_soc bottom section
	ssdReadHistory = make([]float64, numPoints)

	sparkline = w.NewSparkline()
	sparkline.MaxHeight = 100
	sparkline.Data = powerValues

	sparklineGroup = w.NewSparklineGroup(sparkline)

	gpuSparkline = w.NewSparkline()
	gpuSparkline.MaxHeight = 100
	gpuSparkline.Data = gpuValues
	gpuSparklineGroup = w.NewSparklineGroup(gpuSparkline)
	gpuSparklineGroup.Title = i18n.T("TUI_GPUUsageHistory")

	// TB Net sparklines
	tbNetSparklineIn = w.NewSparkline()
	tbNetSparklineIn.Data = tbNetInValues
	tbNetSparklineIn.LineColor = ui.ColorGreen
	tbNetSparklineIn.TitleStyle.Fg = ui.ColorGreen

	tbNetSparklineOut = w.NewSparkline()
	tbNetSparklineOut.Data = tbNetOutValues
	tbNetSparklineOut.LineColor = ui.ColorMagenta
	tbNetSparklineOut.TitleStyle.Fg = ui.ColorMagenta

	tbNetSparklineGroup = w.NewSparklineGroup(tbNetSparklineIn, tbNetSparklineOut)
	tbNetSparklineGroup.Title = i18n.T("TUI_TBNet")

	// StepChart widgets for History layout
	gpuHistoryChart = w.NewStepChart()
	gpuHistoryChart.Title = i18n.T("TUI_GPUUsageHistory")
	gpuHistoryChart.ShowAxes = false
	gpuHistoryChart.ShowRightAxis = true
	gpuHistoryChart.LineColors = []ui.Color{ui.ColorGreen}

	powerHistoryChart = w.NewStepChart()
	powerHistoryChart.Title = i18n.T("TUI_PowerHistory")
	powerHistoryChart.ShowAxes = false
	powerHistoryChart.ShowRightAxis = true
	powerHistoryChart.LineColors = []ui.Color{ui.ColorYellow}

	memoryHistoryChart = w.NewStepChart()
	memoryHistoryChart.Title = i18n.T("TUI_MemorySwapHistory")
	memoryHistoryChart.ShowAxes = false
	memoryHistoryChart.ShowRightAxis = true
	memoryHistoryChart.LineColors = []ui.Color{ui.ColorBlue, ui.ColorMagenta}

	cpuHistoryChart = w.NewStepChart()
	cpuHistoryChart.Title = i18n.T("TUI_CPUUsageHistory")
	cpuHistoryChart.ShowAxes = false
	cpuHistoryChart.ShowRightAxis = true
	cpuHistoryChart.LineColors = []ui.Color{ui.ColorGreen}

	memBWHistoryChart = w.NewStepChart()
	memBWHistoryChart.Title = i18n.T("TUI_MemoryBandwidthHistory")
	memBWHistoryChart.ShowAxes = false
	memBWHistoryChart.ShowRightAxis = true
	memBWHistoryChart.LineColors = []ui.Color{ui.ColorCyan, ui.ColorMagenta}

	aneHistoryChart = w.NewStepChart()
	aneHistoryChart.Title = i18n.T("TUI_ANEUsageHistory")
	aneHistoryChart.ShowAxes = false
	aneHistoryChart.ShowRightAxis = true
	aneHistoryChart.LineColors = []ui.Color{ui.ColorMagenta}

	bandwidthHistoryChart = w.NewStepChart()
	bandwidthHistoryChart.Title = i18n.T("TUI_DRAMBandwidthHistory")
	bandwidthHistoryChart.ShowAxes = false
	bandwidthHistoryChart.ShowRightAxis = true
	bandwidthHistoryChart.LineColors = []ui.Color{ui.ColorCyan, ui.ColorYellow} // read, write

	// Multi-component power history for the SoC layout
	socPowerHistoryChart = w.NewStepChart()
	socPowerHistoryChart.Title = i18n.T("TUI_SoCPowerHistory")
	socPowerHistoryChart.ShowAxes = false
	socPowerHistoryChart.ShowRightAxis = true
	socPowerHistoryChart.LineColors = []ui.Color{ui.ColorGreen, ui.ColorBlue, ui.ColorMagenta, ui.ColorYellow}

	ssdReadHistoryChart = w.NewStepChart()
	ssdReadHistoryChart.Title = i18n.T("TUI_SSDReadHistory")
	ssdReadHistoryChart.ShowAxes = false
	ssdReadHistoryChart.ShowRightAxis = true
	ssdReadHistoryChart.LineColors = []ui.Color{ui.ColorCyan}

	cpuCoreWidget = NewCPUCoreWidget(appleSiliconModel)
	coreSummary := FormatCoreSummary(cpuCoreWidget.eCoreCount, cpuCoreWidget.pCoreCount, cpuCoreWidget.sCoreCount)
	totalCPUCores := cpuCoreWidget.eCoreCount + cpuCoreWidget.pCoreCount + cpuCoreWidget.sCoreCount
	coreTitle := fmt.Sprintf(i18n.T("TUI_Cores"), totalCPUCores)
	if coreSummary != "" {
		coreTitle = fmt.Sprintf("%s %s", coreTitle, coreSummary)
	}
	cpuCoreWidget.Title = coreTitle
	cpuGauge.Title = coreTitle

	confirmModal = w.NewModal(i18n.T("TUI_ConfirmKillBody"))
	confirmModal.Title = i18n.T("TUI_ConfirmKill")
	confirmModal.Border = true
	confirmModal.BorderRounded = true
	confirmModal.BorderStyle.Fg = ui.ColorRed
	confirmModal.BorderStyle.Bg = ui.ColorBlack
	confirmModal.TextStyle.Fg = ui.ColorWhite
	confirmModal.TextStyle.Bg = ui.ColorBlack
	confirmModal.ActiveButtonIndex = 1 // Default to No (Safe)

	_ = confirmModal.AddButton(i18n.T("TUI_ConfirmYes"), func() {
		// Callback logic will be handled elsewhere or reused
	})
	_ = confirmModal.AddButton(i18n.T("TUI_ConfirmNo"), func() {
		// Callback logic
	})
}

func updateMainTitleWithHardware() {
	info := getSOCInfo()

	model := info.Name
	if model == "" {
		model = i18n.T("TUI_UnknownModel")
	}

	// CPU cores string (with E/P/S breakdown when available)
	cpuStr := fmt.Sprintf("%dC", info.CoreCount)
	if info.ECoreCount > 0 || info.PCoreCount > 0 || info.SCoreCount > 0 {
		parts := []string{}
		if info.ECoreCount > 0 {
			parts = append(parts, fmt.Sprintf("%dE", info.ECoreCount))
		}
		if info.PCoreCount > 0 {
			parts = append(parts, fmt.Sprintf("%dP", info.PCoreCount))
		}
		if info.SCoreCount > 0 {
			parts = append(parts, fmt.Sprintf("%dS", info.SCoreCount))
		}
		if len(parts) > 0 {
			cpuStr = fmt.Sprintf("%dC (%s)", info.CoreCount, strings.Join(parts, "+"))
		}
	}

	gpuStr := ""
	if info.GPUCoreCount > 0 {
		gpuStr = fmt.Sprintf(" • %d GPU", info.GPUCoreCount)
	}

	ramStr := ""
	ramGB := getTotalRAMGB()
	if ramGB > 0 {
		ramStr = fmt.Sprintf(" • %d GB", ramGB)
	}

	mainBlock.Title = fmt.Sprintf(" mactop  •  %s  •  %s%s%s ", model, cpuStr, gpuStr, ramStr)
}

func updateModelText() {
	appleSiliconModel := getSOCInfo()
	modelName := appleSiliconModel.Name
	if modelName == "" {
		modelName = i18n.T("TUI_UnknownModel")
	}
	eCoreCount := appleSiliconModel.ECoreCount
	pCoreCount := appleSiliconModel.PCoreCount
	sCoreCount := appleSiliconModel.SCoreCount
	gpuCoreCount := appleSiliconModel.GPUCoreCount

	gpuCoreCountStr := "?"
	if gpuCoreCount > 0 {
		gpuCoreCountStr = fmt.Sprintf("%d", gpuCoreCount)
	}

	totalCores := eCoreCount + pCoreCount + sCoreCount
	var coreLines string
	cBase := i18n.T("TUI_Cores")
	cE := i18n.T("TUI_ECores")
	cP := i18n.T("TUI_PCores")
	cS := i18n.T("TUI_SCores")

	if eCoreCount > 0 && sCoreCount > 0 {
		coreLines = fmt.Sprintf(cBase+"\n"+cE+"\n"+cP+"\n"+cS,
			totalCores, eCoreCount, pCoreCount, sCoreCount)
	} else if sCoreCount > 0 {
		coreLines = fmt.Sprintf(cBase+"\n"+cP+"\n"+cS,
			totalCores, pCoreCount, sCoreCount)
	} else if eCoreCount > 0 {
		coreLines = fmt.Sprintf(cBase+"\n"+cE+"\n"+cP,
			totalCores, eCoreCount, pCoreCount)
	} else {
		coreLines = fmt.Sprintf(cBase+"\n"+cP,
			totalCores, pCoreCount)
	}

	modelText.Text = fmt.Sprintf("%s\n%s\n%s",
		modelName,
		coreLines,
		fmt.Sprintf(i18n.T("TUI_GPUCores"), gpuCoreCountStr),
	)
}

func updateIntervalText() {
	mainBlock.TitleBottomRight = fmt.Sprintf(" -/+ %dms ", updateInterval)
}

func updateInfoUI() {
	if currentConfig.DefaultLayout == LayoutFan {
		themeColor := "green"
		if currentConfig.Theme != "" {
			themeColor = currentConfig.Theme
		}
		if IsLightMode && themeColor == "white" {
			themeColor = "black"
		}
		tc := GetThemeColor(themeColor)

		fanStatusPanel.Text = buildFanStatusText(themeColor)
		fanTempPanel.Text = buildFanTempText(themeColor)
		fanControlPanel.Text = buildFanControlText(themeColor)

		for _, p := range []*w.Paragraph{fanStatusPanel, fanTempPanel, fanControlPanel} {
			p.BorderStyle.Fg = tc
			p.TitleStyle.Fg = tc
		}
		mainBlock.BorderStyle.Fg = tc
		mainBlock.TitleStyle.Fg = tc
		return
	}

	if currentConfig.DefaultLayout != LayoutInfo {
		return
	}

	infoParagraph.Text = buildInfoText()
	infoParagraph.BorderRounded = true

	themeColor := "green"
	if currentConfig.Theme != "" {
		themeColor = currentConfig.Theme
	}
	if IsLightMode && themeColor == "white" {
		themeColor = "black"
	}
	tc := GetThemeColor(themeColor)

	infoParagraph.BorderStyle.Fg = tc
	infoParagraph.TitleStyle.Fg = tc

	mainBlock.BorderStyle.Fg = tc
	mainBlock.TitleStyle.Fg = tc
}

func updateHelpText() {
	prometheusStatus := "Disabled"
	if prometheusPort != "" {
		prometheusStatus = fmt.Sprintf("Enabled (Port: %s)", prometheusPort)
	}
	fullText := fmt.Sprintf(
		i18n.T("Help_FullText"),
		prometheusStatus,
		version,
		currentConfig.DefaultLayout,
		currentConfig.Theme,
		currentConfig.Background,
		updateInterval,
	)

	lines := strings.Split(fullText, "\n")
	_, termHeight := GetCachedTerminalDimensions()

	// Determine if we need scrolling
	// First calculate raw available height minus borders
	rawHeight := max(termHeight-2, 1)

	availableHeight := rawHeight
	maxOffset := 0

	// If content doesn't fit, we need to reserve space for indicators
	if len(lines) > rawHeight {
		// Reserve 2 lines (1 for top indicator/spacer, 1 for bottom indicator/spacer)
		availableHeight = max(rawHeight-2, 1)
		maxOffset = len(lines) - availableHeight
	}

	if helpScrollOffset > maxOffset {
		helpScrollOffset = maxOffset
	}
	if helpScrollOffset < 0 {
		helpScrollOffset = 0
	}

	start := helpScrollOffset
	end := min(start+availableHeight, len(lines))

	visibleLines := lines[start:end]

	var finalBuilder strings.Builder
	tc := getThemeColor()

	// Top indicator (only if scrolling is active)
	if maxOffset > 0 {
		if helpScrollOffset > 0 {
			fmt.Fprintf(&finalBuilder, "[%s (k/↑)](fg:%s)\n", i18n.T("Info_ScrollUp"), tc)
		} else {
			finalBuilder.WriteString("\n") // Spacer
		}
	}

	// Content
	finalBuilder.WriteString(strings.Join(visibleLines, "\n"))

	// Bottom indicator (only if scrolling is active)
	if maxOffset > 0 {
		if helpScrollOffset < maxOffset {
			fmt.Fprintf(&finalBuilder, "\n[%s (j/↓)](fg:%s)", i18n.T("Info_ScrollDown"), tc)
		} else {
			finalBuilder.WriteString("\n") // Spacer
		}
	}

	helpText.Text = finalBuilder.String()
}

func toggleHelpMenu() {
	showHelp = !showHelp
	if showHelp {
		helpScrollOffset = 0
	}
	updateHelpText()

	renderMutex.Lock()
	defer renderMutex.Unlock()

	if showHelp {
		newGrid := ui.NewGrid()
		newGrid.Set(
			ui.NewRow(1.0,
				ui.NewCol(1.0, helpText),
			),
		)
		termWidth, termHeight := ui.TerminalDimensions()
		helpTextGridWidth := termWidth
		helpTextGridHeight := termHeight
		x := (termWidth - helpTextGridWidth) / 2
		y := (termHeight - helpTextGridHeight) / 2
		newGrid.SetRect(x, y, x+helpTextGridWidth, y+helpTextGridHeight)
		grid = newGrid
	} else {
		applyLayout(currentConfig.DefaultLayout)
	}
	ui.Clear()
	width, height := ui.TerminalDimensions()
	if width > 2 && height > 2 {
		ui.Render(mainBlock, grid)
	} else {
		ui.Render(mainBlock)
	}
}

func togglePartyMode() {
	partyMode = !partyMode
	if partyMode {
		partyTicker = time.NewTicker(time.Duration(updateInterval/2) * time.Millisecond)
		go func() {
			for range partyTicker.C {
				if !partyMode {
					partyTicker.Stop()
					return
				}
				cycleTheme(1)
				renderMutex.Lock()
				updateProcessList()
				width, height := ui.TerminalDimensions()
				ui.Clear()
				if width > 2 && height > 2 {
					ui.Render(mainBlock, grid)
				} else {
					ui.Render(mainBlock)
				}
				renderMutex.Unlock()
			}
		}()
	} else if partyTicker != nil {
		partyTicker.Stop()
	}
}

func renderUI() {
	renderMutex.Lock()
	defer renderMutex.Unlock()
	w, h := ui.TerminalDimensions()
	if w > 2 && h > 2 {
		if killPending {
			ui.Render(mainBlock, grid, confirmModal) // Render on top
		} else {
			ui.Render(mainBlock, grid)
		}
	} else {
		ui.Render(mainBlock)
	}
}

func applyInitialTheme(colorName string, setColor bool) {
	if setColor {
		applyTheme(colorName, IsLightMode)
	} else {
		if currentConfig.Theme == "" {
			currentConfig.Theme = "green"
		}
		applyTheme(currentConfig.Theme, IsLightMode)
	}
}

// initializeTheme sets up all theming with priority: CLI flags > theme.json > saved config
// Each property (foreground, background) is evaluated independently
func initializeTheme(colorName string, setColor bool, interval int, setInterval bool) {
	// Interval priority: 1) CLI --interval, 2) saved config, 3) default 1000ms
	if setInterval {
		updateInterval = interval
		currentConfig.Interval = interval
		updateIntervalText()
	} else if currentConfig.Interval > 0 {
		updateInterval = currentConfig.Interval
		updateIntervalText()
	}

	// Always load theme.json to get both foreground and background values
	// We'll selectively apply based on CLI flag priorities
	fgFromFile, bgFromFile := applyCustomThemeFile()

	// Foreground priority: 1) CLI --foreground, 2) theme.json, 3) saved config
	if setColor {
		applyTheme(colorName, IsLightMode)
	} else if !fgFromFile {
		// Neither CLI nor theme.json set foreground, use saved config
		applyInitialTheme(colorName, false)
	}
	// else: theme.json foreground was already applied by applyCustomThemeFile()

	// Background priority: 1) CLI --bg, 2) theme.json, 3) saved config
	if cliBgColor != "" {
		applyBackground(cliBgColor)
		currentConfig.Background = cliBgColor
	} else if !bgFromFile {
		// Neither CLI nor theme.json set background, use saved config
		applyInitialBackground()
	}
	// else: theme.json background was already applied by applyCustomThemeFile()

	currentColorName = currentConfig.Theme
}

// runAlternateMode checks for non-TUI modes and runs them.
// Returns true if an alternate mode was handled (caller should return).
func runAlternateMode() bool {
	if dumpTemps {
		if err := initSocMetrics(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize metrics: %v\n", err)
			os.Exit(1)
		}
		defer cleanupSocMetrics()
		sysInfo := getSOCInfo()
		fmt.Printf("System: %s\n", sysInfo.Name)
		fmt.Printf("Cores: %d E + %d P + %d S = %d total\n",
			sysInfo.ECoreCount, sysInfo.PCoreCount, sysInfo.SCoreCount, sysInfo.CoreCount)
		fmt.Printf("GPU Cores: %d\n\n", sysInfo.GPUCoreCount)
		DumpAllSMCTemps()
		return true
	}
	if dumpDebug {
		sysInfo := getSOCInfo()
		fmt.Printf("System: %s\n", sysInfo.Name)
		fmt.Printf("Cores: %d E + %d P + %d S = %d total\n",
			sysInfo.ECoreCount, sysInfo.PCoreCount, sysInfo.SCoreCount, sysInfo.CoreCount)
		fmt.Printf("GPU Cores: %d\n\n", sysInfo.GPUCoreCount)
		DumpIOReportDebug()
		return true
	}
	if dumpFPS {
		DumpDisplayFPSDiagnostics()
		return true
	}
	if menubarWorker {
		startMenuBarWorker()
		return true
	}
	if overlayWorker {
		startOverlayWorker()
		return true
	}
	if headless {
		runHeadless(headlessCount)
		return true
	}
	return false
}

// renderLoadingScreen shows a branded loading message centered on screen.
// Called immediately after ui.Init() to give instant visual feedback while
// metrics subsystems initialize in the background.
func renderLoadingScreen() {
	termWidth, termHeight := ui.TerminalDimensions()

	loadingBlock := ui.NewBlock()
	loadingBlock.BorderRounded = true
	loadingBlock.Title = i18n.T("TUI_MactopTitle")
	loadingBlock.TitleRight = " " + version + " "
	loadingBlock.TitleAlignment = ui.AlignLeft
	loadingBlock.BorderStyle = ui.NewStyle(ui.ColorGreen)
	loadingBlock.TitleStyle = ui.NewStyle(ui.ColorGreen)
	loadingBlock.SetRect(0, 0, termWidth, termHeight)

	loadingText := w.NewParagraph()
	loadingText.Border = false

	// Build vertically centered text: pad with newlines to reach middle
	innerHeight := termHeight - 2 // subtract outer block borders
	var topPad strings.Builder
	if innerHeight > 3 {
		for i := 0; i < (innerHeight/2)-1; i++ {
			topPad.WriteString("\n")
		}
	}

	// Horizontally center the loading text manually with spaces
	msg := i18n.T("TUI_Loading")
	msgWidth := runewidth.StringWidth(msg)
	innerWidth := termWidth - 2 // subtract outer block borders
	var leftPad strings.Builder
	if innerWidth > msgWidth {
		for i := 0; i < (innerWidth-msgWidth)/2; i++ {
			leftPad.WriteString(" ")
		}
	}

	loadingText.Text = topPad.String() + leftPad.String() + msg
	loadingText.TextStyle = ui.NewStyle(ui.ColorGreen)
	loadingText.SetRect(1, 1, termWidth-1, termHeight-1)

	ui.Clear()
	ui.Render(loadingBlock, loadingText)
}

// drainSeededMetrics consumes the initial metrics pushed by seedInitialMetrics
// and populates all UI widgets so the first render shows real data.
func drainSeededMetrics() {
	select {
	case cpuMetrics := <-cpuMetricsChan:
		lastCPUMetrics = cpuMetrics
		updateCPUUI(cpuMetrics)
		updateTotalPowerChart(cpuMetrics.PackageW)
	default:
	}
	select {
	case gpuMetrics := <-gpuMetricsChan:
		lastGPUMetrics = gpuMetrics
		updateGPUUI(gpuMetrics)
	default:
	}
	select {
	case netdiskMetrics := <-netdiskMetricsChan:
		lastNetDiskMetrics = netdiskMetrics
		updateNetDiskUI(netdiskMetrics)
	default:
	}
	select {
	case processes := <-processMetricsChan:
		lastProcesses = processes
		updateProcessList()
	default:
	}
}

// seedInitialMetrics takes a quick sample and pushes initial values into the metric channels.
func seedInitialMetrics() {
	m := normalizeSocMetricsPower(sampleSocMetrics(50))
	thermalLevel := getThermalStateLevel()
	coreUsages, _ := GetCPUPercentages()
	cpuMetrics := cpuMetricsFromSoc(m, coreUsages, averageCPUUsage(coreUsages), thermalStateThrottled(thermalLevel))
	gpuMetrics := gpuMetricsFromSoc(m)
	cpuMetricsChan <- cpuMetrics
	gpuMetricsChan <- gpuMetrics
	if processes, err := getProcessList(0.0); err == nil {
		processMetricsChan <- processes
	}
	netdiskMetrics := getNetDiskMetrics()
	netdiskMetricsChan <- netdiskMetrics
	publishPrometheusNetDiskMetrics(netdiskMetrics)
	tbNetStats := GetThunderboltNetStats()
	rdmaStatus := CheckRDMAAvailable()
	publishPrometheusMetrics(prometheusMetricsSnapshot{
		SystemInfo:   getSOCInfo(),
		CPUMetrics:   cpuMetrics,
		GPUMetrics:   gpuMetrics,
		Memory:       getMemoryMetrics(),
		TBNetStats:   tbNetStats,
		RDMAStatus:   rdmaStatus,
		ThermalLevel: thermalLevel,
	})
}

func Run() {
	// Pre-resolve language from CLI args / env so that early-exit legacy flags
	// (--version, --help, --dump-ioreport, --test) honor --lang. This is a
	// best-effort scan since flag.Parse() hasn't run yet; the full priority
	// chain (CLI > env > config > system) is re-applied after loadConfig().
	earlyLang := earlyResolveLanguage()
	i18n.Init(earlyLang)

	colorName, interval, setColor, setInterval := handleLegacyFlags()

	// Fail fast on Intel Macs. handleLegacyFlags already handled --version /
	// --help (which os.Exit), so by this point the user actually intends to
	// run the TUI / a diagnostic dump — both of which depend on Apple Silicon
	// IOReport channels and would otherwise hang at the loading screen.
	requireAppleSilicon()

	logfile, err := setupLogfile()
	if err != nil {
		stderrLogger.Fatalf("failed to setup log file: %v", err)
	}
	defer logfile.Close()

	parseCommandLineFlags()

	loadConfig()

	// Load saved sort column from config (only if explicitly set)
	if currentConfig.SortColumn != nil && *currentConfig.SortColumn >= 0 && *currentConfig.SortColumn < len(columns) {
		selectedColumn = *currentConfig.SortColumn
	}
	sortReverse = currentConfig.SortReverse

	flag.Parse()

	// Initialize i18n engine with override priorities
	resolvedLanguage = currentConfig.Language
	if cliLanguage != "" {
		resolvedLanguage = cliLanguage // CLI overrides config.json
	} else if envLang := os.Getenv("MACTOP_LANG"); envLang != "" {
		resolvedLanguage = envLang
	}
	currentConfig.Language = resolvedLanguage
	i18n.Init(resolvedLanguage)

	// If cli.go didn't catch --foreground (e.g., because it used an '=' sign like --foreground=green)
	// then flag.Parse() will have populated cliFgColor. Update colorName and setColor.
	if !setColor && cliFgColor != "" {
		if !IsHexColor(cliFgColor) {
			cliFgColor = strings.ToLower(cliFgColor)
		}
		colorName = cliFgColor
		setColor = true
	}

	currentUser = os.Getenv("USER")

	if runAlternateMode() {
		return
	}

	IsLightMode = detectLightMode()

	if err := ui.Init(); err != nil {
		stderrLogger.Fatalf("failed to initialize gotui: %v", err)
	}
	defer ui.Close()

	// Show branded loading screen immediately — gives instant visual feedback
	// while metrics subsystems initialize (especially DRAM BW calibration on M5+).
	renderLoadingScreen()

	if err := initSocMetrics(); err != nil {
		stderrLogger.Fatalf("failed to initialize metrics: %v", err)
	}
	defer cleanupSocMetrics()
	defer cleanupFanControl()

	StderrToLogfile(logfile)

	if prometheusPort != "" {
		startPrometheusServer(prometheusPort)
		stderrLogger.Printf("Prometheus metrics available at http://localhost:%s/metrics\n", prometheusPort)
	}
	setupUI()
	initializeTheme(colorName, setColor, interval, setInterval)
	setupGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	setupMainBlockLayout(termWidth, termHeight)

	// Seed metrics and consume them to populate all widgets BEFORE the first render.
	// This ensures users see a fully-populated TUI instead of blank/zero gauges.
	seedInitialMetrics()
	drainSeededMetrics()
	updateInfoUI()

	// Transition from loading screen to full TUI
	ui.Clear()
	renderUI()

	triggerProcessCollectionChan := make(chan struct{}, 1)

	startBackgroundWorkers()

	// Ensure clean shutdown (worker processes killed, fans restored to auto) on
	// SIGINT (Ctrl-C), SIGTERM (kill), and SIGHUP (terminal window closed).
	// SIGHUP matters for --fan-control: without catching it, closing the
	// terminal would terminate the process by default action and leave the fans
	// pinned in manual mode.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		shutdownAndExit(false)
	}()

	go collectMetrics(done, cpuMetricsChan, gpuMetricsChan, tbNetStatsChan, triggerProcessCollectionChan)
	go collectProcessMetrics(done, processMetricsChan, triggerProcessCollectionChan)
	go collectNetDiskMetrics(done, netdiskMetricsChan)

	uiEvents := ui.PollEvents()
	ticker = time.NewTicker(time.Duration(updateInterval) * time.Millisecond)

	startBackgroundUpdates(done)
	renderUI()

	defer func() {
		if partyTicker != nil {
			partyTicker.Stop()
		}
	}()
	lastUpdateTime = time.Now()

	runEventLoop(done, uiEvents)
}

// runEventLoop dispatches the event loop.
// When --menubar is active, the menu bar is already initialized (in Run())
// and metrics are pushed to it from collectMetrics via pushMenuBarMetricsFromTUI.
// We do NOT pump AppKit events here — dispatch_async in updateMenuBarMetrics
// is sufficient for the menu bar title to update.
func runEventLoop(done chan struct{}, uiEvents <-chan ui.Event) {
	handleEvents(done, uiEvents)
}

func setupLogfile() (*os.File, error) {
	logPath := mactopStatePath("mactop.log")
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to make the log directory: %v", err)
	}
	logfile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0660)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(logfile)
	return logfile, nil
}

func updateTotalPowerChart(watts float64) {
	if watts > maxPowerSeen {
		maxPowerSeen = watts * 1.1
	}
	scaledValue := int((watts / maxPowerSeen) * 8)
	if watts > 0 && scaledValue == 0 {
		scaledValue = 1
	}
	for i := 0; i < len(powerValues)-1; i++ {
		powerValues[i] = powerValues[i+1]
		powerUsageHistory[i] = powerUsageHistory[i+1]
	}
	powerValues[len(powerValues)-1] = float64(scaledValue)
	powerUsageHistory[len(powerUsageHistory)-1] = watts

	var sum float64
	count := 0
	for _, v := range powerUsageHistory {
		if v > 0 {
			sum += v
			count++
		}
	}
	avgWatts := 0.0
	if count > 0 {
		avgWatts = sum / float64(count)
	}
	sparkline.Data = powerValues
	sparkline.MaxVal = 8
	sparklineGroup.Title = fmt.Sprintf(i18n.T("Metrics_PowerSparklineGroup"), watts, maxPowerSeen)
	thermalStr, _ := getThermalStateString()
	sparkline.Title = fmt.Sprintf(i18n.T("Metrics_PowerSparklineTitle"), avgWatts, thermalStr)

	// Update power history StepChart - use terminal width for reliable slicing
	if powerHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := (termWidth / 2) - 4 // Half width, account for borders
		if visibleWidth <= 0 || visibleWidth > len(powerUsageHistory) {
			visibleWidth = len(powerUsageHistory)
		}
		visibleData := powerUsageHistory[len(powerUsageHistory)-visibleWidth:]
		powerHistoryChart.Data = [][]float64{visibleData}
		powerHistoryChart.MaxVal = maxPowerSeen * 1.1
		powerHistoryChart.DataLabels = []string{fmt.Sprintf("%.1fW", watts)}
		powerHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_PowerHistoryDetail"), avgWatts, maxPowerSeen)
	}
}

func updateCPUUI(cpuMetrics CPUMetrics) {
	if len(cpuMetrics.CoreUsages) > 0 {
		cpuCoreWidget.UpdateUsage(cpuMetrics.CoreUsages)
	}

	totalUsage := cpuMetrics.AvgUsage
	cpuGauge.Percent = int(totalUsage)

	updateCPUHistory(totalUsage)

	updateCPUGaugeTitles(totalUsage, cpuMetrics)

	thermalStr, _ := getThermalStateString()
	updatePowerChartText(cpuMetrics, thermalStr)

	memoryMetrics := getMemoryMetrics()
	updateMemoryGaugeTitle(memoryMetrics)
	memoryPercent := (float64(memoryMetrics.Used) / float64(memoryMetrics.Total)) * 100
	memoryGauge.Percent = int(memoryPercent)

	updateMemoryHistory(memoryMetrics)

	// New SoC history charts (for history_soc layout)
	// Bandwidth first: the ANE history chart derives its bandwidth-mode
	// series from aneRead/WriteBwHistory, so they must hold this tick's
	// values before renderANEHistoryChart runs.
	updateBandwidthHistory(cpuMetrics)
	updateANEHistory(cpuMetrics)
	updateSoCPowerHistory(cpuMetrics)

	if len(cpuMetrics.CoreUsages) > 0 {
		if currentConfig.Theme == "1977" {
			update1977GaugeColors()
		}
	}
}

func updateCPUHistory(totalUsage float64) {
	// Update CPU history StepChart (raw value)
	for i := 0; i < len(cpuUsageHistory)-1; i++ {
		cpuUsageHistory[i] = cpuUsageHistory[i+1]
		cpuPeakHistory[i] = cpuPeakHistory[i+1]
	}
	cpuUsageHistory[len(cpuUsageHistory)-1] = totalUsage

	// Decaying peak (slow decay when below current peak)
	peakDecay := 0.98
	if len(cpuPeakHistory) > 1 {
		prevPeak := cpuPeakHistory[len(cpuPeakHistory)-2]
		newPeak := math.Max(totalUsage, prevPeak*peakDecay)
		cpuPeakHistory[len(cpuPeakHistory)-1] = newPeak
	} else {
		cpuPeakHistory[len(cpuPeakHistory)-1] = totalUsage
	}

	if cpuHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := (termWidth / 2) - 4
		if visibleWidth <= 0 || visibleWidth > len(cpuUsageHistory) {
			visibleWidth = len(cpuUsageHistory)
		}
		if visibleWidth > 0 {
			visibleRaw := cpuUsageHistory[len(cpuUsageHistory)-visibleWidth:]
			visiblePeak := cpuPeakHistory[len(cpuPeakHistory)-visibleWidth:]

			maxVal := 0.0
			for _, v := range visibleRaw {
				if v > maxVal {
					maxVal = v
				}
			}

			scaleMax := 100.0
			if maxVal <= 25.0 {
				scaleMax = 25.0
			} else if maxVal <= 50.0 {
				scaleMax = 50.0
			}

			// In history_soc: single current value line + peak number in title only
			if currentConfig.DefaultLayout == LayoutHistorySoC {
				currentPeak := 0.0
				if len(visiblePeak) > 0 {
					currentPeak = visiblePeak[len(visiblePeak)-1]
				}
				cpuHistoryChart.Data = [][]float64{visibleRaw}
				cpuHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.CPU }, ui.ColorYellow)} // CPU color for SoC
				cpuHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_CPUHistoryPeak"), totalUsage, currentPeak)
				cpuHistoryChart.DataLabels = []string{fmt.Sprintf("%.0f%%", totalUsage)}
			} else {
				cpuHistoryChart.Data = [][]float64{visibleRaw}
				cpuHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.CPU }, ui.ColorGreen)}
				cpuHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_CPUHistoryDetail"), totalUsage)
				cpuHistoryChart.DataLabels = []string{fmt.Sprintf("%.0f%%", totalUsage)}
			}
			cpuHistoryChart.MaxVal = scaleMax
		}
	}
}

func updateMemoryHistory(memoryMetrics MemoryMetrics) {
	// Update memory used history for StepChart - use terminal width for reliable slicing
	usedGB := float64(memoryMetrics.Used) / 1024 / 1024 / 1024
	swapGB := float64(memoryMetrics.SwapUsed) / 1024 / 1024 / 1024
	totalGB := float64(memoryMetrics.Total) / 1024 / 1024 / 1024

	for i := 0; i < len(memoryUsedHistory)-1; i++ {
		memoryUsedHistory[i] = memoryUsedHistory[i+1]
		swapUsedHistory[i] = swapUsedHistory[i+1]
	}
	memoryUsedHistory[len(memoryUsedHistory)-1] = usedGB
	swapUsedHistory[len(swapUsedHistory)-1] = swapGB

	if memoryHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := (termWidth / 2) - 4 // Half width, account for borders
		if currentConfig.DefaultLayout == LayoutHistorySoC {
			visibleWidth = (termWidth / 3) - 4
		}
		if visibleWidth <= 0 || visibleWidth > len(memoryUsedHistory) {
			visibleWidth = len(memoryUsedHistory)
		}

		visibleMem := memoryUsedHistory[len(memoryUsedHistory)-visibleWidth:]
		visibleSwap := swapUsedHistory[len(swapUsedHistory)-visibleWidth:]

		memoryHistoryChart.Data = [][]float64{visibleMem, visibleSwap}
		memoryHistoryChart.MaxVal = totalGB // Scale to total physical RAM
		memoryHistoryChart.DataLabels = []string{
			fmt.Sprintf("%.1fGB", usedGB),
			fmt.Sprintf("%.1fGB", swapGB),
		}
		memoryHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_MemoryHistoryDetail"), usedGB, totalGB, swapGB)
	}

	updateMemBandwidthHistory()
}

// anePoweredLabel renders the binary ANE power-domain signal (M5 Max / macOS 27
// non-root, where no PMP utilization channel exists) as a powered/idle word
// instead of a misleading percentage. The value is the power-domain duty cycle
// over the sample window; >0 means the ANE was powered for at least part of it.
func anePoweredLabel(dutyPct float64) string {
	if dutyPct > 0 {
		return "powered"
	}
	return "idle"
}

// aneOnOffLabel renders the exclave ANE (M5 / M5 Max) power-domain state as a
// terse ON/idle word for the main gauge, where CurrentPowerState is binary and a
// percentage would be meaningless (pinned high by background macOS ML services).
func aneOnOffLabel(dutyPct float64) string {
	if dutyPct > 0 {
		return "ON"
	}
	return "idle"
}

func aneClusterIsActive(pct float64) bool {
	return pct > 0
}

func formatDualANEClusterStatus(c0, c1 float64, powered bool) string {
	active0 := aneClusterIsActive(c0)
	active1 := aneClusterIsActive(c1)
	if powered {
		switch {
		case active0 && active1:
			return "ANE0 & ANE1 powered"
		case active0 && !active1:
			return "ANE0 powered, ANE1 idle"
		case !active0 && active1:
			return "ANE0 idle, ANE1 powered"
		default:
			return "idle"
		}
	}
	switch {
	case active0 && active1:
		return "ANE0 & ANE1 active"
	case active0 && !active1:
		return "ANE0 active, ANE1 idle"
	case !active0 && active1:
		return "ANE0 idle, ANE1 active"
	default:
		return "idle"
	}
}

// formatDualANEClusterChartText builds a consistent title + per-line labels for
// the history_soc dual-cluster ANE chart. Title summarizes the combined state;
// line labels match each cluster's individual powered/idle (or %) reading.
func formatDualANEClusterChartText(c0, c1 float64, powered bool, nClusters int) (title string, label0, label1 string) {
	if powered {
		label0 = fmt.Sprintf("ANE0 %s", anePoweredLabel(c0))
		label1 = fmt.Sprintf("ANE1 %s", anePoweredLabel(c1))
	} else {
		label0 = fmt.Sprintf("ANE0 %.0f%%", c0)
		label1 = fmt.Sprintf("ANE1 %.0f%%", c1)
	}
	title = fmt.Sprintf("ANE (%d clusters) · %s", nClusters, formatDualANEClusterStatus(c0, c1, powered))
	return title, label0, label1
}

func clampANEPercent(pct float64) float64 {
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// staggerANEClusterChartSeries applies a small display-only vertical offset when
// two ANE cluster traces are identical (common when both read "powered" at 100%).
// Real metrics/titles are unchanged; only the StepChart draw positions shift.
func staggerANEClusterChartSeries(c0, c1 []float64, scaleMax float64) (displayC0, displayC1 []float64) {
	const eps = 1.0
	const halfStagger = 4.0

	displayC0 = make([]float64, len(c0))
	displayC1 = make([]float64, len(c1))
	for i := range c0 {
		displayC0[i] = c0[i]
		displayC1[i] = c1[i]
		if math.Abs(c0[i]-c1[i]) >= eps {
			continue
		}
		displayC0[i] = math.Max(0, c0[i]-halfStagger)
		displayC1[i] = math.Min(scaleMax, c1[i]+halfStagger)
	}
	return displayC0, displayC1
}

func updateANEHistory(cpuMetrics CPUMetrics) {
	// Same utilization source as the ANE gauge: PMP residency (macOS 27/M5)
	// -> Energy Model power estimate (macOS 26) -> bandwidth activity
	// estimate (M1-M4 on macOS 27). Keeps the history chart consistent with
	// the gauge instead of reading 0 where only bandwidth is available.
	anePct := aneUtilizationPercent(cpuMetrics)
	aneWatts := cpuMetrics.ANEW

	clusterCount := len(cpuMetrics.ANEClusterActive)
	cluster0 := 0.0
	cluster1 := 0.0
	if clusterCount > 0 {
		cluster0 = clampANEPercent(cpuMetrics.ANEClusterActive[0])
	}
	if clusterCount > 1 {
		cluster1 = clampANEPercent(cpuMetrics.ANEClusterActive[1])
	}

	for i := 0; i < len(aneUsageHistory)-1; i++ {
		aneUsageHistory[i] = aneUsageHistory[i+1]
		anePeakHistory[i] = anePeakHistory[i+1]
		if len(aneCluster0History) > 0 {
			aneCluster0History[i] = aneCluster0History[i+1]
			aneCluster1History[i] = aneCluster1History[i+1]
		}
	}
	aneUsageHistory[len(aneUsageHistory)-1] = anePct
	if len(aneCluster0History) > 0 {
		aneCluster0History[len(aneCluster0History)-1] = cluster0
		aneCluster1History[len(aneCluster1History)-1] = cluster1
	}

	// Decaying peak for ANE
	peakDecay := 0.98
	if len(anePeakHistory) > 1 {
		prevPeak := anePeakHistory[len(anePeakHistory)-2]
		anePeakHistory[len(anePeakHistory)-1] = math.Max(anePct, prevPeak*peakDecay)
	} else {
		anePeakHistory[len(anePeakHistory)-1] = anePct
	}

	renderANEHistoryChart(cpuMetrics, anePct, aneWatts, cpuMetrics.ANEBW, aneBWLabelMode(cpuMetrics), cluster0, cluster1)
}

// aneVisibleSeries returns the plotted ANE utilization window. In bandwidth
// mode the percentages are derived at render time from the stored physical
// GB/s histories against the *current* adaptive reference: stored percentages
// were computed against whatever reference existed when each was pushed, so
// after the reference ratchets (e.g. during a load ramp) they stop being
// comparable — a 7x bandwidth ramp would paint as a flat 100% plateau.
func aneVisibleSeries(visibleWidth int, bwMode bool) []float64 {
	// Re-derive from physical bandwidth only for the tier-3 adaptive-bandwidth
	// estimate (M1-M4 on macOS 27), whose stored percentages go stale as the
	// reference ratchets. Residency (tier 1, M5) and power (tier 2, macOS 26)
	// percentages are against a fixed scale and stay comparable across ticks,
	// so plot them as stored — re-deriving residency from bandwidth would
	// diverge from the gauge, which reads the residency tier.
	if !bwMode || aneResidencyLatched.Load() {
		return aneUsageHistory[len(aneUsageHistory)-visibleWidth:]
	}
	ref := max(math.Float64frombits(maxANEBWSeenBits.Load()), aneBWRefFloorGBs)
	rd := aneReadBwHistory[len(aneReadBwHistory)-visibleWidth:]
	wr := aneWriteBwHistory[len(aneWriteBwHistory)-visibleWidth:]
	out := make([]float64, visibleWidth)
	for i := range out {
		pct := (rd[i] + wr[i]) / ref * 100
		if pct > 100 {
			pct = 100
		}
		out[i] = pct
	}
	return out
}

// historyLineColor returns the active custom-theme color for a history chart
// component, or the fallback default when no custom theme is set. Per-tick
// LineColors assignments must route through this so they don't clobber the
// colors applyCustomWidgetColors applied (same pattern as updateSoCPowerHistory).
func historyLineColor(pick func(*CustomThemeConfig) string, fallback ui.Color) ui.Color {
	if currentConfig.CustomTheme == nil {
		return fallback
	}
	fg := GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode)
	return resolveCustomColor(pick(currentConfig.CustomTheme), fg)
}

// seriesMax returns the largest value in the series (0 for an empty one).
func seriesMax(series []float64) float64 {
	peak := 0.0
	for _, v := range series {
		if v > peak {
			peak = v
		}
	}
	return peak
}

func renderANEHistoryChart(cpuMetrics CPUMetrics, anePct, aneWatts, aneBW float64, bwMode bool, cluster0, cluster1 float64) {
	if aneHistoryChart == nil {
		return
	}
	termWidth, _ := GetCachedTerminalDimensions()
	visibleWidth := (termWidth / 2) - 4
	if visibleWidth <= 0 || visibleWidth > len(aneUsageHistory) {
		visibleWidth = len(aneUsageHistory)
	}
	if visibleWidth <= 0 {
		return
	}
	visibleRaw := aneVisibleSeries(visibleWidth, bwMode)
	visiblePeak := anePeakHistory[len(anePeakHistory)-visibleWidth:]

	maxVal := 0.0
	for _, v := range visibleRaw {
		if v > maxVal {
			maxVal = v
		}
	}
	scaleMax := 100.0
	if maxVal <= 25.0 {
		scaleMax = 25.0
	} else if maxVal <= 50.0 {
		scaleMax = 50.0
	}

	clusterCount := len(cpuMetrics.ANEClusterActive)
	if currentConfig.DefaultLayout == LayoutHistorySoC && clusterCount > 1 && len(aneCluster0History) > 0 {
		visibleC0 := aneCluster0History[len(aneCluster0History)-visibleWidth:]
		visibleC1 := aneCluster1History[len(aneCluster1History)-visibleWidth:]
		for _, series := range [][]float64{visibleC0, visibleC1} {
			for _, v := range series {
				if v > maxVal {
					maxVal = v
				}
			}
		}
		if maxVal <= 25.0 {
			scaleMax = 25.0
		} else if maxVal <= 50.0 {
			scaleMax = 50.0
		} else {
			scaleMax = 100.0
		}
		displayC0, displayC1 := staggerANEClusterChartSeries(visibleC0, visibleC1, scaleMax)
		aneColor := historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, ui.ColorRed)
		aneHistoryChart.Data = [][]float64{displayC0, displayC1}
		aneHistoryChart.LineColors = []ui.Color{aneColor, aneColor}
		nClusters := cpuMetrics.ANEClusterCount
		if nClusters < 2 {
			nClusters = clusterCount
		}
		title, label0, label1 := formatDualANEClusterChartText(cluster0, cluster1, cpuMetrics.ANEPowered, nClusters)
		aneHistoryChart.Title = title
		aneHistoryChart.DataLabels = []string{label0, label1}
		aneHistoryChart.MaxVal = scaleMax
		return
	}

	aneHistoryChart.Data = [][]float64{visibleRaw}
	if cpuMetrics.ANEExclave {
		// Exclave ANE (M5 / M5 Max): binary powered/idle, never a %. The history
		// trace is the 0/100 power-domain series; the title and data label carry
		// ON/idle so layout 19 matches the gauge instead of showing "100.0%".
		state := aneOnOffLabel(anePct)
		aneColor := ui.ColorRed
		if currentConfig.DefaultLayout != LayoutHistorySoC {
			aneColor = ui.ColorMagenta
		}
		aneHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, aneColor)}
		aneHistoryChart.DataLabels = []string{state}
		aneHistoryChart.Title = fmt.Sprintf("ANE: %s", state)
		aneHistoryChart.MaxVal = scaleMax
		return
	}
	aneHistoryChart.DataLabels = []string{fmt.Sprintf("%.1f%%", anePct)}
	if currentConfig.DefaultLayout == LayoutHistorySoC {
		// Peak: in bandwidth mode use the max of the visible window — ANE
		// load is typically flat at saturation, so the decaying tracker
		// collapses to the current value within two ticks and the label
		// degenerates to "Peak == current". In watts/residency modes keep
		// the decaying tracker, consistent with the CPU/GPU charts.
		currentPeak := 0.0
		if bwMode {
			currentPeak = seriesMax(visibleRaw)
		} else if len(visiblePeak) > 0 {
			currentPeak = visiblePeak[len(visiblePeak)-1]
		}
		aneHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, ui.ColorRed)} // ANE red in SoC
		if bwMode {
			// macOS 27+: the ANE energy counter is dead, so a wattage reading
			// would always be a meaningless 0.00W — show bandwidth instead.
			aneHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_ANEHistoryPeakBW"), anePct, currentPeak, aneBW)
		} else {
			aneHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_ANEHistoryPeak"), anePct, currentPeak, aneWatts)
		}
	} else {
		aneHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, ui.ColorMagenta)}
		if bwMode {
			aneHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_ANEHistoryDetailBW"), anePct, aneBW)
		} else {
			aneHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_ANEHistoryDetail"), anePct, aneWatts)
		}
	}
	aneHistoryChart.MaxVal = scaleMax
}

func updateBandwidthHistory(cpuMetrics CPUMetrics) {
	readGBs := cpuMetrics.DRAMReadBW
	writeGBs := cpuMetrics.DRAMWriteBW
	aneReadGBs := cpuMetrics.ANEReadBW
	aneWriteGBs := cpuMetrics.ANEWriteBW

	for i := 0; i < len(dramReadHistory)-1; i++ {
		dramReadHistory[i] = dramReadHistory[i+1]
		dramWriteHistory[i] = dramWriteHistory[i+1]
		aneReadBwHistory[i] = aneReadBwHistory[i+1]
		aneWriteBwHistory[i] = aneWriteBwHistory[i+1]
		bwPeakHistory[i] = bwPeakHistory[i+1]
	}
	dramReadHistory[len(dramReadHistory)-1] = readGBs
	dramWriteHistory[len(dramWriteHistory)-1] = writeGBs
	aneReadBwHistory[len(aneReadBwHistory)-1] = aneReadGBs
	aneWriteBwHistory[len(aneWriteBwHistory)-1] = aneWriteGBs

	combined := readGBs + writeGBs

	// Decaying peak across the plotted series: DRAM total and the ANE fabric
	// pair. ANE traffic is normally a subset of DRAM total, but stalled DCS
	// counters (macOS 27 beta) can report DRAM 0 while ANE histograms still
	// flow — the peak label must bound whatever is actually drawn.
	peakInput := math.Max(combined, math.Max(aneReadGBs, aneWriteGBs))
	peakDecay := 0.98
	if len(bwPeakHistory) > 1 {
		prevPeak := bwPeakHistory[len(bwPeakHistory)-2]
		bwPeakHistory[len(bwPeakHistory)-1] = math.Max(peakInput, prevPeak*peakDecay)
	} else {
		bwPeakHistory[len(bwPeakHistory)-1] = peakInput
	}

	renderBandwidthHistoryChart(readGBs, writeGBs, aneReadGBs, aneWriteGBs)
}

// bandwidthScaleMax returns the adaptive Y-axis maximum for the bandwidth
// history chart: 1.2x the largest visible sample across all series, floored
// at 1 GB/s.
func bandwidthScaleMax(series ...[]float64) float64 {
	maxVal := 0.0
	for _, vals := range series {
		for _, v := range vals {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal < 1.0 {
		maxVal = 1.0
	}
	return maxVal * 1.2
}

func renderBandwidthHistoryChart(readGBs, writeGBs, aneReadGBs, aneWriteGBs float64) {
	if bandwidthHistoryChart == nil {
		return
	}
	{
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := (termWidth / 2) - 4
		if currentConfig.DefaultLayout == LayoutHistorySoC {
			// One-third-width column in the bottom row — match the adjacent
			// memory and SSD charts' time window.
			visibleWidth = (termWidth / 3) - 4
		}
		if visibleWidth <= 0 || visibleWidth > len(dramReadHistory) {
			visibleWidth = len(dramReadHistory)
		}

		visibleRead := dramReadHistory[len(dramReadHistory)-visibleWidth:]
		visibleWrite := dramWriteHistory[len(dramWriteHistory)-visibleWidth:]
		visibleAneRead := aneReadBwHistory[len(aneReadBwHistory)-visibleWidth:]
		visibleAneWrite := aneWriteBwHistory[len(aneWriteBwHistory)-visibleWidth:]
		visiblePeak := bwPeakHistory[len(bwPeakHistory)-visibleWidth:]

		// Scale to the drawn series only: bwPeakHistory decays slowly and is
		// not rendered, so including it would pin the Y-axis high long after a
		// spike and flatten the live lines.
		scaleSeries := [][]float64{visibleRead, visibleWrite, visibleAneRead, visibleAneWrite}

		// history_soc also draws a combined Read+Write total line — it must
		// participate in scaling or it clips against the chart top whenever
		// read and write are both high in the same sample.
		var visibleTotal []float64
		if currentConfig.DefaultLayout == LayoutHistorySoC {
			visibleTotal = make([]float64, len(visibleRead))
			for i := range visibleRead {
				visibleTotal[i] = visibleRead[i] + visibleWrite[i]
			}
			scaleSeries = append(scaleSeries, visibleTotal)
		}
		scaleMax := bandwidthScaleMax(scaleSeries...)

		// In history_soc layout, force a minimum visible scale so the graph
		// doesn't look completely dead when bandwidth is low (common even with high GPU/ANE load)
		if currentConfig.DefaultLayout == LayoutHistorySoC && scaleMax < 8.0 {
			scaleMax = 8.0
		}

		if currentConfig.DefaultLayout == LayoutHistorySoC {
			currentPeak := 0.0
			if len(visiblePeak) > 0 {
				currentPeak = visiblePeak[len(visiblePeak)-1]
			}

			// To make Write (red) visible on top:
			// Total (bottom, violet), Read (blue), Write (red), then the ANE
			// fabric BW pair (green/yellow) as the top layers.
			bandwidthHistoryChart.Data = [][]float64{visibleTotal, visibleRead, visibleWrite, visibleAneRead, visibleAneWrite}
			bandwidthHistoryChart.LineColors = []ui.Color{ui.ColorMagenta, ui.ColorBlue, ui.ColorRed, ui.ColorGreen, ui.ColorYellow}
			total := readGBs + writeGBs
			bandwidthHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_BandwidthHistoryPeak"), readGBs, writeGBs, aneReadGBs, aneWriteGBs, currentPeak)
			bandwidthHistoryChart.DataLabels = []string{
				fmt.Sprintf("Tot:%.1f", total),
				fmt.Sprintf("R:%.1f", readGBs),
				fmt.Sprintf("W:%.1f", writeGBs),
				fmt.Sprintf("AR:%.1f", aneReadGBs),
				fmt.Sprintf("AW:%.1f", aneWriteGBs),
			}
		} else {
			bandwidthHistoryChart.Data = [][]float64{visibleRead, visibleWrite, visibleAneRead, visibleAneWrite}
			bandwidthHistoryChart.LineColors = []ui.Color{ui.ColorCyan, ui.ColorYellow, ui.ColorGreen, ui.ColorMagenta}
			total := readGBs + writeGBs
			bandwidthHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_BandwidthHistoryDetail"), readGBs, writeGBs, total) +
				fmt.Sprintf(" ANE R:%.1f W:%.1f", aneReadGBs, aneWriteGBs)
			bandwidthHistoryChart.DataLabels = []string{
				fmt.Sprintf("R:%.1f", readGBs),
				fmt.Sprintf("W:%.1f", writeGBs),
				fmt.Sprintf("AR:%.1f", aneReadGBs),
				fmt.Sprintf("AW:%.1f", aneWriteGBs),
			}
		}
		bandwidthHistoryChart.MaxVal = scaleMax
	}
}

// updateSoCPowerHistory maintains rolling histories for individual power rails
// (CPU, GPU, ANE, DRAM) and feeds the multi-line socPowerHistoryChart.
func updateSoCPowerHistory(cpuMetrics CPUMetrics) {
	for i := 0; i < len(cpuPowerHistory)-1; i++ {
		cpuPowerHistory[i] = cpuPowerHistory[i+1]
		gpuPowerHistory[i] = gpuPowerHistory[i+1]
		anePowerHistory[i] = anePowerHistory[i+1]
		dramPowerHistory[i] = dramPowerHistory[i+1]
	}
	cpuPowerHistory[len(cpuPowerHistory)-1] = cpuMetrics.CPUW
	gpuPowerHistory[len(gpuPowerHistory)-1] = cpuMetrics.GPUW + cpuMetrics.GPUSRAMW
	anePowerHistory[len(anePowerHistory)-1] = cpuMetrics.ANEW
	dramPowerHistory[len(dramPowerHistory)-1] = cpuMetrics.DRAMW

	if socPowerHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := termWidth - 4
		if currentConfig.DefaultLayout == LayoutHistorySoC {
			// Half-width column (row 2, beside the ANE chart) — match the
			// neighboring CPU/GPU/ANE charts' time window.
			visibleWidth = (termWidth / 2) - 4
		}
		if visibleWidth <= 0 || visibleWidth > len(cpuPowerHistory) {
			visibleWidth = len(cpuPowerHistory)
		}

		visCPU := cpuPowerHistory[len(cpuPowerHistory)-visibleWidth:]
		visGPU := gpuPowerHistory[len(gpuPowerHistory)-visibleWidth:]
		visANE := anePowerHistory[len(anePowerHistory)-visibleWidth:]
		visDRAM := dramPowerHistory[len(dramPowerHistory)-visibleWidth:]

		// Find max across all for scaling
		maxVal := 0.0
		for i := range visCPU {
			if visCPU[i] > maxVal {
				maxVal = visCPU[i]
			}
			if visGPU[i] > maxVal {
				maxVal = visGPU[i]
			}
			if visANE[i] > maxVal {
				maxVal = visANE[i]
			}
			if visDRAM[i] > maxVal {
				maxVal = visDRAM[i]
			}
		}
		if maxVal < 0.5 {
			maxVal = 0.5
		}

		// ANE last so its red line draws on top of overlapping series
		// (at idle all rails sit near 0 and later series overpaint earlier ones).
		socPowerHistoryChart.Data = [][]float64{visCPU, visGPU, visDRAM, visANE}
		socPowerHistoryChart.MaxVal = maxVal * 1.15
		// ANE is omitted from the labels and title entirely when its energy
		// counter is provably dead (macOS 27+) — there is no reading to show.
		// The (flat) series itself stays plotted so the chart structure is
		// stable, and the label/segment return automatically if a future OS
		// build revives the counter (aneBWLabelMode flips off when watts flow).
		aneDead := aneBWLabelMode(cpuMetrics)
		labels := []string{
			fmt.Sprintf("CPU:%.1f", cpuMetrics.CPUW),
			fmt.Sprintf("GPU:%.1f", cpuMetrics.GPUW+cpuMetrics.GPUSRAMW),
			fmt.Sprintf("DRAM:%.1f", cpuMetrics.DRAMW),
		}
		if !aneDead {
			// ANE is the last series, so omitting its label leaves the
			// CPU/GPU/DRAM labels correctly aligned with their series.
			labels = append(labels, fmt.Sprintf("ANE:%.1f", cpuMetrics.ANEW))
		}
		socPowerHistoryChart.DataLabels = labels
		// Series order: CPU, GPU, DRAM, ANE (ANE last so its red line draws on
		// top). Resolve per-component custom theme colors when set instead of
		// clobbering them with hard-coded defaults every tick.
		cpuC, gpuC, memC := ui.ColorYellow, ui.ColorGreen, ui.ColorCyan
		if currentConfig.CustomTheme != nil {
			fg := GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode)
			cpuC = resolveCustomColor(currentConfig.CustomTheme.CPU, fg)
			gpuC = resolveCustomColor(currentConfig.CustomTheme.GPU, fg)
			memC = resolveCustomColor(currentConfig.CustomTheme.Memory, fg)
		}
		socPowerHistoryChart.LineColors = []ui.Color{cpuC, gpuC, memC, ui.ColorRed}

		totalPower := cpuMetrics.CPUW + cpuMetrics.GPUW + cpuMetrics.GPUSRAMW + cpuMetrics.ANEW + cpuMetrics.DRAMW
		if aneDead {
			socPowerHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_SoCPowerDetailNoANE"),
				totalPower, cpuMetrics.CPUW, cpuMetrics.GPUW+cpuMetrics.GPUSRAMW, cpuMetrics.DRAMW)
		} else {
			socPowerHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_SoCPowerDetail"),
				totalPower, fmt.Sprintf("%.1fW", cpuMetrics.ANEW), cpuMetrics.CPUW, cpuMetrics.GPUW+cpuMetrics.GPUSRAMW, cpuMetrics.DRAMW)
		}
	}
}

func updateMemBandwidthHistory() {
	readBW := lastCPUMetrics.DRAMReadBW
	writeBW := lastCPUMetrics.DRAMWriteBW
	combinedBW := lastCPUMetrics.DRAMBWCombined
	for i := 0; i < len(memBWReadHistory)-1; i++ {
		memBWReadHistory[i] = memBWReadHistory[i+1]
		memBWWriteHistory[i] = memBWWriteHistory[i+1]
	}
	memBWReadHistory[len(memBWReadHistory)-1] = readBW
	memBWWriteHistory[len(memBWWriteHistory)-1] = writeBW

	if combinedBW > maxMemBWSeen {
		maxMemBWSeen = combinedBW
	}

	if memBWHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := termWidth - 4
		if visibleWidth <= 0 || visibleWidth > len(memBWReadHistory) {
			visibleWidth = len(memBWReadHistory)
		}
		visibleRead := memBWReadHistory[len(memBWReadHistory)-visibleWidth:]
		visibleWrite := memBWWriteHistory[len(memBWWriteHistory)-visibleWidth:]

		memBWHistoryChart.Data = [][]float64{visibleRead, visibleWrite}
		scaleMax := maxMemBWSeen
		if scaleMax < 10 {
			scaleMax = 10
		}
		memBWHistoryChart.MaxVal = scaleMax
		memBWHistoryChart.DataLabels = []string{
			fmt.Sprintf("R %.1f GB/s", readBW),
			fmt.Sprintf("W %.1f GB/s", writeBW),
		}
		memBWHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_MemBWHistoryDetail"), combinedBW)
	}
}

var lastEFreq, lastPFreq, lastSFreq int

func formatCPUFreq(cpuMetrics CPUMetrics) string {
	// Retain last known non-zero frequency so idle samples don't cause flicker
	if cpuMetrics.EClusterFreqMHz > 0 {
		lastEFreq = cpuMetrics.EClusterFreqMHz
	}
	if cpuMetrics.PClusterFreqMHz > 0 {
		lastPFreq = cpuMetrics.PClusterFreqMHz
	}
	if cpuMetrics.SClusterFreqMHz > 0 {
		lastSFreq = cpuMetrics.SClusterFreqMHz
	}
	if lastEFreq <= 0 && lastPFreq <= 0 && lastSFreq <= 0 {
		return ""
	}
	parts := make([]string, 0, 3)
	if lastEFreq > 0 {
		parts = append(parts, fmt.Sprintf("E%.1f", float64(lastEFreq)/1000.0))
	}
	if lastPFreq > 0 {
		parts = append(parts, fmt.Sprintf("P%.1f", float64(lastPFreq)/1000.0))
	}
	if lastSFreq > 0 {
		parts = append(parts, fmt.Sprintf("S%.1f", float64(lastSFreq)/1000.0))
	}
	return " @ " + strings.Join(parts, "/") + " GHz"
}

func updateCPUGaugeTitles(totalUsage float64, cpuMetrics CPUMetrics) {
	coreSummary := FormatCoreSummary(cpuCoreWidget.eCoreCount, cpuCoreWidget.pCoreCount, cpuCoreWidget.sCoreCount)
	totalCPUCores := cpuCoreWidget.eCoreCount + cpuCoreWidget.pCoreCount + cpuCoreWidget.sCoreCount
	cpuFreqStr := formatCPUFreq(cpuMetrics)
	if isCompactLayout() {
		cpuGauge.Title = fmt.Sprintf(i18n.T("Metrics_CPUGaugeCompact"), totalUsage, formatTemp(cpuMetrics.CPUTemp))
	} else {
		cpuGauge.Title = fmt.Sprintf(i18n.T("Metrics_CPUGauge"),
			totalCPUCores,
			coreSummary,
			totalUsage,
			cpuFreqStr,
			formatTemp(cpuMetrics.CPUTemp),
		)
	}
	cpuCoreWidget.Title = fmt.Sprintf(i18n.T("Metrics_CPUGauge"),
		totalCPUCores,
		coreSummary,
		totalUsage,
		cpuFreqStr,
		formatTemp(cpuMetrics.CPUTemp),
	)
	// Utilization priority (see aneUtilizationPercent): PMP residency-based %
	// (macOS 27/M5) -> Energy Model power estimate (macOS 26) -> AMC/PMP
	// bandwidth activity estimate (M1-M4 on macOS 27).
	aneUtil := aneUtilizationPercent(cpuMetrics)
	// Bandwidth-form label whenever the ANE power channel yields nothing —
	// session-latched (see aneBWLabelMode) so the label doesn't flip back to
	// "@ 0.00 W" when the ANE goes idle on an OS whose watts are never nonzero.
	bwMode := aneBWLabelMode(cpuMetrics)
	// The Gauge prints its inner bar label as "NN%" unless Label is set; default
	// back to that for the %-form layouts, override it for the binary exclave case.
	aneGauge.Label = ""
	if cpuMetrics.ANEExclave {
		// Exclave ANE (M5 / M5 Max): binary ON/idle only. aneUtil is 0 or 100, so
		// the gauge bar reads empty/full and neither the title nor the bar label
		// shows a percentage.
		aneGauge.Title = fmt.Sprintf("ANE: %s", aneOnOffLabel(aneUtil))
		aneGauge.Label = aneOnOffLabel(aneUtil)
	} else if isCompactLayout() {
		if bwMode {
			aneGauge.Title = fmt.Sprintf(i18n.T("Metrics_ANEGaugeBWCompact"), cpuMetrics.ANEBW)
		} else {
			aneGauge.Title = fmt.Sprintf(i18n.T("Metrics_ANEGaugeCompact"), cpuMetrics.ANEW)
		}
	} else if cpuMetrics.ANEPowered && !bwMode {
		aneGauge.Title = fmt.Sprintf("ANE %s (%.2fW)", anePoweredLabel(aneUtil), cpuMetrics.ANEW)
	} else {
		if bwMode {
			aneGauge.Title = fmt.Sprintf(i18n.T("Metrics_ANEGaugeBW"), aneUtil, cpuMetrics.ANEBW)
		} else {
			aneGauge.Title = fmt.Sprintf(i18n.T("Metrics_ANEGauge"), aneUtil, cpuMetrics.ANEW)
		}
	}
	aneGauge.Percent = int(aneUtil)
}

func updatePowerChartText(cpuMetrics CPUMetrics, thermalStr string) {
	PowerChart.Title = i18n.T("TUI_PowerUsage")

	// When the ANE energy counter is dead (macOS 27+), a wattage reading
	// would always be a meaningless 0.00 W — show fabric bandwidth instead,
	// matching the ANE gauge.
	bwMode := aneBWLabelMode(cpuMetrics)

	if isCompactLayout() {
		aneStr := fmt.Sprintf("%.1fW", cpuMetrics.ANEW)
		if bwMode {
			aneStr = fmt.Sprintf("%.1fGB/s", cpuMetrics.ANEBW)
		}
		PowerChart.Title = i18n.T("Metrics_PowerChartTitleCompact")
		PowerChart.Text = fmt.Sprintf(i18n.T("Metrics_PowerChartTextCompact"),
			cpuMetrics.CPUW,
			cpuMetrics.GPUW+cpuMetrics.GPUSRAMW,
			aneStr,
			cpuMetrics.DRAMW,
			cpuMetrics.PackageW,
			thermalStr,
		)
	} else {
		aneStr := fmt.Sprintf("%.2f W", cpuMetrics.ANEW)
		if bwMode {
			aneStr = fmt.Sprintf("%.2f GB/s", cpuMetrics.ANEBW)
		}
		uptimeSeconds, _ := GetNativeUptime()
		uptimeStr := formatTime(float64(uptimeSeconds))

		PowerChart.Text = fmt.Sprintf(i18n.T("Metrics_PowerChartText"),
			cpuMetrics.CPUW,
			cpuMetrics.GPUW+cpuMetrics.GPUSRAMW,
			aneStr,
			cpuMetrics.DRAMW,
			cpuMetrics.SystemW,
			cpuMetrics.PackageW,
			thermalStr,
			uptimeStr,
		)
	}

	if line := formatBatteryLine(); line != "" {
		PowerChart.Text += "\n" + line
	}
}

func updateMemoryGaugeTitle(memoryMetrics MemoryMetrics) {
	if isCompactLayout() {
		memoryGauge.Title = fmt.Sprintf(i18n.T("Metrics_MemGaugeCompact"), float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024, lastCPUMetrics.DRAMBWCombined)
	} else {
		memoryGauge.Title = fmt.Sprintf(i18n.T("Metrics_MemGauge"), float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024, float64(memoryMetrics.SwapUsed)/1024/1024/1024, float64(memoryMetrics.SwapTotal)/1024/1024/1024, lastCPUMetrics.DRAMBWCombined)
	}
}

func updateGPUUI(gpuMetrics GPUMetrics) {
	if isCompactLayout() {
		if gpuMetrics.Temp > 0 {
			gpuGauge.Title = fmt.Sprintf(i18n.T("Metrics_GPUGaugeCompactTemp"), int(gpuMetrics.ActivePercent), formatTemp(float64(gpuMetrics.Temp)))
		} else {
			gpuGauge.Title = fmt.Sprintf(i18n.T("Metrics_GPUGaugeCompactFreq"), int(gpuMetrics.ActivePercent), gpuMetrics.FreqMHz)
		}
	} else {
		if gpuMetrics.Temp > 0 {
			gpuGauge.Title = fmt.Sprintf(i18n.T("Metrics_GPUGaugeTemp"), int(gpuMetrics.ActivePercent), gpuMetrics.FreqMHz, formatTemp(float64(gpuMetrics.Temp)))
		} else {
			gpuGauge.Title = fmt.Sprintf(i18n.T("Metrics_GPUGaugeFreq"), int(gpuMetrics.ActivePercent), gpuMetrics.FreqMHz)
		}
	}
	gpuGauge.Percent = int(gpuMetrics.ActivePercent)

	for i := 0; i < len(gpuValues)-1; i++ {
		gpuValues[i] = gpuValues[i+1]
		gpuPeakHistory[i] = gpuPeakHistory[i+1]
		gpuEffectiveHistory[i] = gpuEffectiveHistory[i+1]
	}
	gpuValues[len(gpuValues)-1] = gpuMetrics.ActivePercent

	// Compute effective load *at this moment* using the frequency of this sample
	effectiveNow := gpuMetrics.ActivePercent
	if gpuMetrics.FreqMHz > 0 {
		maxFreq := GetGPUMaxFreqMHz()
		if maxFreq > 0 {
			eff := gpuMetrics.ActivePercent * (float64(gpuMetrics.FreqMHz) / float64(maxFreq))
			if eff > 100 {
				eff = 100
			}
			effectiveNow = eff
		}
	}
	gpuEffectiveHistory[len(gpuEffectiveHistory)-1] = effectiveNow

	// Decaying peak for GPU (based on raw load)
	peakDecay := 0.98
	if len(gpuPeakHistory) > 1 {
		prevPeak := gpuPeakHistory[len(gpuPeakHistory)-2]
		gpuPeakHistory[len(gpuPeakHistory)-1] = math.Max(gpuMetrics.ActivePercent, prevPeak*peakDecay)
	} else {
		gpuPeakHistory[len(gpuPeakHistory)-1] = gpuMetrics.ActivePercent
	}

	var sum float64
	count := 0
	for _, v := range gpuValues {
		if v > 0 {
			sum += v
			count++
		}
	}
	avgGPU := 0.0
	if count > 0 {
		avgGPU = sum / float64(count)
	}

	gpuSparkline.Data = gpuValues
	gpuSparkline.MaxVal = 100
	if isCompactLayout() {
		gpuSparklineGroup.Title = fmt.Sprintf(i18n.T("Metrics_GPUSparklineCompact"), int(gpuMetrics.ActivePercent), avgGPU)
	} else {
		gpuSparklineGroup.Title = fmt.Sprintf(i18n.T("Metrics_GPUSparkline"), int(gpuMetrics.ActivePercent), avgGPU)
	}

	renderGPUHistoryChart(gpuMetrics, avgGPU, effectiveNow)

	// Update gauge colors with dynamic saturation if 1977 theme is active
	if currentConfig.Theme == "1977" {
		update1977GaugeColors()
	}
}

func renderGPUHistoryChart(gpuMetrics GPUMetrics, avgGPU, effectiveNow float64) {
	if gpuHistoryChart == nil {
		return
	}
	termWidth, _ := GetCachedTerminalDimensions()
	visibleWidth := termWidth - 4
	if currentConfig.DefaultLayout == LayoutHistoryFull || currentConfig.DefaultLayout == LayoutHistorySoC {
		visibleWidth = (termWidth / 2) - 4
	}
	if visibleWidth <= 0 || visibleWidth > len(gpuValues) {
		visibleWidth = len(gpuValues)
	}

	visibleRaw := gpuValues[len(gpuValues)-visibleWidth:]
	visibleEffective := gpuEffectiveHistory[len(gpuEffectiveHistory)-visibleWidth:]
	visiblePeak := gpuPeakHistory[len(gpuPeakHistory)-visibleWidth:]

	if currentConfig.DefaultLayout == LayoutHistorySoC {
		currentPeak := 0.0
		if len(visiblePeak) > 0 {
			currentPeak = visiblePeak[len(visiblePeak)-1]
		}

		// Use the pre-recorded effective history (each point scaled with the freq at the time it was sampled).
		// This is the correct behavior — frequency changes only affect new data points.
		gpuHistoryChart.Data = [][]float64{visibleEffective}
		gpuHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.GPU }, ui.ColorGreen)}
		// Use the same locally-computed effective value that was just pushed
		// into gpuEffectiveHistory (the plotted line): gpuMetrics.EffectiveLoad
		// is unset on the seed path and can lag the history's freq source, so
		// labeling from it can read 0%% while the line shows real load.
		gpuHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_GPUHistoryEff"),
			effectiveNow, gpuMetrics.FreqMHz, gpuMetrics.ActivePercent, currentPeak)
		gpuHistoryChart.DataLabels = []string{fmt.Sprintf("Eff %.0f%%", effectiveNow)}
	} else {
		gpuHistoryChart.Data = [][]float64{visibleRaw}
		gpuHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.GPU }, ui.ColorGreen)}
		gpuHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_GPUHistoryChart"), avgGPU)
		gpuHistoryChart.DataLabels = []string{fmt.Sprintf("%.0f%%", gpuMetrics.ActivePercent)}
	}
	gpuHistoryChart.MaxVal = 100
}

func getCachedLinkInfo() ([]EthernetLinkInfo, *WiFiLinkInfo) {
	linkInfoMutex.RLock()
	needsRefresh := time.Since(linkInfoLastUpdate) >= 5*time.Second
	ethInfo := cachedEthernetLinkInfo
	wifiInfo := cachedWiFiLinkInfo
	linkInfoMutex.RUnlock()

	if needsRefresh {
		linkInfoMutex.Lock()
		if time.Since(linkInfoLastUpdate) >= 5*time.Second {
			cachedEthernetLinkInfo = GetEthernetLinkInfo()
			cachedWiFiLinkInfo = GetWiFiLinkInfo()
			linkInfoLastUpdate = time.Now()
		}
		ethInfo = cachedEthernetLinkInfo
		wifiInfo = cachedWiFiLinkInfo
		linkInfoMutex.Unlock()
	}

	return ethInfo, wifiInfo
}

func getBestLinkInfoString(ethInfo []EthernetLinkInfo, wifiInfo *WiFiLinkInfo) string {
	var bestEth uint64
	for _, eth := range ethInfo {
		if eth.LinkUp && eth.LinkSpeedMbps > bestEth {
			bestEth = eth.LinkSpeedMbps
		}
	}

	bestWifi := 0
	if wifiInfo != nil && wifiInfo.IsConnected {
		bestWifi = wifiInfo.TxRateMbps
	}

	if bestEth > 0 && bestEth >= uint64(bestWifi) {
		return FormatLinkSpeed(bestEth)
	} else if wifiInfo != nil && wifiInfo.IsConnected {
		if wifiInfo.WiFiGeneration != "" {
			return fmt.Sprintf("%s", wifiInfo.WiFiGeneration)
		}
		return fmt.Sprintf("%dMbps", bestWifi)
	}

	return ""
}

func updateNetDiskUI(netdiskMetrics NetDiskMetrics) {
	// Update SSD read history (GB/s) for history_soc layout
	// Decimal GB/s (bytes/1e9), matching the adjacent DRAM/ANE bandwidth
	// charts so equal throughput plots at equal height under the shared
	// "GB/s" axis label. ReadKBytesPerSec is KiB/s (bytes/1024), so multiply
	// back to bytes before the decimal divide.
	readGBs := netdiskMetrics.ReadKBytesPerSec * 1024.0 / 1e9
	for i := 0; i < len(ssdReadHistory)-1; i++ {
		ssdReadHistory[i] = ssdReadHistory[i+1]
	}
	ssdReadHistory[len(ssdReadHistory)-1] = readGBs

	if ssdReadHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		visibleWidth := termWidth - 4
		if currentConfig.DefaultLayout == LayoutHistorySoC {
			visibleWidth = (termWidth / 3) - 4
		}
		if visibleWidth <= 0 || visibleWidth > len(ssdReadHistory) {
			visibleWidth = len(ssdReadHistory)
		}
		visibleData := ssdReadHistory[len(ssdReadHistory)-visibleWidth:]

		// True observed peak for the title; a separate floored value for the
		// Y-axis so an idle chart still has visible height without the title
		// claiming a 0.50 GB/s peak that never occurred.
		peak := seriesMax(visibleData)
		scaleMax := peak
		if scaleMax < 0.5 {
			scaleMax = 0.5
		}

		ssdReadHistoryChart.Data = [][]float64{visibleData}
		ssdReadHistoryChart.MaxVal = scaleMax * 1.3
		ssdReadHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_SSDReadDetail"), readGBs, peak)
	}

	var sb strings.Builder

	ethInfo, wifiInfo := getCachedLinkInfo()

	netOut := formatBytes(netdiskMetrics.OutBytesPerSec, networkUnit)
	netIn := formatBytes(netdiskMetrics.InBytesPerSec, networkUnit)

	linkInfo := getBestLinkInfoString(ethInfo, wifiInfo)

	if linkInfo != "" {
		fmt.Fprintf(&sb, i18n.T("Metrics_NetLink")+"\n", linkInfo, netOut, netIn)
	} else {
		fmt.Fprintf(&sb, i18n.T("Metrics_Net")+"\n", netOut, netIn)
	}

	diskRead := formatBytes(netdiskMetrics.ReadKBytesPerSec*1024, diskUnit)
	diskWrite := formatBytes(netdiskMetrics.WriteKBytesPerSec*1024, diskUnit)
	fmt.Fprintf(&sb, i18n.T("Metrics_IO")+"\n", diskRead, diskWrite)

	volumes := getVolumes()
	for i, v := range volumes {
		if i >= 3 {
			break
		}
		// VolumeInfo fields are stored in decimal GB (bytes / 1e9). Convert
		// back to raw bytes and format with decimal units to match macOS
		// Finder / Disk Utility (e.g. an 8TB drive shows as ~8.0 TB, not
		// 7.3 TiB).
		used := formatBytesDecimal(v.Used*1e9, diskUnit)
		total := formatBytesDecimal(v.Total*1e9, diskUnit)
		avail := formatBytesDecimal(v.Available*1e9, diskUnit)

		fmt.Fprintf(&sb, i18n.T("Metrics_DiskFree")+"\n", v.Name, used, total, avail)
	}
	NetworkInfo.Text = strings.TrimSuffix(sb.String(), "\n")
}

func updateTBNetUI(tbStats []ThunderboltNetStats) {
	if tbStats == nil {
		return
	}
	// Calculate total bandwidth from all Thunderbolt interfaces (in bytes/sec)
	var totalBytesIn, totalBytesOut float64
	for _, stat := range tbStats {
		totalBytesIn += stat.BytesInPerSec
		totalBytesOut += stat.BytesOutPerSec
	}
	lastTBInBytes = totalBytesIn
	lastTBOutBytes = totalBytesOut
	rdmaStatus := CheckRDMAAvailable()
	rdmaLabel := fmt.Sprintf("%s: %s", i18n.T("Info_RDMA"), i18n.T("Info_Disabled"))
	if rdmaStatus.Available {
		rdmaLabel = fmt.Sprintf("%s: %s", i18n.T("Info_RDMA"), i18n.T("Info_Enabled"))
	}

	// Use formatBytes for consistent unit display
	inStr := formatBytes(totalBytesIn, networkUnit)
	outStr := formatBytes(totalBytesOut, networkUnit)

	// Set simple title
	tbInfoParagraph.Title = i18n.T("TUI_ThunderboltRDMA")

	// Use cached device info
	tbInfoMutex.Lock()
	tbDeviceInfo := tbDeviceInfo
	tbInfoMutex.Unlock()
	if tbDeviceInfo == "" {
		tbDeviceInfo = i18n.T("TUI_Loading")
	}

	// Show RDMA status and bandwidth in text, above device list
	tbInfoParagraph.Text = fmt.Sprintf("%s | %s: ↓%s/s ↑%s/s\n%s", rdmaLabel, i18n.T("Info_TBNet"), inStr, outStr, tbDeviceInfo)

	// Update TB Net sparklines with separate download/upload
	// Shift values left and add new values
	// Scale bytes to KB for sparkline
	for i := 0; i < len(tbNetInValues)-1; i++ {
		tbNetInValues[i] = tbNetInValues[i+1]
		tbNetOutValues[i] = tbNetOutValues[i+1]
	}
	tbNetInValues[len(tbNetInValues)-1] = totalBytesIn / 1024
	tbNetOutValues[len(tbNetOutValues)-1] = totalBytesOut / 1024

	// Calculate independent max values for specific scaling
	maxValIn := 1.0
	for _, v := range tbNetInValues {
		if v > maxValIn {
			maxValIn = v
		}
	}
	maxValOut := 1.0
	for _, v := range tbNetOutValues {
		if v > maxValOut {
			maxValOut = v
		}
	}

	// Update sparklines and group title
	if tbNetSparklineGroup != nil {
		tbNetSparklineGroup.Title = fmt.Sprintf("%s: ↓%s/s ↑%s/s", i18n.T("Info_TBNet"), inStr, outStr)
		if tbNetSparklineIn != nil {
			tbNetSparklineIn.Data = tbNetInValues
			tbNetSparklineIn.MaxVal = maxValIn * 1.1
		}
		if tbNetSparklineOut != nil {
			tbNetSparklineOut.Data = tbNetOutValues
			tbNetSparklineOut.MaxVal = maxValOut * 1.1
		}
	}

}

func parseCommandLineFlags() {
	flag.StringVar(&prometheusPort, "prometheus", "", "Port to run Prometheus metrics server on (e.g. :9090)")
	flag.StringVar(&prometheusPort, "p", "", "Port to run Prometheus metrics server on (e.g. :9090)")
	flag.BoolVar(&headless, "headless", false, "Run in headless mode (no TUI, output JSON to stdout)")
	flag.BoolVar(&headlessPretty, "pretty", false, "Pretty print output in headless mode")
	flag.IntVar(&headlessCount, "count", 0, "Number of samples to collect in headless mode (0 = infinite)")
	flag.StringVar(&headlessFormat, "format", "json", "Output format for headless mode: json, yaml, xml, csv, toon")
	flag.IntVar(&updateInterval, "interval", 1000, "Update interval in milliseconds")
	flag.IntVar(&updateInterval, "i", 1000, "Update interval in milliseconds")
	flag.Bool("d", false, "Dump all available IOReport channels and exit")
	flag.Bool("dump-ioreport", false, "Dump all available IOReport channels and exit")
	flag.StringVar(&cliFgColor, "foreground", "", "Set the UI foreground color (named or hex, e.g., green, #9580FF)")
	flag.StringVar(&cliBgColor, "bg", "", "Set the UI background color (named or hex, e.g., mocha-base, #22212C)")
	flag.StringVar(&cliBgColor, "background", "", "Set the UI background color (alias for --bg)")
	flag.StringVar(&cliLanguage, "lang", "", "Language override (e.g., en, es, ja)")
	flag.StringVar(&networkUnit, "unit-network", "auto", "Network unit: auto, byte, kb, mb, gb")
	flag.StringVar(&diskUnit, "unit-disk", "auto", "Disk unit: auto, byte, kb, mb, gb")
	flag.StringVar(&tempUnit, "unit-temp", "celsius", "Temperature unit: celsius, fahrenheit")
	flag.BoolVar(&menubar, "menubar", false, "Run as macOS menu bar status item (no TUI)")
	flag.BoolVar(&menubarWorker, "menubar-worker", false, "Internal: Run as menu bar worker process")
	flag.BoolVar(&overlay, "overlay", false, "Show floating overlay HUD window on top of all apps (requires Screen Recording permission for FPS)")
	flag.BoolVar(&overlayWorker, "overlay-worker", false, "Internal: Run as overlay worker process")
	flag.StringVar(&overlaySections, "overlay-sections", "", "Comma-separated visible sections for overlay (e.g. cpu,gpu,memory)")
	flag.Float64Var(&overlayOpacity, "overlay-opacity", 0.88, "Overlay window opacity (0.15-1.0)")
	flag.IntVar(&filterPID, "pid", 0, "Monitor a specific process by PID")
	flag.BoolVar(&fanControl, "fan-control", false, "Enable interactive fan speed control (⚠️  writes to SMC)")
	flag.BoolVar(&dumpTemps, "dump-temps", false, "Diagnostic: dump all raw SMC temperature keys and exit")
	flag.BoolVar(&dumpDebug, "dump-debug", false, "Diagnostic: dump IOReport/HID/SMC/NVMe debug info and exit")
	flag.BoolVar(&dumpFPS, "dump-fps", false, "Diagnostic: dump display info and test CGDisplayStream FPS at multiple sizes")
}

func setupMainBlockLayout(termWidth, termHeight int) {
	mainBlock.SetRect(0, 0, termWidth, termHeight)
	if termWidth < 93 {
		mainBlock.TitleBottom = ""
	} else {
		mainBlock.TitleBottom = i18n.T("TUI_InfoLayoutColorExit")
	}
	if termWidth > 2 && termHeight > 2 {
		grid.SetRect(1, 1, termWidth-1, termHeight-1)
	}
}

func startBackgroundWorkers() {
	if menubar {
		if err := startMenuBarProcess(); err != nil {
			stderrLogger.Printf("Failed to start menubar worker: %v\n", err)
		}
	}
	if overlay {
		if err := startOverlayProcess(); err != nil {
			stderrLogger.Printf("Failed to start overlay worker: %v\n", err)
		}
	}
}

// shutdownWorkers kills any running overlay/menubar worker processes.
func shutdownWorkers() {
	overlayMu.Lock()
	if overlayWorkerStdin != nil {
		overlayWorkerStdin.Close()
		overlayWorkerStdin = nil
	}
	if overlayWorkerCmd != nil && overlayWorkerCmd.Process != nil {
		overlayWorkerCmd.Process.Kill()
		overlayWorkerCmd = nil
	}
	overlayMetricsEncoder = nil
	overlayMu.Unlock()

	menubarMu.Lock()
	if menubarWorkerStdin != nil {
		menubarWorkerStdin.Close()
		menubarWorkerStdin = nil
	}
	if menubarWorkerCmd != nil && menubarWorkerCmd.Process != nil {
		menubarWorkerCmd.Process.Kill()
		menubarWorkerCmd = nil
	}
	menubarMetricsEncoder = nil
	menubarMu.Unlock()
}

var shutdownOnce sync.Once

func shutdownAndExit(closeDone bool) {
	shutdownOnce.Do(func() {
		// Restore fans to automatic control FIRST. This is the only place fan
		// cleanup reliably runs: every quit path (q key, SIGINT/SIGTERM) ends
		// in os.Exit() below, which does NOT run deferred functions — so the
		// `defer cleanupFanControl()` in Run() never fires. Without this,
		// quitting while in --fan-control would leave the fans pinned in manual
		// mode at whatever RPM was last set. cleanupFanControl is a no-op when
		// fan control isn't active.
		cleanupFanControl()

		if closeDone {
			// Scope the recover to just close(done): if the channel is
			// already closed and panics, swallow it inside this inner
			// func so the rest of the shutdown sequence (workers, UI,
			// os.Exit) still runs. A defer at the outer scope would
			// abort the closure on panic, leaving subprocesses alive.
			func() {
				defer func() { _ = recover() }()
				close(done)
			}()
		}
		shutdownWorkers()
		saveConfigFlush()
		ui.Close()
		os.Exit(0)
	})
}
