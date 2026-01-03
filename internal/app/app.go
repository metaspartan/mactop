// Copyright (c) 2024-2026 Carsen Klock under MIT License
// mactop is a simple terminal based Apple Silicon power monitor written in Go Lang! github.com/metaspartan/mactop
package app

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"sync"

	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
)

var renderMutex sync.Mutex

func setupUI() {
	appleSiliconModel := getSOCInfo()
	modelText, helpText, infoParagraph = w.NewParagraph(), w.NewParagraph(), w.NewParagraph()
	modelText.Title = "Apple Silicon"
	helpText.Title = "mactop help menu"
	infoParagraph.Text = "Loading..."
	modelName := appleSiliconModel.Name
	if modelName == "" {
		modelName = "Unknown Model"
	}

	cachedHostname, _ = os.Hostname()
	cachedCurrentUser = os.Getenv("USER")
	cachedShell = os.Getenv("SHELL")

	kv, _ := exec.Command("uname", "-r").Output()
	cachedKernelVersion = strings.TrimSpace(string(kv))

	ov, _ := exec.Command("sw_vers", "-productVersion").Output()
	cachedOSVersion = strings.TrimSpace(string(ov))

	cachedModelName = modelName
	cachedSystemInfo = appleSiliconModel
	eCoreCount := appleSiliconModel.ECoreCount
	pCoreCount := appleSiliconModel.PCoreCount
	gpuCoreCount := appleSiliconModel.GPUCoreCount
	updateModelText()
	updateHelpText()
	stderrLogger.Printf("Model: %s\nE-Core Count: %d\nP-Core Count: %d\nGPU Core Count: %d", modelName, eCoreCount, pCoreCount, gpuCoreCount)

	systemInfoGauge.With(prometheus.Labels{
		"model":          modelName,
		"core_count":     fmt.Sprintf("%d", eCoreCount+pCoreCount),
		"e_core_count":   fmt.Sprintf("%d", eCoreCount),
		"p_core_count":   fmt.Sprintf("%d", pCoreCount),
		"gpu_core_count": fmt.Sprintf("%d", gpuCoreCount),
	}).Set(1)

	processList = w.NewList()
	processList.Title = "Process List"
	processList.TextStyle = ui.NewStyle(ui.ColorGreen)
	processList.WrapText = false
	processList.SelectedStyle = ui.NewStyle(ui.ColorBlack, ui.ColorGreen)
	processList.Rows = []string{}
	processList.SelectedRow = 0

	gauges := []*w.Gauge{
		w.NewGauge(), w.NewGauge(), w.NewGauge(), w.NewGauge(),
	}
	titles := []string{"E-CPU Usage", "P-CPU Usage", "GPU Usage", "Memory Usage", "ANE Usage"}
	for i, gauge := range gauges {
		gauge.Percent = 0
		gauge.Title = titles[i]
		gauge.Percent = 0
		gauge.Title = titles[i]
	}
	cpuGauge, gpuGauge, memoryGauge, aneGauge = gauges[0], gauges[1], gauges[2], gauges[3]

	PowerChart, NetworkInfo = w.NewParagraph(), w.NewParagraph()
	PowerChart.Title, NetworkInfo.Title = "Power Usage", "Network & Disk"

	tbInfoParagraph = w.NewParagraph()
	tbInfoParagraph.Title = "Thunderbolt / RDMA"
	tbInfoParagraph.Text = "Loading Thunderbolt Info..."
	go func() {
		description := GetThunderboltDescription()
		tbInfoMutex.Lock()
		tbDeviceInfo = description
		tbInfoMutex.Unlock()
	}()

	mainBlock = ui.NewBlock()
	mainBlock.BorderRounded = true
	mainBlock.Title = " mactop "
	mainBlock.TitleRight = " " + version + " "
	mainBlock.TitleAlignment = ui.AlignLeft
	mainBlock.TitleBottomLeft = fmt.Sprintf(" %d/%d layout (%s) ", currentLayoutNum, totalLayouts, currentColorName)
	mainBlock.TitleBottom = " Info: i | Layout: l | Color: c | BG: b | Exit: q "
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
	gpuSparklineGroup.Title = "GPU Usage History"

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
	tbNetSparklineGroup.Title = "TB Net ↓0/s ↑0/s"

	// StepChart widgets for History layout
	gpuHistoryChart = w.NewStepChart()
	gpuHistoryChart.Title = "GPU Usage History"
	gpuHistoryChart.ShowAxes = false
	gpuHistoryChart.ShowRightAxis = true
	gpuHistoryChart.LineColors = []ui.Color{ui.ColorGreen}

	powerHistoryChart = w.NewStepChart()
	powerHistoryChart.Title = "Power History"
	powerHistoryChart.ShowAxes = false
	powerHistoryChart.ShowRightAxis = true
	powerHistoryChart.LineColors = []ui.Color{ui.ColorYellow}

	memoryHistoryChart = w.NewStepChart()
	memoryHistoryChart.Title = "Memory/Swap History"
	memoryHistoryChart.ShowAxes = false
	memoryHistoryChart.ShowRightAxis = true
	memoryHistoryChart.LineColors = []ui.Color{ui.ColorBlue, ui.ColorMagenta}

	cpuHistoryChart = w.NewStepChart()
	cpuHistoryChart.Title = "CPU Usage History"
	cpuHistoryChart.ShowAxes = false
	cpuHistoryChart.ShowRightAxis = true
	cpuHistoryChart.LineColors = []ui.Color{ui.ColorGreen}

	updateProcessList()

	cpuCoreWidget = NewCPUCoreWidget(appleSiliconModel)
	eCoreCount = appleSiliconModel.ECoreCount
	pCoreCount = appleSiliconModel.PCoreCount
	cpuCoreWidget.Title = fmt.Sprintf("%d Cores (%dE/%dP)",
		eCoreCount+pCoreCount,
		eCoreCount,
		pCoreCount,
	)
	cpuGauge.Title = fmt.Sprintf("%d Cores (%dE/%dP)",
		eCoreCount+pCoreCount,
		eCoreCount,
		pCoreCount,
	)

	confirmModal = w.NewModal("CONFIRM KILL")
	confirmModal.Title = " CONFIRM "
	confirmModal.Border = true
	confirmModal.BorderRounded = true
	confirmModal.BorderStyle.Fg = ui.ColorRed
	confirmModal.BorderStyle.Bg = ui.ColorBlack
	confirmModal.TextStyle.Fg = ui.ColorWhite
	confirmModal.TextStyle.Bg = ui.ColorBlack
	confirmModal.ActiveButtonIndex = 1 // Default to No (Safe)

	_ = confirmModal.AddButton("Yes", func() {
		// Callback logic will be handled elsewhere or reused
	})
	_ = confirmModal.AddButton("No", func() {
		// Callback logic
	})
}

