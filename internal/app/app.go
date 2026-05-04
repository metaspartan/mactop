// Copyright (c) 2024-2026 Carsen Klock under MIT License
// mactop is a simple terminal based Apple Silicon power monitor written in Go Lang! github.com/metaspartan/mactop
package app

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"sync"

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

	systemInfoGauge.With(prometheus.Labels{
		"model":          modelName,
		"core_count":     fmt.Sprintf("%d", eCoreCount+pCoreCount+sCoreCount),
		"e_core_count":   fmt.Sprintf("%d", eCoreCount),
		"p_core_count":   fmt.Sprintf("%d", pCoreCount),
		"s_core_count":   fmt.Sprintf("%d", sCoreCount),
		"gpu_core_count": fmt.Sprintf("%d", gpuCoreCount),
	}).Set(1)

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
	mainBlock.Title = i18n.T("TUI_MactopTitle")
	mainBlock.TitleRight = " " + version + " "
	mainBlock.TitleAlignment = ui.AlignLeft
	mainBlock.TitleBottomLeft = fmt.Sprintf(i18n.T("TUI_LayoutInfo"), currentLayoutNum, totalLayouts, currentColorName)
	mainBlock.TitleBottom = i18n.T("TUI_InfoLayoutColorExit")
	mainBlock.TitleBottomAlignment = ui.AlignCenter
	mainBlock.TitleBottomRight = fmt.Sprintf(" -/+ %dms ", updateInterval)

	termWidth, termHeight := ui.TerminalDimensions()
	UpdateCachedTerminalDimensions(termWidth, termHeight)
	// Use full terminal width for StepChart data buffers (old sparkline sizing used half)
	numPoints := termWidth
	if numPoints < 500 {
		numPoints = 500 // Minimum buffer size
	}

	powerValues = make([]float64, numPoints)
	gpuValues = make([]float64, numPoints)
	memoryUsedHistory = make([]float64, numPoints)
	swapUsedHistory = make([]float64, numPoints)
	cpuUsageHistory = make([]float64, numPoints)
	powerUsageHistory = make([]float64, numPoints)

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
	rawHeight := termHeight - 2
	if rawHeight < 1 {
		rawHeight = 1
	}

	availableHeight := rawHeight
	maxOffset := 0

	// If content doesn't fit, we need to reserve space for indicators
	if len(lines) > rawHeight {
		// Reserve 2 lines (1 for top indicator/spacer, 1 for bottom indicator/spacer)
		availableHeight = rawHeight - 2
		if availableHeight < 1 {
			availableHeight = 1
		}
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
				cycleTheme()
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
	m := sampleSocMetrics(50)
	_, throttled := getThermalStateString()
	componentSum := m.TotalPower
	totalPower := componentSum
	systemResidual := 0.0

	if m.SystemPower > componentSum {
		totalPower = m.SystemPower
		systemResidual = m.SystemPower - componentSum
	}
	cpuMetricsChan <- CPUMetrics{
		CPUW:            m.CPUPower,
		GPUW:            m.GPUPower,
		ANEW:            m.ANEPower,
		DRAMW:           m.DRAMPower,
		GPUSRAMW:        m.GPUSRAMPower,
		SystemW:         systemResidual,
		PackageW:        totalPower,
		Throttled:       throttled,
		CPUTemp:         float64(m.CPUTemp),
		GPUTemp:         float64(m.GPUTemp),
		EClusterActive:  int(m.EClusterActive),
		PClusterActive:  int(m.PClusterActive),
		EClusterFreqMHz: int(m.EClusterFreqMHz),
		PClusterFreqMHz: int(m.PClusterFreqMHz),
		SClusterActive:  int(m.SClusterActive),
		SClusterFreqMHz: int(m.SClusterFreqMHz),
		DRAMReadBW:      m.DRAMReadBW,
		DRAMWriteBW:     m.DRAMWriteBW,
		DRAMBWCombined:  m.DRAMBWCombined,
		Fans:            m.Fans,
		TempSensors:     m.TempSensors,
	}
	gpuMetricsChan <- GPUMetrics{
		FreqMHz:       int(m.GPUFreqMHz),
		ActivePercent: m.GPUActive,
		Power:         m.GPUPower + m.GPUSRAMPower,
		Temp:          m.GPUTemp,
	}
	if processes, err := getProcessList(0.0); err == nil {
		processMetricsChan <- processes
	}
	netdiskMetricsChan <- getNetDiskMetrics()
}

