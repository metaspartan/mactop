package app

import (
	"image"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		val      float64
		unitType string
		want     string
	}{
		{"Auto Bytes", 500, "auto", "500.0B"},
		{"Auto KB", 1500, "auto", "1.5KB"},
		{"Auto MB", 1024 * 1024 * 2.5, "auto", "2.5MB"},
		{"Force KB", 2048, "kb", "2.0KB"},
		{"Force MB", 1024 * 1024 * 5, "mb", "5.0MB"},
		{"Force GB", 1024 * 1024 * 1024, "gb", "1.0GB"},
		{"Unknown Unit (Default Auto)", 1024, "xyz", "1.0KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBytes(tt.val, tt.unitType); got != tt.want {
				t.Errorf("formatBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTemp(t *testing.T) {
	// Save original state
	origTempUnit := tempUnit
	defer func() { tempUnit = origTempUnit }()

	tests := []struct {
		name    string
		celsius float64
		unit    string
		want    string
	}{
		{"Celsius Default", 25.0, "celsius", "25°C"},
		{"Fahrenheit Conversion", 0.0, "fahrenheit", "32°F"},
		{"Fahrenheit Boiling", 100.0, "fahrenheit", "212°F"},
		{"Celsius Negative", -10.0, "celsius", "-10°C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempUnit = tt.unit
			if got := formatTemp(tt.celsius); got != tt.want {
				t.Errorf("formatTemp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiedComputeHistoryTitleIncludesAvailableTemperatures(t *testing.T) {
	origTempUnit := tempUnit
	t.Cleanup(func() { tempUnit = origTempUnit })
	tempUnit = "celsius"

	if got := unifiedComputeHistoryTitle(CPUMetrics{CPUTemp: 42}, GPUMetrics{Temp: 51}); !strings.Contains(got, "CPU 42°C | GPU 51°C") {
		t.Fatalf("title %q does not contain CPU/GPU temperatures", got)
	}
	if got := unifiedComputeHistoryTitle(CPUMetrics{}, GPUMetrics{}); strings.Contains(got, "°") {
		t.Fatalf("title %q must omit unavailable temperatures", got)
	}
}

func TestNewCPUMetrics(t *testing.T) {
	m := NewCPUMetrics()
	if m.CoreMetrics == nil {
		t.Error("CoreMetrics map should be initialized")
	}
	if m.ECores == nil {
		t.Error("ECores slice should be initialized")
	}
	if m.PCores == nil {
		t.Error("PCores slice should be initialized")
	}
}

func TestUnifiedLayoutIsLast(t *testing.T) {
	if len(layoutOrder) == 0 {
		t.Fatal("layoutOrder must not be empty")
	}
	if got := layoutOrder[len(layoutOrder)-1]; got != LayoutUnified {
		t.Fatalf("last layout = %q, want %q", got, LayoutUnified)
	}
	if got, want := len(layoutOrder), 21; got != want {
		t.Fatalf("layout count = %d, want %d", got, want)
	}
}

func TestShiftAndAppend(t *testing.T) {
	history := []float64{1, 2, 3}
	shiftAndAppend(history, 4)
	want := []float64{2, 3, 4}
	for i, value := range want {
		if got := history[i]; got != value {
			t.Fatalf("history[%d] = %.1f, want %.1f", i, got, value)
		}
	}
}

func TestSeriesMaxUsesObservedSamples(t *testing.T) {
	if got, want := seriesMax([]float64{0.2, 4.7, 3.1}), 4.7; got != want {
		t.Fatalf("series max = %.1f, want %.1f", got, want)
	}
	if got := seriesMax(nil); got != 0 {
		t.Fatalf("empty series max = %.1f, want 0", got)
	}
}

func TestNormalizeHistorySeriesPreservesRelativeTrend(t *testing.T) {
	got := normalizeHistorySeries([]float64{2, 4, 8})
	want := []float64{25, 50, 100}
	if !slices.Equal(got, want) {
		t.Fatalf("normalized series = %v, want %v", got, want)
	}
	if got := normalizeHistorySeries([]float64{0, 0}); !slices.Equal(got, []float64{0, 0}) {
		t.Fatalf("zero series = %v, want all zeros", got)
	}
}

func TestRenderUnifiedMemoryDRAMHistoryUsesFourNormalizedSeries(t *testing.T) {
	origConfig := currentConfig
	origWidth, origHeight := GetCachedTerminalDimensions()
	origChart := memoryHistoryChart
	origMemory, origSwap := memoryUsedHistory, swapUsedHistory
	origRead, origWrite := dramReadHistory, dramWriteHistory
	t.Cleanup(func() {
		currentConfig = origConfig
		UpdateCachedTerminalDimensions(origWidth, origHeight)
		memoryHistoryChart = origChart
		memoryUsedHistory, swapUsedHistory = origMemory, origSwap
		dramReadHistory, dramWriteHistory = origRead, origWrite
	})

	currentConfig.DefaultLayout = LayoutUnified
	UpdateCachedTerminalDimensions(20, 10)
	memoryHistoryChart = NewPeakStepChart()
	memoryUsedHistory = []float64{10, 20, 40}
	swapUsedHistory = []float64{1, 2, 4}
	dramReadHistory = []float64{2, 4, 8}
	dramWriteHistory = []float64{3, 6, 12}

	renderUnifiedMemoryDRAMHistory(
		MemoryMetrics{Used: 40 * 1024 * 1024 * 1024, SwapUsed: 4 * 1024 * 1024 * 1024},
		CPUMetrics{DRAMReadBW: 8, DRAMWriteBW: 12},
	)

	if got, want := len(memoryHistoryChart.Data), 4; got != want {
		t.Fatalf("mixed series count = %d, want %d", got, want)
	}
	wantPeaks := []float64{10, 100, 100 * 8.0 / 12.0, 100}
	for i, series := range memoryHistoryChart.Data {
		if got := seriesMax(series); math.Abs(got-wantPeaks[i]) > 0.001 {
			t.Fatalf("series %d normalized peak = %.3f, want %.3f", i, got, wantPeaks[i])
		}
	}
	if got, want := memoryHistoryChart.MaxVal, 100.0; got != want {
		t.Fatalf("mixed chart max = %.1f, want %.1f", got, want)
	}
	if got, want := memoryHistoryChart.DataLabels, []string{"S 4.0GB", "M 40.0GB", "R 8.0GB/s", "W 12.0GB/s"}; !slices.Equal(got, want) {
		t.Fatalf("mixed current labels = %v, want %v", got, want)
	}
	if got, want := memoryHistoryChart.PeakLabels, []string{"S 4.0GB", "M 40.0GB", "R 8.0GB/s", "W 12.0GB/s"}; !slices.Equal(got, want) {
		t.Fatalf("mixed peak labels = %v, want %v", got, want)
	}
	if got, want := memoryHistoryChart.LineColors[:2], []ui.Color{ui.ColorOrange, ui.ColorMagenta}; !slices.Equal(got, want) {
		t.Fatalf("memory draw order colors = %v, want swap then memory %v", got, want)
	}
	if got := memoryHistoryChart.Title; !strings.Contains(got, "(Memory / Swap, GB)") || !strings.Contains(got, "(Read / Write, GB/s)") {
		t.Fatalf("memory chart title does not contain English legends: %q", got)
	}
}

func TestUnifiedChartTitleLocalizesBaseAndUsesEnglishLegend(t *testing.T) {
	if got := unifiedChartTitle("TUI_UnifiedNetworkHistory", "Download / Upload"); !strings.HasSuffix(got, " (Download / Upload)") || strings.Contains(got, "下行") {
		t.Fatalf("unified chart title has incorrect legend: %q", got)
	}
	if got := unifiedChartTitle("TUI_Temperatures", ""); strings.Contains(got, "(") {
		t.Fatalf("non-legend title unexpectedly contains legend: %q", got)
	}
}

func TestRenderUnifiedMemoryHistoryUsesChartColumnWidth(t *testing.T) {
	origConfig := currentConfig
	origWidth, origHeight := GetCachedTerminalDimensions()
	origChart := memoryHistoryChart
	origMemory, origSwap := memoryUsedHistory, swapUsedHistory
	origRead, origWrite := dramReadHistory, dramWriteHistory
	t.Cleanup(func() {
		currentConfig = origConfig
		UpdateCachedTerminalDimensions(origWidth, origHeight)
		memoryHistoryChart = origChart
		memoryUsedHistory, swapUsedHistory = origMemory, origSwap
		dramReadHistory, dramWriteHistory = origRead, origWrite
	})

	currentConfig.DefaultLayout = LayoutUnified
	UpdateCachedTerminalDimensions(120, 40)
	memoryHistoryChart = NewPeakStepChart()
	memoryUsedHistory = make([]float64, 120)
	swapUsedHistory = make([]float64, 120)
	dramReadHistory = make([]float64, 120)
	dramWriteHistory = make([]float64, 120)
	memoryUsedHistory[119] = 1

	renderUnifiedMemoryDRAMHistory(MemoryMetrics{}, CPUMetrics{})
	if got, want := len(memoryHistoryChart.Data[1]), 76; got != want {
		t.Fatalf("visible memory samples = %d, want chart width %d", got, want)
	}
	if got := memoryHistoryChart.Data[1][75]; got != 100 {
		t.Fatalf("newest memory sample = %.1f, want visible normalized sample 100", got)
	}
}

func TestUnifiedTemperatureLinesPreferComponentTemperatures(t *testing.T) {
	origTempUnit := tempUnit
	t.Cleanup(func() { tempUnit = origTempUnit })
	tempUnit = "celsius"
	lines := unifiedTemperatureLines(CPUMetrics{
		CPUTemp: 50,
		GPUTemp: 48,
		TempSensors: []TempSensor{
			{Key: "Tm0", Name: "Memory", Value: 44},
			{Key: "Ts0", Name: "SSD", Value: 40},
		},
	})
	if len(lines) < 2 || lines[0] != "CPU 50°C | GPU 48°C" || lines[1] != "Memory 44°C | SSD 40°C" {
		t.Fatalf("temperature lines = %v", lines)
	}
}

func TestReadWriteHistoryColors(t *testing.T) {
	origConfig := currentConfig
	t.Cleanup(func() { currentConfig = origConfig })

	currentConfig.CustomTheme = nil
	if got, want := readWriteHistoryColors(), []ui.Color{ui.ColorCyan, ui.ColorRed}; !slices.Equal(got, want) {
		t.Fatalf("default read/write colors = %v, want %v", got, want)
	}

	currentConfig.CustomTheme = &CustomThemeConfig{Bandwidth: "#123456"}
	read, err := ParseHexColor("#123456")
	if err != nil {
		t.Fatalf("ParseHexColor: %v", err)
	}
	if got, want := readWriteHistoryColors(), []ui.Color{read, ui.ColorRed}; !slices.Equal(got, want) {
		t.Fatalf("custom read/write colors = %v, want %v", got, want)
	}
}

func TestUnifiedComputeHistoryColors(t *testing.T) {
	got := unifiedComputeHistoryColors()
	// StepChart draws in slice order, so ANE is underneath GPU and CPU.
	want := []ui.Color{ui.ColorGreen, ui.ColorYellow, ui.ColorRed}
	if !slices.Equal(got, want) {
		t.Fatalf("unified compute colors = %v, want %v", got, want)
	}
}

func TestRenderUnifiedComputeHistoryDrawsCPUAboveGPUAboveANE(t *testing.T) {
	origChart, origCPU, origGPU, origANE := unifiedComputeHistoryChart, cpuUsageHistory, gpuEffectiveHistory, aneUsageHistory
	origMetrics := lastCPUMetrics
	t.Cleanup(func() {
		unifiedComputeHistoryChart, cpuUsageHistory, gpuEffectiveHistory, aneUsageHistory = origChart, origCPU, origGPU, origANE
		lastCPUMetrics = origMetrics
	})

	unifiedComputeHistoryChart = NewPeakStepChart()
	cpuUsageHistory = []float64{10, 20}
	gpuEffectiveHistory = []float64{30, 40}
	aneUsageHistory = []float64{50, 60}
	lastCPUMetrics = CPUMetrics{AvgUsage: 20}
	UpdateCachedTerminalDimensions(9, 20) // (9 * 2 / 3) - 4 == 2 samples

	renderUnifiedComputeHistory()
	got := unifiedComputeHistoryChart.Data
	if !slices.Equal(got[0], aneUsageHistory) || !slices.Equal(got[1], gpuEffectiveHistory) || !slices.Equal(got[2], cpuUsageHistory) {
		t.Fatalf("compute draw order = %v, want ANE/GPU/CPU", got)
	}
}

func TestPeakStepChartDrawsPeakBesideHighestSample(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Border = false
	chart.ShowAxes = false
	chart.ShowRightAxis = true
	chart.SetRect(0, 0, 20, 8)
	chart.MaxVal = 10
	chart.Data = [][]float64{{1, 3, 9, 2, 1}}
	chart.DataLabels = []string{"1"}
	chart.PeakLabels = []string{"CPU 9"}
	chart.ShowPeakLabels = true

	buf := ui.NewBuffer(image.Rect(0, 0, 20, 8))
	chart.Draw(buf)

	peakFound := false
	for y := 0; y < 4; y++ {
		for x := 2; x < 8; x++ {
			if buf.GetCell(image.Pt(x, y)).Rune == 'C' {
				peakFound = true
			}
		}
	}
	if !peakFound {
		t.Fatal("peak label was not drawn beside the highest non-terminal sample")
	}
	if got := buf.GetCell(image.Pt(19, 6)).Rune; got == 'C' {
		t.Fatal("peak label was drawn at the right edge instead of at its sample")
	}
}

func TestUnifiedCPUCorePanelUsesCoreTypeGroups(t *testing.T) {
	w := &CPUCoreWidget{cores: make([]float64, 15), eCoreCount: 0, pCoreCount: 10, sCoreCount: 5}
	if w.groupByType {
		t.Fatal("core grouping must remain opt-in before the unified layout is applied")
	}
	if got, want := FormatCoreSummary(w.eCoreCount, w.pCoreCount, w.sCoreCount), "(10P/5S)"; got != want {
		t.Fatalf("core summary = %q, want %q", got, want)
	}
	w.groupByType = true
	if !w.groupByType {
		t.Fatal("unified CPU core panel must enable P/S grouping")
	}
	groups := w.coreGroups()
	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if got, want := groups[0], (cpuCoreGroup{label: "P 10", start: 0, end: 10}); got != want {
		t.Fatalf("P-core group = %+v, want %+v", got, want)
	}
	if got, want := groups[1], (cpuCoreGroup{label: "S 5", start: 10, end: 15}); got != want {
		t.Fatalf("S-core group = %+v, want %+v", got, want)
	}
}

func TestFormatUnifiedProcessRowUsesCompactColumns(t *testing.T) {
	process := ProcessMetrics{
		PID:       42,
		CPU:       12.34,
		GPU:       25.0, // ms/s, displayed as 2.5%
		RSS:       1536 * 1024,
		Footprint: 2048 * 1024,
		Command:   "long-running-command",
	}

	if got, want := formatUnifiedProcessRow(process, 35), "    42  14.8%  1.5G  2.0G long-r..."; got != want {
		t.Fatalf("compact process row = %q, want %q", got, want)
	}
	if got := runewidth.StringWidth(formatUnifiedProcessRow(process, 12)); got > 12 {
		t.Fatalf("narrow compact process row width = %d, want <= 12", got)
	}
}

func TestUnifiedProcessListUsesValidUnselectedRow(t *testing.T) {
	list := w.NewList()
	list.TextStyle = ui.NewStyle(ui.ColorGreen)
	list.SelectedStyle = list.TextStyle
	list.SelectedRow = 0
	list.Rows = []string{"process"}
	list.SetRect(0, 0, 20, 4)

	buf := ui.NewBuffer(image.Rect(0, 0, 20, 4))
	list.Draw(buf)
}

func TestUnifiedProcessHeaderShowsSelectedSortColumn(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})

	unifiedProcessSelectedColumn = 1
	unifiedProcessSortReverse = false
	if got := formatUnifiedProcessHeader(40); !strings.Contains(got, "C+GPU↓") {
		t.Fatalf("header %q does not show selected descending column", got)
	}
	unifiedProcessSortReverse = true
	if got := formatUnifiedProcessHeader(40); !strings.Contains(got, "C+GPU↑") {
		t.Fatalf("header %q does not show reversed column", got)
	}
}

func TestUnifiedProcessHeaderAlignsWithDataColumns(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})
	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 1, false

	if got, want := formatUnifiedProcessHeader(40), "   PID C+GPU↓   RSS  FOOT CMD"; got != want {
		t.Fatalf("aligned header = %q, want %q", got, want)
	}
	if got, want := formatUnifiedProcessRow(ProcessMetrics{PID: 42, CPU: 12.3, RSS: 1536 * 1024, Footprint: 2048 * 1024, Command: "cmd"}, 40), "    42  12.3%  1.5G  2.0G cmd"; got != want {
		t.Fatalf("aligned row = %q, want %q", got, want)
	}
}

func TestSortUnifiedProcessesUsesCombinedComputeAndRSS(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})
	processes := []ProcessMetrics{
		{PID: 1, CPU: 20, GPU: 0, RSS: 3 * 1024 * 1024},
		{PID: 2, CPU: 10, GPU: 200, RSS: 1 * 1024 * 1024},
	}

	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 1, false
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 2 {
		t.Fatalf("combined compute first PID = %d, want 2", got)
	}
	unifiedProcessSelectedColumn = 2
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 1 {
		t.Fatalf("RSS first PID = %d, want 1", got)
	}
}