func updateModelText() {
	appleSiliconModel := getSOCInfo()
	modelName := appleSiliconModel.Name
	if modelName == "" {
		modelName = "Unknown Model"
	}
	eCoreCount := appleSiliconModel.ECoreCount
	pCoreCount := appleSiliconModel.PCoreCount
	gpuCoreCount := appleSiliconModel.GPUCoreCount

	gpuCoreCountStr := "?"
	if gpuCoreCount > 0 {
		gpuCoreCountStr = fmt.Sprintf("%d", gpuCoreCount)
	}

	modelText.Text = fmt.Sprintf("%s\n%d Cores\n%d E-Cores\n%d P-Cores\n%s GPU Cores",
		modelName,
		eCoreCount+pCoreCount,
		eCoreCount,
		pCoreCount,
		gpuCoreCountStr,
	)
}

func updateIntervalText() {
	mainBlock.TitleBottomRight = fmt.Sprintf(" -/+ %dms ", updateInterval)
}

func updateInfoUI() {
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
	helpText.Text = fmt.Sprintf(
		"mactop is open source monitoring tool for Apple Silicon authored by Carsen Klock in Go Lang!\n\n"+
			"Repo: github.com/metaspartan/mactop\n\n"+
			"Prometheus Metrics: %s\n\n"+
			"Controls:\n"+
			"- r: Refresh the UI data manually\n"+
			"- c: Cycle through UI color themes\n"+
			"- b: Cycle through UI background colors\n"+
			"- p: Toggle party mode (color cycling)\n"+
			"- l: Cycle through the 17 available layouts\n"+
			"- F9: Kill selected process (y/n confirm)\n"+ // Updated text
			"- /: Search process list\n"+ // Added help
			"- g/G: Jump to top/bottom of process list\n"+ // Added help
			"- + or -: Adjust update interval (faster/slower)\n"+
			"- h or ?: Toggle this help menu\n"+
			"- q or <C-c>: Quit the application\n\n"+
			"Start Flags:\n"+
			"--help, -h: Show this help menu\n"+
			"--version, -v: Show the version of mactop\n"+
			"--interval, -i: Set the update interval in milliseconds. Default is 1000.\n"+
			"--prometheus, -p: Set and enable a Prometheus metrics port. Default is none. (e.g. --prometheus=9090)\n"+
			"--headless: Run in headless mode (no TUI, output to stdout)\n"+
			"--format: Output format for headless mode (json, yaml, xml, csv, toon). Default is json.\n"+
			"--pretty: Pretty print output in headless mode\n"+
			"--count: Number of samples to collect in headless mode (0 = infinite)\n"+
			"--dump-ioreport, -d: Dump all available IOReport channels and exit\n"+
			"--unit-network: Network unit: auto, byte, kb, mb, gb (default: auto)\n"+
			"--unit-disk: Disk unit: auto, byte, kb, mb, gb (default: auto)\n"+
			"--unit-temp: Temperature unit: celsius, fahrenheit (default: celsius)\n"+
			"--color, -c: Set the UI color. Default is none. Options are 'green', 'red', 'blue', 'skyblue', 'magenta', 'yellow', 'gold', 'silver', and 'white'.\n\n"+
			"Version: %s\n\n"+
			"Current Settings:\n"+
			"Layout: %s\n"+
			"Theme: %s\n"+
			"Update Interval: %dms",
		prometheusStatus,
		version,
		currentConfig.DefaultLayout,
		currentConfig.Theme,
		updateInterval,
	)
}