func Run() {
	// Pre-resolve language from CLI args / env so that early-exit legacy flags
	// (--version, --help, --dump-ioreport, --test) honor --lang. This is a
	// best-effort scan since flag.Parse() hasn't run yet; the full priority
	// chain (CLI > env > config > system) is re-applied after loadConfig().
	earlyLang := earlyResolveLanguage()
	i18n.Init(earlyLang)

	colorName, interval, setColor, setInterval := handleLegacyFlags()

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

	// Ensure worker processes are killed on SIGINT/SIGTERM (e.g. terminal close)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
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
	if len(cpuMetrics.CoreUsages) > 0 {
		finalizeCPUUI(totalUsage, cpuMetrics.CoreUsages, cpuMetrics, memoryMetrics)
	}
}

func updateCPUHistory(totalUsage float64) {
	// Update CPU history StepChart
	for i := 0; i < len(cpuUsageHistory)-1; i++ {
		cpuUsageHistory[i] = cpuUsageHistory[i+1]
	}
	cpuUsageHistory[len(cpuUsageHistory)-1] = totalUsage

	if cpuHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()
		// CPU Chart is usually half width in LayoutHistoryFull
		visibleWidth := (termWidth / 2) - 4
		if visibleWidth <= 0 || visibleWidth > len(cpuUsageHistory) {
			visibleWidth = len(cpuUsageHistory)
		}
		if visibleWidth > 0 {
			visibleData := cpuUsageHistory[len(cpuUsageHistory)-visibleWidth:]

			// Calculate max value in visible data for adaptive scaling
			maxVal := 0.0
			for _, v := range visibleData {
				if v > maxVal {
					maxVal = v
				}
			}

			// Adaptive Scale: Snap to 25%, 50%, or 100%
			scaleMax := 100.0
			if maxVal <= 25.0 {
				scaleMax = 25.0
			} else if maxVal <= 50.0 {
				scaleMax = 50.0
			}

			cpuHistoryChart.Data = [][]float64{visibleData}
			cpuHistoryChart.MaxVal = scaleMax
			cpuHistoryChart.DataLabels = []string{fmt.Sprintf("%.0f%%", totalUsage)}
			cpuHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_CPUHistoryDetail"), totalUsage)
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
}

func finalizeCPUUI(totalUsage float64, coreUsages []float64, cpuMetrics CPUMetrics, memoryMetrics MemoryMetrics) {
	ecoreAvg, pcoreAvg, scoreAvg := calculateCoreAverages(coreUsages)
	updateCPUPrometheusMetrics(totalUsage, ecoreAvg, pcoreAvg, scoreAvg, coreUsages, cpuMetrics, memoryMetrics)

	// Update gauge colors with dynamic saturation if 1977 theme is active
	if currentConfig.Theme == "1977" {
		update1977GaugeColors()
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
	aneUtil := float64(cpuMetrics.ANEW / 1 / 8.0 * 100)
	if isCompactLayout() {
		aneGauge.Title = fmt.Sprintf(i18n.T("Metrics_ANEGaugeCompact"), cpuMetrics.ANEW)
	} else {
		aneGauge.Title = fmt.Sprintf(i18n.T("Metrics_ANEGauge"), aneUtil, cpuMetrics.ANEW)
	}
	aneGauge.Percent = int(aneUtil)
}

func updatePowerChartText(cpuMetrics CPUMetrics, thermalStr string) {
	PowerChart.Title = i18n.T("TUI_PowerUsage")

	if isCompactLayout() {
		PowerChart.Title = i18n.T("Metrics_PowerChartTitleCompact")
		PowerChart.Text = fmt.Sprintf(i18n.T("Metrics_PowerChartTextCompact"),
			cpuMetrics.CPUW,
			cpuMetrics.GPUW+cpuMetrics.GPUSRAMW,
			cpuMetrics.ANEW,
			cpuMetrics.DRAMW,
			cpuMetrics.PackageW,
			thermalStr,
		)
	} else {
		uptimeSeconds, _ := GetNativeUptime()
		uptimeStr := formatTime(float64(uptimeSeconds))

		PowerChart.Text = fmt.Sprintf(i18n.T("Metrics_PowerChartText"),
			cpuMetrics.CPUW,
			cpuMetrics.GPUW+cpuMetrics.GPUSRAMW,
			cpuMetrics.ANEW,
			cpuMetrics.DRAMW,
			cpuMetrics.SystemW,
			cpuMetrics.PackageW,
			thermalStr,
			uptimeStr,
		)
	}
}

func updateMemoryGaugeTitle(memoryMetrics MemoryMetrics) {
	if isCompactLayout() {
		memoryGauge.Title = fmt.Sprintf(i18n.T("Metrics_MemGaugeCompact"), float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024, lastCPUMetrics.DRAMBWCombined)
	} else {
		memoryGauge.Title = fmt.Sprintf(i18n.T("Metrics_MemGauge"), float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024, float64(memoryMetrics.SwapUsed)/1024/1024/1024, float64(memoryMetrics.SwapTotal)/1024/1024/1024, lastCPUMetrics.DRAMBWCombined)
	}
}

func calculateCoreAverages(coreUsages []float64) (ecoreAvg, pcoreAvg, scoreAvg float64) {
	if cpuCoreWidget.eCoreCount > 0 && len(coreUsages) >= cpuCoreWidget.eCoreCount {
		for i := 0; i < cpuCoreWidget.eCoreCount; i++ {
			ecoreAvg += coreUsages[i]
		}
		ecoreAvg /= float64(cpuCoreWidget.eCoreCount)
	}
	pStart := cpuCoreWidget.eCoreCount
	pEnd := pStart + cpuCoreWidget.pCoreCount
	if cpuCoreWidget.pCoreCount > 0 && len(coreUsages) >= pEnd {
		for i := pStart; i < pEnd; i++ {
			pcoreAvg += coreUsages[i]
		}
		pcoreAvg /= float64(cpuCoreWidget.pCoreCount)
	}
	sStart := pEnd
	sEnd := sStart + cpuCoreWidget.sCoreCount
	if cpuCoreWidget.sCoreCount > 0 && len(coreUsages) >= sEnd {
		for i := sStart; i < sEnd; i++ {
			scoreAvg += coreUsages[i]
		}
		scoreAvg /= float64(cpuCoreWidget.sCoreCount)
	}
	return ecoreAvg, pcoreAvg, scoreAvg
}

func updateCPUPrometheusMetrics(totalUsage, ecoreAvg, pcoreAvg, scoreAvg float64, coreUsages []float64, cpuMetrics CPUMetrics, memoryMetrics MemoryMetrics) {
	thermalStateNum := 0
	switch getThermalStateLevel() {
	case thermalStateFair:
		thermalStateNum = 1
	case thermalStateSerious:
		thermalStateNum = 2
	case thermalStateCritical:
		thermalStateNum = 3
	}

	cpuUsage.Set(totalUsage)
	ecoreUsage.Set(ecoreAvg)
	pcoreUsage.Set(pcoreAvg)
	scoreUsage.Set(scoreAvg)
	powerUsage.With(prometheus.Labels{"component": "cpu"}).Set(cpuMetrics.CPUW)
	powerUsage.With(prometheus.Labels{"component": "gpu"}).Set(cpuMetrics.GPUW)
	powerUsage.With(prometheus.Labels{"component": "ane"}).Set(cpuMetrics.ANEW)
	powerUsage.With(prometheus.Labels{"component": "dram"}).Set(cpuMetrics.DRAMW)
	powerUsage.With(prometheus.Labels{"component": "gpu_sram"}).Set(cpuMetrics.GPUSRAMW)
	powerUsage.With(prometheus.Labels{"component": "system"}).Set(cpuMetrics.SystemW)
	powerUsage.With(prometheus.Labels{"component": "total"}).Set(cpuMetrics.PackageW)
	socTemp.Set(cpuMetrics.CPUTemp)
	gpuTemp.Set(cpuMetrics.GPUTemp)
	thermalState.Set(float64(thermalStateNum))

	// DRAM bandwidth
	dramBandwidth.With(prometheus.Labels{"direction": "read"}).Set(cpuMetrics.DRAMReadBW)
	dramBandwidth.With(prometheus.Labels{"direction": "write"}).Set(cpuMetrics.DRAMWriteBW)
	dramBandwidth.With(prometheus.Labels{"direction": "combined"}).Set(cpuMetrics.DRAMBWCombined)

	memoryUsage.With(prometheus.Labels{"type": "used"}).Set(float64(memoryMetrics.Used) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "total"}).Set(float64(memoryMetrics.Total) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "swap_used"}).Set(float64(memoryMetrics.SwapUsed) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "swap_total"}).Set(float64(memoryMetrics.SwapTotal) / 1024 / 1024 / 1024)

	// Update per-core CPU usage metrics
	eCoreCount := cpuCoreWidget.eCoreCount
	pEnd := eCoreCount + cpuCoreWidget.pCoreCount
	for i, usage := range coreUsages {
		coreType := "s"
		if i < eCoreCount {
			coreType = "e"
		} else if i < pEnd {
			coreType = "p"
		}
		cpuCoreUsage.With(prometheus.Labels{"core": fmt.Sprintf("%d", i), "type": coreType}).Set(usage)
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
	}
	gpuValues[len(gpuValues)-1] = gpuMetrics.ActivePercent

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
	gpuSparkline.MaxVal = 100 // GPU usage is 0-100%
	if isCompactLayout() {
		gpuSparklineGroup.Title = fmt.Sprintf(i18n.T("Metrics_GPUSparklineCompact"), int(gpuMetrics.ActivePercent), avgGPU)
	} else {
		gpuSparklineGroup.Title = fmt.Sprintf(i18n.T("Metrics_GPUSparkline"), int(gpuMetrics.ActivePercent), avgGPU)
	}

	// Update GPU history StepChart - use terminal width for reliable slicing
	if gpuHistoryChart != nil {
		termWidth, _ := GetCachedTerminalDimensions()

		// Determine full vs half width based on layout
		visibleWidth := termWidth - 4
		if currentConfig.DefaultLayout == LayoutHistoryFull {
			visibleWidth = (termWidth / 2) - 4
		}

		if visibleWidth <= 0 || visibleWidth > len(gpuValues) {
			visibleWidth = len(gpuValues)
		}
		visibleData := gpuValues[len(gpuValues)-visibleWidth:]
		gpuHistoryChart.Data = [][]float64{visibleData}
		gpuHistoryChart.MaxVal = 100 // GPU usage is 0-100%
		gpuHistoryChart.DataLabels = []string{fmt.Sprintf("%.0f%%", gpuMetrics.ActivePercent)}
		gpuHistoryChart.Title = fmt.Sprintf(i18n.T("Metrics_GPUHistoryChart"), avgGPU)
	}

	if gpuMetrics.ActivePercent > 0 {
		gpuUsage.Set(gpuMetrics.ActivePercent)
	} else {
		gpuUsage.Set(0)
	}
	gpuFreqMHz.Set(float64(gpuMetrics.FreqMHz))

	// Update gauge colors with dynamic saturation if 1977 theme is active
	if currentConfig.Theme == "1977" {
		update1977GaugeColors()
	}
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

	// Update Prometheus metrics for Thunderbolt network and RDMA
	tbNetworkSpeed.With(prometheus.Labels{"direction": "download"}).Set(totalBytesIn)
	tbNetworkSpeed.With(prometheus.Labels{"direction": "upload"}).Set(totalBytesOut)
	if rdmaStatus.Available {
		rdmaAvailable.Set(1)
	} else {
		rdmaAvailable.Set(0)
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
		ui.Close()
		os.Exit(0)
	})
}