func TestSortUnifiedProcessesUsesFootprint(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})
	processes := []ProcessMetrics{
		{PID: 1, Footprint: 1 * 1024 * 1024},
		{PID: 2, Footprint: 3 * 1024 * 1024},
	}

	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 3, false
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 2 {
		t.Fatalf("footprint first PID = %d, want 2", got)
	}
}

func TestNormalizeSocMetricsPower(t *testing.T) {
	withResidual := normalizeSocMetricsPower(SocMetrics{
		TotalPower:  10,
		SystemPower: 14,
	})
	if withResidual.TotalPower != 14 {
		t.Fatalf("expected package power 14, got %.2f", withResidual.TotalPower)
	}
	if withResidual.SystemPower != 4 {
		t.Fatalf("expected residual system power 4, got %.2f", withResidual.SystemPower)
	}

	componentOnly := normalizeSocMetricsPower(SocMetrics{
		TotalPower:  10,
		SystemPower: 7,
	})
	if componentOnly.TotalPower != 10 {
		t.Fatalf("expected package power to fall back to component sum 10, got %.2f", componentOnly.TotalPower)
	}
	if componentOnly.SystemPower != 0 {
		t.Fatalf("expected no residual system power, got %.2f", componentOnly.SystemPower)
	}
}

func TestPrometheusCoreAveragesAndTypes(t *testing.T) {
	info := SystemInfo{CoreCount: 6, ECoreCount: 2, PCoreCount: 3, SCoreCount: 1}
	eAvg, pAvg, sAvg := calculateCoreAveragesForSystem([]float64{10, 30, 40, 50, 60, 80}, info)
	if eAvg != 20 {
		t.Fatalf("expected E-core average 20, got %.2f", eAvg)
	}
	if pAvg != 50 {
		t.Fatalf("expected P-core average 50, got %.2f", pAvg)
	}
	if sAvg != 80 {
		t.Fatalf("expected S-core average 80, got %.2f", sAvg)
	}

	expectedTypes := []string{"e", "e", "p", "p", "p", "s"}
	for i, want := range expectedTypes {
		if got := coreTypeForIndex(i, info); got != want {
			t.Fatalf("core %d type = %s, want %s", i, got, want)
		}
	}
}