func toggleHelpMenu() {
	updateHelpText()
	showHelp = !showHelp

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

func applyInitialTheme(colorName string, setColor bool, interval int, setInterval bool) {
	if setColor {
		applyTheme(colorName, IsLightMode)
	} else {
		if currentConfig.Theme == "" {
			currentConfig.Theme = "green"
		}
		applyTheme(currentConfig.Theme, IsLightMode)
	}
	if setInterval {
		updateInterval = interval
		updateIntervalText()
	}
}

func Run() {
	colorName, interval, setColor, setInterval := handleLegacyFlags()

	logfile, err := setupLogfile()
	if err != nil {
		stderrLogger.Fatalf("failed to setup log file: %v", err)
	}
	defer logfile.Close()

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
	flag.StringVar(&colorName, "color", "", "Set the UI color. Options are 'green', 'red', 'blue', 'skyblue', 'magenta', 'yellow', 'gold', 'silver', and 'white'.")
	flag.StringVar(&colorName, "c", "", "Set the UI color.")
	flag.StringVar(&networkUnit, "unit-network", "auto", "Network unit: auto, byte, kb, mb, gb")
	flag.StringVar(&diskUnit, "unit-disk", "auto", "Disk unit: auto, byte, kb, mb, gb")
	flag.StringVar(&tempUnit, "unit-temp", "celsius", "Temperature unit: celsius, fahrenheit")

	loadConfig()

	// Load saved sort column from config (only if explicitly set)
	if currentConfig.SortColumn != nil && *currentConfig.SortColumn >= 0 && *currentConfig.SortColumn < len(columns) {
		selectedColumn = *currentConfig.SortColumn
	}
	sortReverse = currentConfig.SortReverse

	flag.Parse()

	currentUser = os.Getenv("USER")

	if headless {
		runHeadless(headlessCount)
		return
	}

	IsLightMode = detectLightMode()

	if err := ui.Init(); err != nil {
		stderrLogger.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	if err := initSocMetrics(); err != nil {
		stderrLogger.Fatalf("failed to initialize metrics: %v", err)
	}
	defer cleanupSocMetrics()

	StderrToLogfile(logfile)

	if prometheusPort != "" {
		startPrometheusServer(prometheusPort)
		stderrLogger.Printf("Prometheus metrics available at http://localhost:%s/metrics\n", prometheusPort)
	}
	setupUI()
	applyInitialTheme(colorName, setColor, interval, setInterval)
	currentColorName = currentConfig.Theme
	applyInitialBackground()
	setupGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	mainBlock.SetRect(0, 0, termWidth, termHeight)
	if termWidth < 93 {
		mainBlock.TitleBottom = ""
	} else {
		mainBlock.TitleBottom = " Info: i | Layout: l | Color: c | BG: b | Exit: q "
	}
	if termWidth > 2 && termHeight > 2 {
		grid.SetRect(1, 1, termWidth-1, termHeight-1)
	}
	renderUI()

	initialSocMetrics := sampleSocMetrics(100)
	_, throttled := getThermalStateString()
	componentSum := initialSocMetrics.TotalPower
	totalPower := componentSum
	systemResidual := 0.0

	if initialSocMetrics.SystemPower > componentSum {
		totalPower = initialSocMetrics.SystemPower
		systemResidual = initialSocMetrics.SystemPower - componentSum
	}
	cpuMetrics := CPUMetrics{
		CPUW:      initialSocMetrics.CPUPower,
		GPUW:      initialSocMetrics.GPUPower,
		ANEW:      initialSocMetrics.ANEPower,
		DRAMW:     initialSocMetrics.DRAMPower,
		GPUSRAMW:  initialSocMetrics.GPUSRAMPower,
		SystemW:   systemResidual,
		PackageW:  totalPower,
		Throttled: throttled,
		CPUTemp:   float64(initialSocMetrics.CPUTemp),
		GPUTemp:   float64(initialSocMetrics.GPUTemp),
	}
	gpuMetrics := GPUMetrics{
		FreqMHz:       int(initialSocMetrics.GPUFreqMHz),
		ActivePercent: initialSocMetrics.GPUActive,
		Power:         initialSocMetrics.GPUPower + initialSocMetrics.GPUSRAMPower,
		Temp:          initialSocMetrics.GPUTemp,
	}

	cpuMetricsChan <- cpuMetrics
	gpuMetricsChan <- gpuMetrics

	if processes, err := getProcessList(0.0); err == nil {
		processMetricsChan <- processes
	}

	netdiskMetricsChan <- getNetDiskMetrics()

	triggerProcessCollectionChan := make(chan struct{}, 1)

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

	handleEvents(done, uiEvents)
}

func setupLogfile() (*os.File, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}
	logDir := filepath.Join(homeDir, ".mactop")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to make the log directory: %v", err)
	}
	logPath := filepath.Join(logDir, "mactop.log")
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
	sparklineGroup.Title = fmt.Sprintf("%.2f W Total (Max: %.2f W)", watts, maxPowerSeen)
	thermalStr, _ := getThermalStateString()
	sparkline.Title = fmt.Sprintf("Avg: %.2f W | %s", avgWatts, thermalStr)

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
		powerHistoryChart.Title = fmt.Sprintf("Power History (Avg: %.1fW, Max: %.1fW)", avgWatts, maxPowerSeen)
	}
}

func updateCPUUI(cpuMetrics CPUMetrics) {
	coreUsages, err := GetCPUPercentages()
	if err != nil {
		stderrLogger.Printf("Error getting CPU percentages: %v\n", err)
		return
	}
	cpuCoreWidget.UpdateUsage(coreUsages)
	var totalUsage float64
	for _, usage := range coreUsages {
		totalUsage += usage
	}
	totalUsage /= float64(len(coreUsages))
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
	finalizeCPUUI(totalUsage, coreUsages, cpuMetrics, memoryMetrics)
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
			cpuHistoryChart.Title = fmt.Sprintf("CPU Usage History (%.1f%%)", totalUsage)
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
		memoryHistoryChart.Title = fmt.Sprintf("Mem: %.1f/%.1fGB, Swap: %.1fGB",
			usedGB, totalGB, swapGB)
	}
}

func finalizeCPUUI(totalUsage float64, coreUsages []float64, cpuMetrics CPUMetrics, memoryMetrics MemoryMetrics) {
	ecoreAvg, pcoreAvg := calculateCoreAverages(coreUsages)
	updateCPUPrometheusMetrics(totalUsage, ecoreAvg, pcoreAvg, coreUsages, cpuMetrics, memoryMetrics)

	// Update gauge colors with dynamic saturation if 1977 theme is active
	if currentConfig.Theme == "1977" {
		update1977GaugeColors()
	}
}