func TestNewCPUCoreWidget(t *testing.T) {
	info := SystemInfo{
		Name:       "Apple M1",
		CoreCount:  8,
		ECoreCount: 4,
		PCoreCount: 4,
	}
	w := NewCPUCoreWidget(info)

	if w.modelName != "Apple M1" {
		t.Errorf("Expected modelName 'Apple M1', got %s", w.modelName)
	}

	totalFromWidget := w.eCoreCount + w.pCoreCount + w.sCoreCount
	if totalFromWidget == 0 {
		t.Error("Expected non-zero core counts")
	}
	if len(w.cores) != totalFromWidget {
		t.Errorf("Expected len(cores) %d to match eCoreCount+pCoreCount+sCoreCount, got %d", totalFromWidget, len(w.cores))
	}
}

func TestEventThrottler(t *testing.T) {
	throttler := NewEventThrottler(50 * time.Millisecond)

	// First notification should trigger after delay
	start := time.Now()
	throttler.Notify()

	select {
	case <-throttler.C:
		elapsed := time.Since(start)
		if elapsed < 50*time.Millisecond {
			t.Errorf("Throttler fired too early: %v", elapsed)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Throttler failed to fire")
	}

	// Multiple notifications should be coalesced
	start = time.Now()
	throttler.Notify()
	throttler.Notify()
	throttler.Notify()

	select {
	case <-throttler.C:
		elapsed := time.Since(start)
		if elapsed < 50*time.Millisecond {
			t.Errorf("Throttler fired too early: %v", elapsed)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Throttler failed to fire")
	}

	// Ensure no extra events are pending
	select {
	case <-throttler.C:
		t.Error("Throttler fired extra event")
	default:
		// OK
	}
}

func BenchmarkGetGPUProcessStats(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = GetGPUProcessStats()
	}
}

func TestGetCachedTerminalDimensions(t *testing.T) {
	UpdateCachedTerminalDimensions(0, 0)

	w, h := GetCachedTerminalDimensions()
	if w == 0 || h == 0 {
		t.Skip("Terminal dimensions unavailable, skipping test")
	}

	UpdateCachedTerminalDimensions(120, 40)

	w2, h2 := GetCachedTerminalDimensions()
	if w2 != 120 {
		t.Errorf("Expected cached width 120, got %d", w2)
	}
	if h2 != 40 {
		t.Errorf("Expected cached height 40, got %d", h2)
	}

	UpdateCachedTerminalDimensions(80, 24)
	w3, h3 := GetCachedTerminalDimensions()
	if w3 != 80 || h3 != 24 {
		t.Errorf("Expected 80x24 after update, got %dx%d", w3, h3)
	}
}

func TestSafeFloat64At(t *testing.T) {
	tests := []struct {
		name   string
		slice  []float64
		index  int
		expect float64
	}{
		{"Valid index 0", []float64{1.0, 2.0, 3.0}, 0, 1.0},
		{"Valid index 2", []float64{1.0, 2.0, 3.0}, 2, 3.0},
		{"Index out of bounds", []float64{1.0, 2.0}, 5, 0.0},
		{"Negative index", []float64{1.0, 2.0}, -1, 0.0},
		{"Empty slice", []float64{}, 0, 0.0},
		{"Nil slice", nil, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeFloat64At(tt.slice, tt.index)
			if got != tt.expect {
				t.Errorf("safeFloat64At() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func resetANETestState(t *testing.T) {
	t.Helper()
	origMax, origLatch, origResidency := maxANEBWSeenBits.Load(), aneBWModeLatched.Load(), aneResidencyLatched.Load()
	t.Cleanup(func() {
		maxANEBWSeenBits.Store(origMax)
		aneBWModeLatched.Store(origLatch)
		aneResidencyLatched.Store(origResidency)
	})
	maxANEBWSeenBits.Store(0)
	aneBWModeLatched.Store(false)
	aneResidencyLatched.Store(false)
}

func TestANEUtilizationAndLabelMode(t *testing.T) {
	resetANETestState(t)

	// 1. Power path (macOS 26 normal): watts present -> W/8 estimate, W-label.
	m := CPUMetrics{ANEW: 2.0}
	if got := aneUtilizationPercent(m); got != 25.0 {
		t.Fatalf("power path: got %v, want 25", got)
	}
	if aneBWLabelMode(m) {
		t.Fatal("power path must use watts label")
	}
	if aneBWModeLatched.Load() {
		t.Fatal("power path must not latch BW mode")
	}

	// 2. Idle with working power counter (macOS 26 idle): stays in W form.
	idle := CPUMetrics{}
	if aneBWLabelMode(idle) {
		t.Fatal("fresh idle must keep watts label (26-compatible)")
	}

	// 3. Dead energy counter, traffic flowing (macOS 27): BW estimate + latch.
	bw := CPUMetrics{ANEBW: 2.0}
	if got := aneUtilizationPercent(bw); got != 50.0 { // ref floor 4.0
		t.Fatalf("bw path: got %v, want 50", got)
	}
	if !aneBWLabelMode(bw) {
		t.Fatal("bw path must use GB/s label")
	}

	// 4. ANE goes idle afterwards: label stays in GB/s form (latched).
	if !aneBWLabelMode(idle) {
		t.Fatal("after latch, idle must keep GB/s label")
	}

	// 5. Adaptive reference: higher BW raises the 100% reference.
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 8.0}); got != 100.0 {
		t.Fatalf("saturation: got %v, want 100", got)
	}
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 4.0}); got != 50.0 {
		t.Fatalf("post-adapt: got %v, want 50 (ref=8)", got)
	}

	// 6. Power returning (Apple fixes the counters) wins immediately.
	if aneBWLabelMode(CPUMetrics{ANEW: 0.5, ANEBW: 3.0}) {
		t.Fatal("working watts must take precedence over latch")
	}

}

// TestANEVisibleSeries verifies the history-line source selection across the
// three tiers, in particular that the M5 residency path is NOT re-derived from
// bandwidth (the regression this guards).
func TestANEVisibleSeries(t *testing.T) {
	resetANETestState(t)

	// Save and restore the shared history buffers we mutate.
	origUsage := append([]float64(nil), aneUsageHistory...)
	origRd := append([]float64(nil), aneReadBwHistory...)
	origWr := append([]float64(nil), aneWriteBwHistory...)
	t.Cleanup(func() {
		copy(aneUsageHistory, origUsage)
		copy(aneReadBwHistory, origRd)
		copy(aneWriteBwHistory, origWr)
	})

	n := len(aneUsageHistory)
	for i := range aneUsageHistory {
		aneUsageHistory[i] = 30 // stored utilization (residency or power)
		aneReadBwHistory[i] = 6 // 6+2 = 8 GB/s combined
		aneWriteBwHistory[i] = 2
	}
	maxANEBWSeenBits.Store(math.Float64bits(16)) // ref = 16 -> 8/16 = 50%

	// Tier 2 (macOS 26, watts): not bwMode -> plot stored 30%.
	if got := aneVisibleSeries(n, false); got[n-1] != 30 {
		t.Fatalf("watts tier: plotted %v, want stored 30", got[n-1])
	}

	// Tier 3 (M1-M4 macOS 27): bwMode, no residency -> re-derive 8/16 = 50%.
	if got := aneVisibleSeries(n, true); got[n-1] != 50 {
		t.Fatalf("bandwidth tier: plotted %v, want derived 50", got[n-1])
	}

	// Tier 1 (M5 macOS 27): bwMode AND residency latched -> plot stored 30%,
	// NOT the 50% bandwidth derivation (must match the residency gauge).
	aneResidencyLatched.Store(true)
	if got := aneVisibleSeries(n, true); got[n-1] != 30 {
		t.Fatalf("residency tier: plotted %v, want stored 30 (gauge consistency)", got[n-1])
	}
}

// TestHistoryLineColor verifies the per-tick history-chart color resolution
// honors a custom theme (so live updates don't clobber applyCustomWidgetColors)
// and falls back to the default when no custom theme is set.
func TestHistoryLineColor(t *testing.T) {
	origConfig := currentConfig
	t.Cleanup(func() { currentConfig = origConfig })
	pickCPU := func(tc *CustomThemeConfig) string { return tc.CPU }

	// No custom theme -> the hardcoded fallback.
	currentConfig.CustomTheme = nil
	if got := historyLineColor(pickCPU, ui.ColorGreen); got != ui.ColorGreen {
		t.Fatalf("nil theme: got %v, want fallback ColorGreen", got)
	}

	// Custom theme with an explicit hex -> that color, NOT the fallback.
	currentConfig.CustomTheme = &CustomThemeConfig{CPU: "#FF0000"}
	want, err := ParseHexColor("#FF0000")
	if err != nil {
		t.Fatalf("ParseHexColor: %v", err)
	}
	if got := historyLineColor(pickCPU, ui.ColorGreen); got != want {
		t.Fatalf("custom hex: got %v, want %v (custom color must survive live updates)", got, want)
	}

	// Custom theme present but this component unset -> the theme foreground.
	currentConfig.CustomTheme = &CustomThemeConfig{}
	fg := GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode)
	if got := historyLineColor(pickCPU, ui.ColorGreen); got != fg {
		t.Fatalf("empty component: got %v, want theme fg %v", got, fg)
	}
}