func updateCPUGaugeTitles(totalUsage float64, cpuMetrics CPUMetrics) {
	if isCompactLayout() {
		cpuGauge.Title = fmt.Sprintf("CPU %.0f%% %s", totalUsage, formatTemp(cpuMetrics.CPUTemp))
	} else {
		cpuGauge.Title = fmt.Sprintf("%d Cores (%dE/%dP) %.2f%% (%s)",
			cpuCoreWidget.eCoreCount+cpuCoreWidget.pCoreCount,
			cpuCoreWidget.eCoreCount,
			cpuCoreWidget.pCoreCount,
			totalUsage,
			formatTemp(cpuMetrics.CPUTemp),
		)
	}
	cpuCoreWidget.Title = fmt.Sprintf("%d Cores (%dE/%dP) %.2f%% (%s)",
		cpuCoreWidget.eCoreCount+cpuCoreWidget.pCoreCount,
		cpuCoreWidget.eCoreCount,
		cpuCoreWidget.pCoreCount,
		totalUsage,
		formatTemp(cpuMetrics.CPUTemp),
	)
	aneUtil := float64(cpuMetrics.ANEW / 1 / 8.0 * 100)
	if isCompactLayout() {
		aneGauge.Title = fmt.Sprintf("ANE %.1fW", cpuMetrics.ANEW)
	} else {
		aneGauge.Title = fmt.Sprintf("ANE Usage: %.2f%% @ %.2f W", aneUtil, cpuMetrics.ANEW)
	}
	aneGauge.Percent = int(aneUtil)
}

func updatePowerChartText(cpuMetrics CPUMetrics, thermalStr string) {
	PowerChart.Title = "Power Usage"
	if isCompactLayout() {
		PowerChart.Title = "Power"
		PowerChart.Text = fmt.Sprintf("C:%.1fW G:%.1fW\nA:%.1fW D:%.1fW\nTot:%.1fW %s",
			cpuMetrics.CPUW,
			cpuMetrics.GPUW+cpuMetrics.GPUSRAMW,
			cpuMetrics.ANEW,
			cpuMetrics.DRAMW,
			cpuMetrics.PackageW,
			thermalStr,
		)
	} else {
		PowerChart.Text = fmt.Sprintf("CPU: %.2f W | GPU: %.2f W\nANE: %.2f W | DRAM: %.2f W\nSystem: %.2f W\nTotal: %.2f W\nThermals: %s",
			cpuMetrics.CPUW,
			cpuMetrics.GPUW+cpuMetrics.GPUSRAMW,
			cpuMetrics.ANEW,
			cpuMetrics.DRAMW,
			cpuMetrics.SystemW,
			cpuMetrics.PackageW,
			thermalStr,
		)
	}
}

func updateMemoryGaugeTitle(memoryMetrics MemoryMetrics) {
	if isCompactLayout() {
		memoryGauge.Title = fmt.Sprintf("Mem %.0f/%.0fG", float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024)
	} else {
		memoryGauge.Title = fmt.Sprintf("Memory: %.2f GB / %.2f GB (Swap: %.2f/%.2f GB)", float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024, float64(memoryMetrics.SwapUsed)/1024/1024/1024, float64(memoryMetrics.SwapTotal)/1024/1024/1024)
	}
}

func calculateCoreAverages(coreUsages []float64) (ecoreAvg, pcoreAvg float64) {
	if cpuCoreWidget.eCoreCount > 0 && len(coreUsages) >= cpuCoreWidget.eCoreCount {
		for i := 0; i < cpuCoreWidget.eCoreCount; i++ {
			ecoreAvg += coreUsages[i]
		}
		ecoreAvg /= float64(cpuCoreWidget.eCoreCount)
	}
	if cpuCoreWidget.pCoreCount > 0 && len(coreUsages) >= cpuCoreWidget.eCoreCount+cpuCoreWidget.pCoreCount {
		for i := cpuCoreWidget.eCoreCount; i < cpuCoreWidget.eCoreCount+cpuCoreWidget.pCoreCount; i++ {
			pcoreAvg += coreUsages[i]
		}
		pcoreAvg /= float64(cpuCoreWidget.pCoreCount)
	}
	return ecoreAvg, pcoreAvg
}

func updateCPUPrometheusMetrics(totalUsage, ecoreAvg, pcoreAvg float64, coreUsages []float64, cpuMetrics CPUMetrics, memoryMetrics MemoryMetrics) {
	thermalStateVal, _ := getThermalStateString()
	thermalStateNum := 0
	switch thermalStateVal {
	case "Fair":
		thermalStateNum = 1
	case "Serious":
		thermalStateNum = 2
	case "Critical":
		thermalStateNum = 3
	}

	cpuUsage.Set(totalUsage)
	ecoreUsage.Set(ecoreAvg)
	pcoreUsage.Set(pcoreAvg)
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

	memoryUsage.With(prometheus.Labels{"type": "used"}).Set(float64(memoryMetrics.Used) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "total"}).Set(float64(memoryMetrics.Total) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "swap_used"}).Set(float64(memoryMetrics.SwapUsed) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "swap_total"}).Set(float64(memoryMetrics.SwapTotal) / 1024 / 1024 / 1024)

	// Update per-core CPU usage metrics
	eCoreCount := cpuCoreWidget.eCoreCount
	for i, usage := range coreUsages {
		coreType := "p"
		if i < eCoreCount {
			coreType = "e"
		}
		cpuCoreUsage.With(prometheus.Labels{"core": fmt.Sprintf("%d", i), "type": coreType}).Set(usage)
	}
}