func TestANERefHysteresis(t *testing.T) {
	resetANETestState(t)

	// Establish a reference above the floor.
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 8.0}); got != 100.0 {
		t.Fatalf("establish ref: got %v, want 100", got)
	}

	// A burst within 3% of the reference reads 100% (clamp) but must NOT
	// ratchet — sustained saturation would otherwise read 96-98% forever
	// against its own burst noise.
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 8.2}); got != 100.0 {
		t.Fatalf("sub-hysteresis burst: got %v, want 100", got)
	}
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 4.0}); got != 50.0 {
		t.Fatalf("ref must still be 8 after sub-hysteresis burst: got %v, want 50", got)
	}

	// A genuine step-up beyond 3% still re-scales the reference.
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 8.5}); got != 100.0 {
		t.Fatalf("step-up: got %v, want 100", got)
	}
	if got := aneUtilizationPercent(CPUMetrics{ANEBW: 4.25}); got != 50.0 {
		t.Fatalf("post-step ref must be 8.5: got %v, want 50", got)
	}
}

// TestANEResidencyTier covers the PMP state-residency utilization source
// (macOS 27/M5), which outranks both estimates.
func TestANEResidencyTier(t *testing.T) {
	resetANETestState(t)

	// Residency with dead watts: preferred for the percent, latches GB/s label.
	res := CPUMetrics{ANEActive: 62.5, ANEBW: 9.0}
	if got := aneUtilizationPercent(res); got != 62.5 {
		t.Fatalf("residency tier: got %v, want 62.5", got)
	}
	if !aneBWModeLatched.Load() || !aneBWLabelMode(res) {
		t.Fatal("residency with dead watts must latch GB/s label")
	}
	// The residency latch keeps the history chart on stored residency
	// percentages instead of re-deriving them from bandwidth (M5 path).
	if !aneResidencyLatched.Load() {
		t.Fatal("residency tier must latch the residency flag")
	}

	// Residency WITH working watts: residency still preferred for the percent,
	// but the label stays in watts form (no latch).
	resetANETestState(t)
	both := CPUMetrics{ANEActive: 40, ANEW: 1.0}
	if got := aneUtilizationPercent(both); got != 40 {
		t.Fatalf("residency+power: got %v, want 40", got)
	}
	if aneBWModeLatched.Load() || aneBWLabelMode(both) {
		t.Fatal("working watts must keep the wattage label even with residency")
	}
}