func updateGPUUI(gpuMetrics GPUMetrics) {
	if isCompactLayout() {
		if gpuMetrics.Temp > 0 {
			gpuGauge.Title = fmt.Sprintf("GPU %d%% %s", int(gpuMetrics.ActivePercent), formatTemp(float64(gpuMetrics.Temp)))
		} else {
			gpuGauge.Title = fmt.Sprintf("GPU %d%% %dMHz", int(gpuMetrics.ActivePercent), gpuMetrics.FreqMHz)
		}
	} else {
		if gpuMetrics.Temp > 0 {
			gpuGauge.Title = fmt.Sprintf("GPU Usage: %d%% @ %d MHz (%s)", int(gpuMetrics.ActivePercent), gpuMetrics.FreqMHz, formatTemp(float64(gpuMetrics.Temp)))
		} else {
			gpuGauge.Title = fmt.Sprintf("GPU Usage: %d%% @ %d MHz", int(gpuMetrics.ActivePercent), gpuMetrics.FreqMHz)
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
		gpuSparklineGroup.Title = fmt.Sprintf("GPU %d%% (%.0f%%)", int(gpuMetrics.ActivePercent), avgGPU)
	} else {
		gpuSparklineGroup.Title = fmt.Sprintf("GPU History: %d%% (Avg: %.1f%%)", int(gpuMetrics.ActivePercent), avgGPU)
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
		gpuHistoryChart.Title = fmt.Sprintf("GPU Usage History (Avg: %.1f%%)", avgGPU)
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

func updateNetDiskUI(netdiskMetrics NetDiskMetrics) {
	var sb strings.Builder

	// Network metrics are in Bytes/sec
	netOut := formatBytes(netdiskMetrics.OutBytesPerSec, networkUnit)
	netIn := formatBytes(netdiskMetrics.InBytesPerSec, networkUnit)
	fmt.Fprintf(&sb, "Net: ↑ %s/s ↓ %s/s\n", netOut, netIn)

	// Disk metrics are in KB/s, convert to Bytes for formatBytes
	diskRead := formatBytes(netdiskMetrics.ReadKBytesPerSec*1024, diskUnit)
	diskWrite := formatBytes(netdiskMetrics.WriteKBytesPerSec*1024, diskUnit)
	fmt.Fprintf(&sb, "I/O: R %s/s W %s/s\n", diskRead, diskWrite)

	volumes := getVolumes()
	for i, v := range volumes {
		if i >= 3 {
			break
		}
		// Volume info is in GB. Convert to Bytes for formatBytes
		used := formatBytes(v.Used*1024*1024*1024, diskUnit)
		total := formatBytes(v.Total*1024*1024*1024, diskUnit)
		avail := formatBytes(v.Available*1024*1024*1024, diskUnit)

		fmt.Fprintf(&sb, "%s: %s/%s (%s free)\n", v.Name, used, total, avail)
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
	rdmaLabel := "RDMA: Disabled"
	if rdmaStatus.Available {
		rdmaLabel = "RDMA: Enabled"
	}

	// Use formatBytes for consistent unit display
	inStr := formatBytes(totalBytesIn, networkUnit)
	outStr := formatBytes(totalBytesOut, networkUnit)

	// Set simple title
	tbInfoParagraph.Title = "Thunderbolt / RDMA"

	// Use cached device info
	tbInfoMutex.Lock()
	tbDeviceInfo := tbDeviceInfo
	tbInfoMutex.Unlock()
	if tbDeviceInfo == "" {
		tbDeviceInfo = "Loading..."
	}

	// Show RDMA status and bandwidth in text, above device list
	tbInfoParagraph.Text = fmt.Sprintf("%s | TB Net: ↓%s/s ↑%s/s\n%s", rdmaLabel, inStr, outStr, tbDeviceInfo)

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
		tbNetSparklineGroup.Title = fmt.Sprintf("TB Net: ↓%s/s ↑%s/s", inStr, outStr)
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
