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
	"github.com/metaspartan/mactop/v2/internal/i18n"
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

func TestFormatRoundedBytes(t *testing.T) {
	if got, want := formatRoundedBytes(1536, "kb"), "2KB"; got != want {
		t.Fatalf("rounded bytes = %q, want %q", got, want)
	}
}

func TestUpdateMemoryHistorySeedsFirstSample(t *testing.T) {
	origMemory, origSwap, origSeeded, origChart := memoryUsedHistory, swapUsedHistory, memoryHistorySeeded, memoryHistoryChart
	t.Cleanup(func() {
		memoryUsedHistory, swapUsedHistory, memoryHistorySeeded, memoryHistoryChart = origMemory, origSwap, origSeeded, origChart
	})

	memoryUsedHistory, swapUsedHistory = make([]float64, 4), make([]float64, 4)
	memoryHistorySeeded = false
	memoryHistoryChart = nil
	updateMemoryHistory(MemoryMetrics{Used: 8 * 1024 * 1024 * 1024, SwapUsed: 2 * 1024 * 1024 * 1024, Total: 16 * 1024 * 1024 * 1024})
	if !memoryHistorySeeded {
		t.Fatal("first memory sample must seed the history")
	}
	if got, want := memoryUsedHistory, []float64{8, 8, 8, 8}; !slices.Equal(got, want) {
		t.Fatalf("seeded memory history = %v, want %v", got, want)
	}
	if got, want := swapUsedHistory, []float64{2, 2, 2, 2}; !slices.Equal(got, want) {
		t.Fatalf("seeded swap history = %v, want %v", got, want)
	}

	updateMemoryHistory(MemoryMetrics{Used: 9 * 1024 * 1024 * 1024, SwapUsed: 3 * 1024 * 1024 * 1024, Total: 16 * 1024 * 1024 * 1024})
	if got, want := memoryUsedHistory, []float64{8, 8, 8, 9}; !slices.Equal(got, want) {
		t.Fatalf("shifted memory history = %v, want %v", got, want)
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

func TestUnifiedComputeHistoryTitleOmitsTemperatures(t *testing.T) {
	if got := unifiedComputeHistoryTitle(); strings.Contains(got, "°") {
		t.Fatalf("title %q must not contain temperature values", got)
	}
}

func TestUnifiedHistoryTitleAddsVisiblePeaks(t *testing.T) {
	if got, want := unifiedHistoryTitle("Compute", []string{"ANE: 30%", "GPU: 50%", "CPU: 70%"}), "Compute (ANE: 30%, GPU: 50%, CPU: 70%)"; got != want {
		t.Fatalf("history title = %q, want %q", got, want)
	}
}

func TestUnifiedTitleSpansDifferentiateValuesUnitsAndCapacity(t *testing.T) {
	spans := unifiedTitleSpans("Memory: 12/32GB, Link: 2.5/5GbE", []titleMetricColor{
		{Name: "Memory", Color: ui.ColorMagenta},
		{Name: "Link", Color: ui.ColorYellow},
	}, ui.ColorBlack)

	find := func(text string) TitleSpan {
		for _, span := range spans {
			if span.Text == text {
				return span
			}
		}
		t.Fatalf("missing title span %q in %#v", text, spans)
		return TitleSpan{}
	}
	if span := find("12"); span.Style.Fg != ui.ColorMagenta || span.Style.Modifier != ui.ModifierBold {
		t.Fatalf("current value style = %+v, want bold magenta", span.Style)
	}
	if span := find("32"); span.Style.Fg != ui.ColorMagenta || span.Style.Modifier != ui.ModifierClear {
		t.Fatalf("capacity style = %+v, want normal magenta", span.Style)
	}
	if span := find("GB, Link: "); span.Style.Fg != ui.ColorGrey || span.Style.Modifier != ui.ModifierClear {
		t.Fatalf("unit/label style = %+v, want muted", span.Style)
	}
	if span := find("2.5"); span.Style.Fg != ui.ColorYellow || span.Style.Modifier != ui.ModifierBold {
		t.Fatalf("decimal Link style = %+v, want bold yellow", span.Style)
	}
	if span := find("5"); span.Style.Fg != ui.ColorYellow || span.Style.Modifier != ui.ModifierClear {
		t.Fatalf("Link capacity style = %+v, want normal yellow", span.Style)
	}
}

func TestMemoryTitleMetricColorsStyleCompactLabels(t *testing.T) {
	spans := unifiedTitleSpans("Memory/Swap (M: 12GB, S: 3GB, R: 10GB/s, W: 8GB/s)", memoryTitleMetricColors(), ui.ColorBlack)
	styles := make(map[string]ui.Style)
	for _, span := range spans {
		styles[span.Text] = span.Style
	}
	for value, color := range map[string]ui.Color{
		"12": ui.ColorMagenta,
		"3":  ui.ColorOrange,
		"10": ui.ColorCyan,
		"8":  ui.ColorRed,
	} {
		style, ok := styles[value]
		if !ok || style.Fg != color || style.Modifier != ui.ModifierBold {
			t.Fatalf("compact memory value %q style = %+v, want bold color %v", value, style, color)
		}
	}
}

func TestPeakStepChartDrawsTitleSpansWithoutOverwritingBorder(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Title = "Memory: 12/32GB"
	chart.TitleSpans = unifiedTitleSpans(chart.Title, []titleMetricColor{{Name: "Memory", Color: ui.ColorMagenta}}, chart.TitleStyle.Bg)
	chart.Data = [][]float64{{1, 2}}
	chart.MaxVal = 2
	chart.SetRect(0, 0, 24, 5)

	buf := ui.NewBuffer(image.Rect(0, 0, 24, 5))
	chart.Draw(buf)
	if cell := buf.GetCell(image.Pt(10, 0)); cell.Rune != '1' || cell.Style.Fg != ui.ColorMagenta || cell.Style.Modifier != ui.ModifierBold {
		t.Fatalf("current title cell = %+v, want bold magenta 1", cell)
	}
	if cell := buf.GetCell(image.Pt(13, 0)); cell.Rune != '3' || cell.Style.Fg != ui.ColorMagenta || cell.Style.Modifier != ui.ModifierClear {
		t.Fatalf("capacity title cell = %+v, want normal magenta 3", cell)
	}
	if cell := buf.GetCell(image.Pt(15, 0)); cell.Rune != 'G' || cell.Style.Fg != ui.ColorGrey {
		t.Fatalf("unit title cell = %+v, want muted G", cell)
	}
	if cell := buf.GetCell(image.Pt(23, 0)); cell.Rune == ' ' {
		t.Fatalf("right border was overwritten: %+v", cell)
	}
}

func TestSetUnifiedTitleSpansClearsAfterLayoutChange(t *testing.T) {
	origLayout := currentConfig.DefaultLayout
	t.Cleanup(func() { currentConfig.DefaultLayout = origLayout })
	chart := NewPeakStepChart()
	chart.Title = "CPU: 80%"
	currentConfig.DefaultLayout = LayoutUnified
	setUnifiedTitleSpans(chart, titleMetricColor{Name: "CPU", Color: ui.ColorRed})
	if len(chart.TitleSpans) == 0 {
		t.Fatal("unified chart must receive styled title spans")
	}
	currentConfig.DefaultLayout = LayoutHistorySoC
	setUnifiedTitleSpans(chart, titleMetricColor{Name: "CPU", Color: ui.ColorRed})
	if chart.TitleSpans != nil {
		t.Fatalf("non-unified chart retained title spans: %#v", chart.TitleSpans)
	}
}

func TestFormatTitleRoundsNumericValuesButPreservesLinkText(t *testing.T) {
	origLayout := currentConfig.DefaultLayout
	t.Cleanup(func() { currentConfig.DefaultLayout = origLayout })
	currentConfig.DefaultLayout = LayoutUnified
	if got, want := formatTitle("Power %.2fW, utilization %.1f%%", 12.6, 48.6), "Power 13W, utilization 49%"; got != want {
		t.Fatalf("integer title = %q, want %q", got, want)
	}
	if got, want := formatTitle("Link: %s", "2.5Gbps"), "Link: 2.5Gbps"; got != want {
		t.Fatalf("Link title = %q, want %q", got, want)
	}
}

func TestFormatTitlePreservesNonUnifiedPrecision(t *testing.T) {
	origLayout := currentConfig.DefaultLayout
	t.Cleanup(func() { currentConfig.DefaultLayout = origLayout })
	currentConfig.DefaultLayout = LayoutHistorySoC
	if got, want := formatTitle("Power %.1fW", 12.6), "Power 12.6W"; got != want {
		t.Fatalf("non-unified title = %q, want %q", got, want)
	}
}

func TestFormatUnifiedPowerTitleUsesAdapterLimitWhenAvailable(t *testing.T) {
	if got, want := formatUnifiedPowerTitle("SoC Power", 56.6, 67), "SoC Power | Total: 57/67W"; got != want {
		t.Fatalf("power title with adapter = %q, want %q", got, want)
	}
	if got, want := formatUnifiedPowerTitle("SoC Power", 56.6, 0), "SoC Power | Total: 57W"; got != want {
		t.Fatalf("power title without adapter = %q, want %q", got, want)
	}
}

func TestFormatCPUFreqRetainsSingleDecimal(t *testing.T) {
	origE, origP, origS := lastEFreq, lastPFreq, lastSFreq
	t.Cleanup(func() { lastEFreq, lastPFreq, lastSFreq = origE, origP, origS })
	lastEFreq, lastPFreq, lastSFreq = 0, 0, 0
	if got := formatCPUFreq(CPUMetrics{EClusterFreqMHz: 3200}); !strings.Contains(got, "E3.2 GHz") {
		t.Fatalf("single-core frequency title = %q, want one decimal", got)
	}
}

func TestUnifiedMemoryHistoryTitleIsCompactOnNarrowTerminals(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() { UpdateCachedTerminalDimensions(origWidth, origHeight) })
	peaks := []string{"S 3GB", "M 12GB", "R 10GB/s", "W 8GB/s"}

	UpdateCachedTerminalDimensions(120, 30)
	if got, want := unifiedMemoryHistoryTitle(peaks), memorySwapTitle()+" (Memory: 12GB, Swap: 3GB, DRAM Read: 10GB/s, DRAM Write: 8GB/s)"; got != want {
		t.Fatalf("wide memory title = %q, want %q", got, want)
	}

	UpdateCachedTerminalDimensions(99, 30)
	if got, want := unifiedMemoryHistoryTitle(peaks), memorySwapTitle()+" (M: 12GB, S: 3GB, R: 10GB/s, W: 8GB/s)"; got != want {
		t.Fatalf("narrow memory title = %q, want compact %q", got, want)
	}
}

func TestUnifiedMemoryHistoryTitleShowsCapacitiesOnAllTerminalWidths(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() { UpdateCachedTerminalDimensions(origWidth, origHeight) })
	peaks := []string{"S 3GB", "M 12GB"}

	UpdateCachedTerminalDimensions(120, 30)
	if got, want := unifiedMemoryHistoryTitleWithCapacity(peaks, 32, 16), memorySwapTitle()+" (Memory: 12/32GB, Swap: 3/16GB)"; got != want {
		t.Fatalf("wide memory title = %q, want %q", got, want)
	}

	UpdateCachedTerminalDimensions(99, 30)
	if got, want := unifiedMemoryHistoryTitleWithCapacity(peaks, 32, 16), memorySwapTitle()+" (M: 12/32GB, S: 3/16GB)"; got != want {
		t.Fatalf("narrow memory title = %q, want %q", got, want)
	}
}

func TestCPUCoreWidgetTitleOmitsTemperature(t *testing.T) {
	origLayout := currentConfig.DefaultLayout
	t.Cleanup(func() { currentConfig.DefaultLayout = origLayout })
	currentConfig.DefaultLayout = LayoutUnified
	title := formatCPUCoreWidgetTitle(12, "(8P/4E)", 37.5, " @ P3.2 GHz")
	if strings.Contains(title, "°") || !strings.Contains(title, "38%") {
		t.Fatalf("CPU core title = %q; want utilization without temperature", title)
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

func TestHistoricalPeakNeverDeclines(t *testing.T) {
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)

	for _, metric := range []string{
		"cpu-usage", "gpu-raw", "gpu-effective", "ane-usage",
		"memory-used", "memory-swap", "dram-read", "dram-write", "dram-total",
		"ane-read-bandwidth", "ane-write-bandwidth", "bandwidth-peak",
		"power-total", "power-cpu", "power-gpu", "power-ane", "power-dram",
		"temperature-C", "temperature-G", "temperature-M", "temperature-S",
		"fan-rpm-0", "fan-rpm-1", "network-download", "network-upload", "network-link",
		"disk-read", "disk-write", "disk-used", "ssd-read",
	} {
		recordHistoricalPeak(metric, 100)
		recordHistoricalPeak(metric, 10)
		if got := historicalPeak(metric, 5); got != 100 {
			t.Fatalf("%s peak = %.1f, want 100.0", metric, got)
		}
	}
}

func TestPerSecondCounterDeltaHandlesCounterReset(t *testing.T) {
	if got, want := perSecondCounterDelta(150, 100, 2), 25.0; got != want {
		t.Fatalf("normal counter delta = %.1f, want %.1f", got, want)
	}
	if got := perSecondCounterDelta(10, 100, 1); got != 0 {
		t.Fatalf("counter reset delta = %.1f, want 0", got)
	}
	if got := perSecondCounterDelta(100, 100, 0); got != 0 {
		t.Fatalf("zero elapsed delta = %.1f, want 0", got)
	}
}

func TestUnifiedProcessSummaryKeepsLifetimePeak(t *testing.T) {
	origSummary, origPeak := latestUnifiedProcessSummary, processCountPeak
	t.Cleanup(func() {
		latestUnifiedProcessSummary, processCountPeak = origSummary, origPeak
	})

	processCountPeak = 0
	updateUnifiedProcessSummary([]ProcessMetrics{{}, {}, {}})
	updateUnifiedProcessSummary([]ProcessMetrics{{}})
	if got, want := latestUnifiedProcessSummary, (unifiedProcessSummary{Total: 1, PeakTotal: 3}); got != want {
		t.Fatalf("process summary = %+v, want %+v", got, want)
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
	UpdateCachedTerminalDimensions(6, 10)
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)
	memoryHistoryChart = NewPeakStepChart()
	memoryUsedHistory = []float64{80, 20, 40}
	swapUsedHistory = []float64{8, 2, 4}
	dramReadHistory = []float64{20, 4, 8}
	dramWriteHistory = []float64{24, 6, 12}
	recordHistoricalPeak("memory-used", 99)
	recordHistoricalPeak("memory-swap", 19)
	recordHistoricalPeak("dram-read", 29)
	recordHistoricalPeak("dram-write", 39)

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
	if got, want := memoryHistoryChart.currentLabelOrder(), []int{2, 3, 0, 1}; !slices.Equal(got, want) {
		t.Fatalf("mixed current label order = %v, want bandwidth before capacity %v", got, want)
	}
	if got, want := memoryHistoryChart.PeakLabels, []string{"S 4GB", "M 40GB", "R 8GB/s", "W 12GB/s"}; !slices.Equal(got, want) {
		t.Fatalf("mixed peak labels = %v, want %v", got, want)
	}
	if got, want := memoryHistoryChart.LineColors[:2], []ui.Color{ui.ColorOrange, ui.ColorMagenta}; !slices.Equal(got, want) {
		t.Fatalf("memory draw order colors = %v, want swap then memory %v", got, want)
	}
	if got, want := memoryHistoryChart.Title, memorySwapTitle()+" (M: 99GB, S: 19GB, R: 29GB/s, W: 39GB/s)"; got != want {
		t.Fatalf("narrow memory chart title = %q, want compact peaks %q", got, want)
	}
}

func TestRenderUnifiedMemoryDRAMHistoryUsesOneEstimatedTotalSeries(t *testing.T) {
	origConfig := currentConfig
	origWidth, origHeight := GetCachedTerminalDimensions()
	origChart := memoryHistoryChart
	origMemory, origSwap := memoryUsedHistory, swapUsedHistory
	origRead, origWrite, origTotal := dramReadHistory, dramWriteHistory, dramTotalHistory
	t.Cleanup(func() {
		currentConfig = origConfig
		UpdateCachedTerminalDimensions(origWidth, origHeight)
		memoryHistoryChart = origChart
		memoryUsedHistory, swapUsedHistory = origMemory, origSwap
		dramReadHistory, dramWriteHistory, dramTotalHistory = origRead, origWrite, origTotal
	})

	currentConfig.DefaultLayout = LayoutUnified
	UpdateCachedTerminalDimensions(20, 10)
	memoryHistoryChart = NewPeakStepChart()
	memoryHistoryChart.MaxVal = 64 // Simulate the physical-GB scale left by updateMemoryHistory.
	memoryUsedHistory = []float64{10, 20, 40}
	swapUsedHistory = []float64{1, 2, 4}
	dramReadHistory = []float64{2, 4, 8}
	dramWriteHistory = []float64{2, 4, 8}
	dramTotalHistory = []float64{4, 8, 16}

	renderUnifiedMemoryDRAMHistory(
		MemoryMetrics{Used: 40 * 1024 * 1024 * 1024, SwapUsed: 4 * 1024 * 1024 * 1024},
		CPUMetrics{DRAMReadBW: 8, DRAMWriteBW: 8, DRAMBWCombined: 16, DRAMBandwidthSource: DRAMBandwidthPowerEstimate},
	)

	if got, want := len(memoryHistoryChart.Data), 3; got != want {
		t.Fatalf("estimated series count = %d, want memory/swap/total %d", got, want)
	}
	if got, want := memoryHistoryChart.MaxVal, 100.0; got != want {
		t.Fatalf("estimated chart max = %.1f, want normalized %.1f", got, want)
	}
	if got, want := memoryHistoryChart.DataLabels[2], "D ~16.0GB/s"; got != want {
		t.Fatalf("estimated label = %q, want %q", got, want)
	}
	if got, want := memoryHistoryChart.currentLabelOrder(), []int{2, 0, 1}; !slices.Equal(got, want) {
		t.Fatalf("estimated current label order = %v, want bandwidth before capacity %v", got, want)
	}
	if strings.Contains(strings.Join(memoryHistoryChart.DataLabels, " "), "R ") || strings.Contains(strings.Join(memoryHistoryChart.DataLabels, " "), "W ") {
		t.Fatalf("estimated labels must not fabricate directions: %v", memoryHistoryChart.DataLabels)
	}
}

func TestPeakStepChartUsesConfiguredCurrentLabelOrder(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Data = [][]float64{{1}, {2}, {3}}
	chart.CurrentLabelOrder = []int{1, 0, 2}
	chart.SeriesGroups = []string{"first", "second", "third"}
	if got, want := chart.currentLabelOrder(), []int{1, 0, 2}; !slices.Equal(got, want) {
		t.Fatalf("current label order = %v, want %v", got, want)
	}
}

func TestPeakStepChartOrdersSameUnitCurrentLabelsHighToLow(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Data = [][]float64{{1.4}, {0}, {2.0}}
	chart.CurrentLabelOrder = []int{1, 0, 2}
	chart.SeriesGroups = []string{"power", "power", "power"}
	if got, want := chart.currentLabelOrder(), []int{1, 0, 2}; !slices.Equal(got, want) {
		t.Fatalf("same-unit placement order = %v, want low-to-high %v", got, want)
	}
}

func TestPeakStepChartKeepsMemoryAboveSwapWhenValuesMatch(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Data = [][]float64{{20}, {20}}
	chart.CurrentLabelOrder = []int{1, 0}
	chart.SeriesGroups = []string{"capacity", "capacity"}
	if got, want := chart.currentLabelOrder(), []int{0, 1}; !slices.Equal(got, want) {
		t.Fatalf("equal memory/swap label order = %v, want swap then memory %v", got, want)
	}
}

func TestPeakStepChartDrawsMemoryAboveSwapWhenValuesMatch(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Border = false
	chart.ShowAxes = false
	chart.ShowRightAxis = true
	chart.SetRect(0, 0, 20, 6)
	chart.MaxVal = 100
	chart.Data = [][]float64{{50}, {50}}
	chart.DataLabels = []string{"S 1.0GB", "M 1.0GB"}
	chart.CurrentLabelOrder = []int{1, 0}
	chart.SeriesGroups = []string{"capacity", "capacity"}

	buf := ui.NewBuffer(image.Rect(0, 0, 20, 6))
	chart.Draw(buf)
	rows := make(map[rune]int)
	for y := 0; y < 6; y++ {
		for x := 0; x < 20; x++ {
			r := buf.GetCell(image.Pt(x, y)).Rune
			if r == 'M' || r == 'S' {
				rows[r] = y
			}
		}
	}
	if rows['M'] >= rows['S'] {
		t.Fatalf("memory/swap rows = %v; memory must be above swap", rows)
	}
}

func TestPeakStepChartKeepsMemoryAndSwapOverDRAMOnShortChart(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Border = false
	chart.ShowAxes = false
	chart.ShowRightAxis = true
	chart.SetRect(0, 0, 20, 4)
	chart.MaxVal = 100
	chart.Data = [][]float64{{50}, {50}, {50}, {50}}
	chart.DataLabels = []string{"S 1.0GB", "M 1.0GB", "R 1.0GB/s", "W 1.0GB/s"}
	chart.CurrentLabelOrder = []int{2, 3, 1, 0}
	chart.SeriesGroups = []string{"capacity", "capacity", "bandwidth", "bandwidth"}

	buf := ui.NewBuffer(image.Rect(0, 0, 20, 4))
	chart.Draw(buf)
	rows := make(map[rune]int)
	for y := 0; y < 4; y++ {
		for x := 0; x < 20; x++ {
			r := buf.GetCell(image.Pt(x, y)).Rune
			if r == 'M' || r == 'S' {
				rows[r] = y
			}
		}
	}
	if len(rows) != 2 || rows['M'] >= rows['S'] {
		t.Fatalf("short-chart memory/swap rows = %v; want M above S", rows)
	}
}

func TestPeakStepChartKeepsDifferentUnitGroupsIndependent(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Data = [][]float64{{1.4}, {0}, {32.5}}
	chart.CurrentLabelOrder = []int{1, 0, 2}
	chart.SeriesGroups = []string{"power", "power", "capacity"}
	if got, want := chart.currentLabelOrder(), []int{1, 0, 2}; !slices.Equal(got, want) {
		t.Fatalf("mixed-unit current order = %v, want %v", got, want)
	}
}

func TestFormatDRAMBandwidthMarksNonDirectionalSources(t *testing.T) {
	if got := formatDRAMBandwidth(CPUMetrics{DRAMBWCombined: 20, DRAMBandwidthSource: DRAMBandwidthPowerEstimate}); got != "~20.0 GB/s (estimate)" {
		t.Fatalf("power estimate = %q", got)
	}
	if got := formatDRAMBandwidth(CPUMetrics{DRAMBandwidthSource: DRAMBandwidthUnavailable}); got != "calibrating" {
		t.Fatalf("unavailable bandwidth = %q", got)
	}
}

func TestUnifiedChartTitleLocalizesBaseAndUsesEnglishLegend(t *testing.T) {
	if got := unifiedChartTitle("TUI_UnifiedNetworkHistory", "D/U"); !strings.HasSuffix(got, " (D/U)") || strings.Contains(got, "下行") {
		t.Fatalf("unified chart title has incorrect legend: %q", got)
	}
	if got := unifiedChartTitle("TUI_Temperatures", ""); strings.Contains(got, "(") {
		t.Fatalf("non-legend title unexpectedly contains legend: %q", got)
	}
}

func TestUnifiedNetworkHistoryTitleUsesCompactLegend(t *testing.T) {
	if got := unifiedNetworkHistoryTitle(); !strings.HasSuffix(got, " (D/U)") {
		t.Fatalf("network title = %q, want compact legend", got)
	}
}

func TestUnifiedMemoryTitleOmitsSwap(t *testing.T) {
	if got, want := memorySwapTitle(), i18n.T("Info_Memory"); got != want {
		t.Fatalf("memory title = %q, want %q", got, want)
	}
}

func TestUnifiedDiskHistoryTitleOmitsIO(t *testing.T) {
	if got := unifiedChartTitle("TUI_UnifiedDiskHistory", "R/W"); strings.Contains(got, "I/O") {
		t.Fatalf("disk title = %q, must omit I/O", got)
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

func TestUnifiedHistoryWidthUsesLeftTwoThirds(t *testing.T) {
	if got, want := unifiedHistoryWidth(120), 76; got != want {
		t.Fatalf("unified history width = %d, want %d", got, want)
	}
}

func TestUnifiedHistoryWidthsExpandWhenSidebarIsHidden(t *testing.T) {
	if unifiedShowsSidebar(99) {
		t.Fatal("sidebar must be hidden below 100 columns")
	}
	if !unifiedShowsSidebar(100) {
		t.Fatal("sidebar must be shown at 100 columns")
	}
	if got, want := unifiedHistoryWidth(99), 95; got != want {
		t.Fatalf("narrow history width = %d, want %d", got, want)
	}
	if got, want := unifiedIOHistoryWidth(99), 95; got != want {
		t.Fatalf("narrow I/O history width = %d, want %d", got, want)
	}
}

func TestUnifiedLayoutPlacesCorrelatedHistoriesInLeftFourRows(t *testing.T) {
	origGrid := grid
	origCompute, origMemory, origPower := unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart
	origNetwork, origDisk := unifiedNetworkHistoryChart, unifiedDiskHistoryChart
	origCores, origTemp, origProcesses := cpuCoreWidget, unifiedTemperatureHistoryChart, unifiedProcessList
	t.Cleanup(func() {
		grid = origGrid
		unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart = origCompute, origMemory, origPower
		unifiedNetworkHistoryChart, unifiedDiskHistoryChart = origNetwork, origDisk
		cpuCoreWidget, unifiedTemperatureHistoryChart, unifiedProcessList = origCores, origTemp, origProcesses
	})

	grid = ui.NewGrid()
	unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart = NewPeakStepChart(), NewPeakStepChart(), NewPeakStepChart()
	unifiedNetworkHistoryChart, unifiedDiskHistoryChart = NewPeakStepChart(), NewPeakStepChart()
	cpuCoreWidget = &CPUCoreWidget{Block: ui.NewBlock()}
	unifiedTemperatureHistoryChart, unifiedProcessList = NewPeakStepChart(), NewStyledList()
	setUnifiedLayoutGridForWidth(120)

	itemFor := func(widget any) *ui.GridItem {
		for _, item := range grid.Items {
			if item.Entry == widget {
				return item
			}
		}
		t.Fatalf("layout item not found for %T", widget)
		return nil
	}

	power := itemFor(socPowerHistoryChart)
	if power.XRatio != 0 || power.WidthRatio != 2.0/3 || power.YRatio != 1.0/2 || power.HeightRatio != 1.0/4 {
		t.Fatalf("power grid placement = %+v, want left third row", power)
	}
	temperature := itemFor(unifiedTemperatureHistoryChart)
	if temperature.XRatio != 0 || temperature.WidthRatio != 2.0/3 || temperature.YRatio != 3.0/4 || temperature.HeightRatio != 1.0/4 {
		t.Fatalf("temperature grid placement = %+v, want left fourth row", temperature)
	}
	for _, chart := range []any{unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart, unifiedTemperatureHistoryChart} {
		item := itemFor(chart)
		if item.XRatio+item.WidthRatio > 2.0/3 {
			t.Fatalf("%T exceeds left two thirds: %+v", chart, item)
		}
	}
	processes := itemFor(unifiedProcessList)
	if processes.XRatio != 2.0/3 || processes.YRatio != 1.0/4 || processes.WidthRatio != 1.0/3 || processes.HeightRatio != 1.0/4 {
		t.Fatalf("process grid placement = %+v, want right second row aligned with memory", processes)
	}
	for chart, wantY := range map[any]float64{unifiedNetworkHistoryChart: 1.0 / 2, unifiedDiskHistoryChart: 3.0 / 4} {
		item := itemFor(chart)
		if item.XRatio != 2.0/3 || item.YRatio != wantY || item.WidthRatio != 1.0/3 || item.HeightRatio != 1.0/4 {
			t.Fatalf("%T grid placement = %+v, want aligned right-side I/O row", chart, item)
		}
	}
}

func TestUnifiedLayoutHidesSidebarOnNarrowTerminal(t *testing.T) {
	origGrid := grid
	origCompute, origMemory, origPower := unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart
	origNetwork, origDisk := unifiedNetworkHistoryChart, unifiedDiskHistoryChart
	origCores, origTemp, origProcesses := cpuCoreWidget, unifiedTemperatureHistoryChart, unifiedProcessList
	t.Cleanup(func() {
		grid = origGrid
		unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart = origCompute, origMemory, origPower
		unifiedNetworkHistoryChart, unifiedDiskHistoryChart = origNetwork, origDisk
		cpuCoreWidget, unifiedTemperatureHistoryChart, unifiedProcessList = origCores, origTemp, origProcesses
	})

	grid = ui.NewGrid()
	unifiedComputeHistoryChart, memoryHistoryChart, socPowerHistoryChart = NewPeakStepChart(), NewPeakStepChart(), NewPeakStepChart()
	unifiedNetworkHistoryChart, unifiedDiskHistoryChart = NewPeakStepChart(), NewPeakStepChart()
	cpuCoreWidget = &CPUCoreWidget{Block: ui.NewBlock()}
	unifiedTemperatureHistoryChart, unifiedProcessList = NewPeakStepChart(), NewStyledList()
	setUnifiedLayoutGridForWidth(99)

	if got, want := len(grid.Items), 4; got != want {
		t.Fatalf("narrow layout item count = %d, want %d charts only", got, want)
	}
	for _, item := range grid.Items {
		if item.XRatio+item.WidthRatio > 1 {
			t.Fatalf("narrow layout item exceeds full width: %+v", item)
		}
	}
}

func TestUnifiedComponentTemperaturesSelectMemoryAndSSD(t *testing.T) {
	memory, ssd := unifiedComponentTemperatures([]TempSensor{
		{Key: "Tm0", Name: "Memory", Value: 44},
		{Key: "Ts0", Name: "SSD", Value: 40},
	})
	if memory != 44 || ssd != 40 {
		t.Fatalf("component temperatures = memory %.1f, ssd %.1f", memory, ssd)
	}
}

func TestUnifiedTemperatureDisplayBoundsRoundToTens(t *testing.T) {
	low, high := unifiedTemperatureDisplayBounds([]float64{33, 35})
	if low != 30 || high != 40 {
		t.Fatalf("temperature bounds = %.0f..%.0f, want 30..40", low, high)
	}
}

func TestUnifiedTemperatureChartColorsPreserveFanLane(t *testing.T) {
	if got, want := unifiedTemperatureChartColors(2), []ui.Color{ui.ColorSilver, ui.ColorGreen, ui.ColorCyan, ui.ColorMagenta, ui.ColorYellow, ui.ColorRed}; !slices.Equal(got, want) {
		t.Fatalf("temperature colors with fan = %v, want %v", got, want)
	}
}

func TestUpdateUnifiedTemperatureHistoryRendersComponentsAndFan(t *testing.T) {
	origChart := unifiedTemperatureHistoryChart
	origCPU, origGPU := cpuTempHistory, gpuTempHistory
	origMemory, origSSD := memoryTempHistory, ssdTempHistory
	origFanDuty, origFanRPM := fanDutyHistory, fanRPMHistory
	origFanIDs, origFanNames := fanHistoryIDs, fanHistoryNames
	origWidth, origHeight := GetCachedTerminalDimensions()
	origLayout := currentConfig.DefaultLayout
	t.Cleanup(func() {
		unifiedTemperatureHistoryChart = origChart
		cpuTempHistory, gpuTempHistory = origCPU, origGPU
		memoryTempHistory, ssdTempHistory = origMemory, origSSD
		fanDutyHistory, fanRPMHistory = origFanDuty, origFanRPM
		fanHistoryIDs, fanHistoryNames = origFanIDs, origFanNames
		currentConfig.DefaultLayout = origLayout
		UpdateCachedTerminalDimensions(origWidth, origHeight)
	})

	UpdateCachedTerminalDimensions(30, 20)
	currentConfig.DefaultLayout = LayoutUnified
	unifiedTemperatureHistoryChart = NewPeakStepChart()
	cpuTempHistory, gpuTempHistory = make([]float64, 10), make([]float64, 10)
	memoryTempHistory, ssdTempHistory = make([]float64, 10), make([]float64, 10)
	fanDutyHistory = [2][]float64{make([]float64, 10), make([]float64, 10)}
	fanRPMHistory = [2][]float64{make([]float64, 10), make([]float64, 10)}
	fanHistoryIDs, fanHistoryNames = [2]int{}, [2]string{}
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)
	recordHistoricalPeak("temperature-C", 90)
	recordHistoricalPeak("fan-rpm-0", 3600)
	updateUnifiedTemperatureHistory(CPUMetrics{
		CPUTemp: 52,
		GPUTemp: 49,
		TempSensors: []TempSensor{
			{Key: "Tm0", Name: "Memory", Value: 45},
			{Key: "Ts0", Name: "SSD", Value: 41},
		},
		Fans: []FanInfo{{ID: 1, Name: "Left", ActualRPM: 1200, MaxRPM: 2400}, {ID: 2, Name: "Fan Two", ActualRPM: 1800, MaxRPM: 3600}},
	})

	if got, want := unifiedTemperatureHistoryChart.DataLabels, []string{"L 1.2k", "F2 1.8k", "S 41°C", "M 45°C", "G 49°C", "C 52°C"}; !slices.Equal(got, want) {
		t.Fatalf("temperature labels = %v, want %v", got, want)
	}
	if got, want := unifiedTemperatureHistoryChart.LineColors, []ui.Color{ui.ColorSilver, ui.ColorGreen, ui.ColorCyan, ui.ColorMagenta, ui.ColorYellow, ui.ColorRed}; !slices.Equal(got, want) {
		t.Fatalf("temperature colors = %v, want %v", got, want)
	}
	foundF2 := false
	for _, span := range unifiedTemperatureHistoryChart.TitleSpans {
		if span.Text == "2" && span.Style.Fg == ui.ColorGreen && span.Style.Modifier == ui.ModifierBold {
			foundF2 = true
		}
	}
	if !foundF2 {
		t.Fatalf("F2 title value must be bold green: %#v", unifiedTemperatureHistoryChart.TitleSpans)
	}
	if got := unifiedTemperatureHistoryChart.Data[0][len(unifiedTemperatureHistoryChart.Data[0])-1]; got != 50 {
		t.Fatalf("left fan duty = %.1f, want 50", got)
	}
	if got := unifiedTemperatureHistoryChart.Data[5][len(unifiedTemperatureHistoryChart.Data[5])-1]; got != 70 {
		t.Fatalf("CPU scaled temperature = %.1f, want 70", got)
	}
	if got, want := unifiedTemperatureHistoryChart.PeakLabels, []string{"L 1k", "F2 2k", "S 41°C", "M 45°C", "G 49°C", "C 52°C"}; !slices.Equal(got, want) {
		t.Fatalf("temperature peak labels = %v, want visible peaks %v", got, want)
	}
	if !strings.Contains(unifiedTemperatureHistoryChart.Title, "CPU: 90°C") || !strings.Contains(unifiedTemperatureHistoryChart.Title, "Fan Left: 4k") {
		t.Fatalf("temperature title = %q, want retained peaks", unifiedTemperatureHistoryChart.Title)
	}
}

func TestSoCPowerHistoryDrawsTotalAndShowsOnlyItsPeakInTitle(t *testing.T) {
	origChart, origHistory := socPowerHistoryChart, totalPowerHistory
	origConfig := currentConfig
	origAdapterWatts := getExternalPowerAdapterWatts
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() {
		socPowerHistoryChart, totalPowerHistory = origChart, origHistory
		currentConfig = origConfig
		getExternalPowerAdapterWatts = origAdapterWatts
		UpdateCachedTerminalDimensions(origWidth, origHeight)
	})

	UpdateCachedTerminalDimensions(10, 20)
	currentConfig.DefaultLayout = LayoutUnified
	getExternalPowerAdapterWatts = func() int { return 0 }
	socPowerHistoryChart = NewPeakStepChart()
	totalPowerHistory = make([]float64, 100)
	totalPowerHistory[len(totalPowerHistory)-6] = 15
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)
	recordHistoricalPeak("power-total", 15)
	updateSoCPowerHistory(CPUMetrics{CPUW: 4, GPUW: 3, DRAMW: 2, ANEW: 1, PackageW: 12})
	if got, want := socPowerHistoryChart.DataLabels[0], "T 12.0W"; got != want {
		t.Fatalf("total current label = %q, want %q", got, want)
	}
	if got, want := socPowerHistoryChart.Data[0][len(socPowerHistoryChart.Data[0])-1], 12.0; got != want {
		t.Fatalf("total trace current value = %.1f, want %.1f", got, want)
	}
	if got, want := socPowerHistoryChart.PeakLabels[0], "T 12W"; got != want {
		t.Fatalf("left peak label = %q, want visible peak %q", got, want)
	}
	if !strings.Contains(socPowerHistoryChart.Title, "Total: 15W") || strings.Contains(socPowerHistoryChart.Title, "Total: 12W/") {
		t.Fatalf("power title = %q", socPowerHistoryChart.Title)
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

func TestRenderUnifiedIOChartUsesVisiblePeakLabelsAndRetainedTitlePeaks(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() { UpdateCachedTerminalDimensions(origWidth, origHeight) })
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)
	UpdateCachedTerminalDimensions(6, 10)

	chart := NewPeakStepChart()
	first := []float64{1000, 10, 20}
	second := []float64{2000, 30, 40}
	recordHistoricalPeak("disk-read", 4000)
	recordHistoricalPeak("disk-write", 8000)
	renderUnifiedIOChart(chart, first, second, "I/O", "R", "W", "disk-read", "disk-write", "B", []ui.Color{ui.ColorCyan, ui.ColorRed}, map[string]string{"R": "Read", "W": "Write"}, nil)

	if got, want := chart.Data, [][]float64{{10, 20}, {30, 40}}; !slices.Equal(got[0], want[0]) || !slices.Equal(got[1], want[1]) {
		t.Fatalf("I/O visible data = %v, want %v", got, want)
	}
	if got, want := chart.PeakLabels, []string{"R " + formatRoundedBytes(20, "B") + "/s", "W " + formatRoundedBytes(40, "B") + "/s"}; !slices.Equal(got, want) {
		t.Fatalf("I/O peak labels = %v, want visible peaks %v", got, want)
	}
	if want := "I/O (Read: " + formatRoundedBytes(4000, "B") + "/s, Write: " + formatRoundedBytes(8000, "B") + "/s)"; chart.Title != want {
		t.Fatalf("I/O title = %q, want full-history peaks %q", chart.Title, want)
	}
}

func TestUnifiedNetworkAndDiskPeakLabelsUseVisibleWindow(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() { UpdateCachedTerminalDimensions(origWidth, origHeight) })
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)
	UpdateCachedTerminalDimensions(6, 10)

	recordHistoricalPeak("network-link", 1000)
	recordHistoricalPeak("network-download", 4000)
	recordHistoricalPeak("network-upload", 8000)
	network := NewPeakStepChart()
	renderUnifiedNetworkHistory(network, []float64{1000, 10, 20}, []float64{2000, 30, 40}, []float64{1000, 100, 200}, "200Mbps", "", "Network", "B")
	if got, want := network.PeakLabels, []string{"L 200Mbps", "U 40B/s", "D 20B/s"}; !slices.Equal(got, want) {
		t.Fatalf("network peak labels = %v, want visible peaks %v", got, want)
	}
	if want := "Network (D: 4KB/s, U: 8KB/s, Link: 200Mbps)"; network.Title != want {
		t.Fatalf("network title = %q, want retained peaks %q", network.Title, want)
	}

	recordHistoricalPeak("disk-used", 9000)
	recordHistoricalPeak("disk-read", 4000)
	recordHistoricalPeak("disk-write", 8000)
	disk := NewPeakStepChart()
	renderUnifiedDiskHistory(disk, []float64{1000, 10, 20}, []float64{2000, 30, 40}, []float64{900, 100, 200}, 1000, "Disk", "B")
	if got, want := disk.PeakLabels, []string{"U 200B", "R 20B/s", "W 40B/s"}; !slices.Equal(got, want) {
		t.Fatalf("disk peak labels = %v, want visible peaks %v", got, want)
	}
	if !strings.Contains(disk.Title, "R: 4KB/s") || !strings.Contains(disk.Title, "W: 8KB/s") {
		t.Fatalf("disk title = %q, want retained throughput peaks", disk.Title)
	}
}

func TestRenderUnifiedDiskHistoryAddsOccupiedLineOnNarrowAndWideTerminals(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	origLayout := currentConfig.DefaultLayout
	t.Cleanup(func() {
		UpdateCachedTerminalDimensions(origWidth, origHeight)
		currentConfig.DefaultLayout = origLayout
	})
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)

	read := []float64{10 * 1024 * 1024, 20 * 1024 * 1024}
	write := []float64{5 * 1024 * 1024, 15 * 1024 * 1024}
	used := []float64{600 * 1e9, 400 * 1e9}

	UpdateCachedTerminalDimensions(120, 30)
	currentConfig.DefaultLayout = LayoutUnified
	wideChart := NewPeakStepChart()
	renderUnifiedDiskHistory(wideChart, read, write, used, 1e12, "Disk", "auto")
	if got, want := len(wideChart.Data), 3; got != want {
		t.Fatalf("wide disk series = %d, want %d", got, want)
	}
	if got, want := wideChart.Data[0][1], 40.0; got != want {
		t.Fatalf("occupied percent = %.1f, want %.1f", got, want)
	}
	if got, want := wideChart.DataLabels[0], "U 400GB"; got != want {
		t.Fatalf("occupied current label = %q, want %q", got, want)
	}
	if got, want := wideChart.SeriesGroups, []string{"occupied", "throughput", "throughput"}; !slices.Equal(got, want) {
		t.Fatalf("wide disk draw order = %v, want %v", got, want)
	}
	if !strings.Contains(wideChart.Title, "U: 400/1TB 40%") {
		t.Fatalf("wide disk title = %q", wideChart.Title)
	}
	styles := make(map[string]ui.Style)
	for _, span := range wideChart.TitleSpans {
		styles[span.Text] = span.Style
	}
	if style := styles["20"]; style.Fg != ui.ColorCyan || style.Modifier != ui.ModifierBold {
		t.Fatalf("R title value style = %+v, want bold cyan", style)
	}
	if style := styles["15"]; style.Fg != ui.ColorRed || style.Modifier != ui.ModifierBold {
		t.Fatalf("W title value style = %+v, want bold red", style)
	}

	UpdateCachedTerminalDimensions(99, 30)
	renderUnifiedDiskHistory(wideChart, read, write, used, 1e12, "Disk", "auto")
	if got, want := len(wideChart.Data), 3; got != want {
		t.Fatalf("narrow disk series = %d, want %d", got, want)
	}
	if !strings.Contains(wideChart.Title, "U: 400/1TB 40%") {
		t.Fatalf("narrow disk title must retain occupancy: %q", wideChart.Title)
	}
	if got, want := wideChart.SeriesGroups, []string{"occupied", "throughput", "throughput"}; !slices.Equal(got, want) {
		t.Fatalf("narrow disk draw order = %v, want %v", got, want)
	}
}

func TestRenderUnifiedDiskHistoryRoundsOccupiedLine(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() { UpdateCachedTerminalDimensions(origWidth, origHeight) })
	UpdateCachedTerminalDimensions(120, 30)

	chart := NewPeakStepChart()
	renderUnifiedDiskHistory(chart, []float64{1}, []float64{1}, []float64{405.5}, 1000, "Disk", "B")
	if got, want := chart.Data[0][0], 41.0; got != want {
		t.Fatalf("occupied chart value = %.1f, want integer %.1f", got, want)
	}
}

func TestRenderUnifiedNetworkHistoryAddsLinkLineOnNarrowAndWideTerminals(t *testing.T) {
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() { UpdateCachedTerminalDimensions(origWidth, origHeight) })
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)

	download := []float64{20 * 1024 * 1024, 40 * 1024 * 1024}
	upload := []float64{10 * 1024 * 1024, 30 * 1024 * 1024}
	link := []float64{1000, 2500}
	recordHistoricalPeak("network-download", download[1])
	recordHistoricalPeak("network-upload", upload[1])
	recordHistoricalPeak("network-link", link[1])

	UpdateCachedTerminalDimensions(120, 30)
	chart := NewPeakStepChart()
	renderUnifiedNetworkHistory(chart, download, upload, link, "2.5Gbps", "5GbE", "Network", "auto")
	if got, want := len(chart.Data), 3; got != want {
		t.Fatalf("wide network series = %d, want %d", got, want)
	}
	if got, want := chart.SeriesGroups, []string{"link", "throughput", "throughput"}; !slices.Equal(got, want) {
		t.Fatalf("wide network draw order = %v, want %v", got, want)
	}
	if got, want := chart.LineColors[0], ui.ColorMagenta; got != want {
		t.Fatalf("link color = %v, want occupied color %v", got, want)
	}
	if got, want := chart.DataLabels[0], "L 2.5Gbps"; got != want {
		t.Fatalf("link label = %q, want %q", got, want)
	}
	if !strings.Contains(chart.Title, "Link: 2.5/5GbE") {
		t.Fatalf("wide network title = %q", chart.Title)
	}
	if !strings.Contains(chart.Title, "D: 40MB/s") || !strings.Contains(chart.Title, "U: 30MB/s") || strings.Contains(chart.Title, "Download:") || strings.Contains(chart.Title, "Upload:") {
		t.Fatalf("wide network title must abbreviate traffic directions: %q", chart.Title)
	}

	UpdateCachedTerminalDimensions(99, 30)
	renderUnifiedNetworkHistory(chart, download, upload, link, "2.5Gbps", "5GbE", "Network", "auto")
	if got, want := len(chart.Data), 3; got != want {
		t.Fatalf("narrow network series = %d, want %d", got, want)
	}
	if !strings.Contains(chart.Title, "Link: 2.5/5GbE") {
		t.Fatalf("narrow network title must retain Link: %q", chart.Title)
	}
	if !strings.Contains(chart.Title, "D: 40MB/s") || !strings.Contains(chart.Title, "U: 30MB/s") || strings.Contains(chart.Title, "Download:") || strings.Contains(chart.Title, "Upload:") {
		t.Fatalf("narrow network title must abbreviate traffic directions: %q", chart.Title)
	}
	if got, want := chart.SeriesGroups, []string{"link", "throughput", "throughput"}; !slices.Equal(got, want) {
		t.Fatalf("narrow network draw order = %v, want %v", got, want)
	}
}

func TestGetBestLinkCapacityString(t *testing.T) {
	ethernet := []EthernetLinkInfo{{Name: "en0", LinkUp: true, LinkSpeedMbps: 1000, SupportedSpeedMbps: 2500}}
	if got, want := getBestLinkCapacityString(ethernet, &WiFiLinkInfo{IsConnected: true, TxRateMbps: 866}), "1GbE"; got != want {
		t.Fatalf("best Ethernet capacity = %q, want %q", got, want)
	}
	if got, want := getBestLinkCapacityString(nil, &WiFiLinkInfo{IsConnected: true, TxRateMbps: 2400}), "2.4Gbps"; got != want {
		t.Fatalf("Wi-Fi current rate = %q, want %q", got, want)
	}
	if got := getBestLinkCapacityString(nil, nil); got != "" {
		t.Fatalf("unavailable link capacity = %q, want empty", got)
	}
	if current, currentText, maximumText := getBestLinkCapacity(ethernet, nil); current != 1000 || currentText != "1GbE" || maximumText != "2.5GbE" {
		t.Fatalf("Ethernet link capability = (%d, %q, %q), want (1000, %q, %q)", current, currentText, maximumText, "1GbE", "2.5GbE")
	}
	if _, _, maximumText := getBestLinkCapacity([]EthernetLinkInfo{{Name: "en0", LinkUp: true, LinkSpeedMbps: 1000}}, nil); maximumText != "" {
		t.Fatalf("unknown Ethernet capability = %q, want empty", maximumText)
	}
}

func TestFormatCurrentAndMaximumOmitsCurrentUnit(t *testing.T) {
	if got, want := formatCurrentAndMaximum("400GB", "1TB"), "400/1TB"; got != want {
		t.Fatalf("disk current/capacity = %q, want %q", got, want)
	}
	if got, want := formatCurrentAndMaximum("2.5Gbps", "5GbE"), "2.5/5GbE"; got != want {
		t.Fatalf("link current/capacity = %q, want %q", got, want)
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

func TestSoCPowerHistoryColorsMatchUnifiedComputeComponents(t *testing.T) {
	// Power series are Total/CPU/GPU/DRAM/ANE; compute series are ANE/GPU/CPU.
	power := socPowerHistoryColors()
	compute := unifiedComputeHistoryColors()
	if got, want := power, []ui.Color{ui.ColorSilver, ui.ColorRed, ui.ColorYellow, ui.ColorCyan, ui.ColorGreen}; !slices.Equal(got, want) {
		t.Fatalf("power colors = %v, want %v", got, want)
	}
	if power[1] != compute[2] || power[2] != compute[1] || power[4] != compute[0] {
		t.Fatalf("power CPU/GPU/ANE colors = %v do not match compute %v", power, compute)
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
	if got := unifiedComputeHistoryChart.Title; !strings.Contains(got, "(CPU: 20%, GPU: 40%, ANE: 60%)") || strings.Contains(got, "°") {
		t.Fatalf("compute title = %q", got)
	}
}

func TestUnifiedComputePeakLabelsUseVisibleWindow(t *testing.T) {
	origChart, origCPU, origGPU, origANE := unifiedComputeHistoryChart, cpuUsageHistory, gpuEffectiveHistory, aneUsageHistory
	origMetrics := lastCPUMetrics
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() {
		unifiedComputeHistoryChart, cpuUsageHistory, gpuEffectiveHistory, aneUsageHistory = origChart, origCPU, origGPU, origANE
		lastCPUMetrics = origMetrics
		UpdateCachedTerminalDimensions(origWidth, origHeight)
	})
	resetHistoricalPeaks()
	t.Cleanup(resetHistoricalPeaks)

	UpdateCachedTerminalDimensions(6, 20)
	unifiedComputeHistoryChart = NewPeakStepChart()
	cpuUsageHistory = []float64{90, 10, 20}
	gpuEffectiveHistory = []float64{80, 30, 40}
	aneUsageHistory = []float64{70, 50, 60}
	lastCPUMetrics = CPUMetrics{AvgUsage: 20}
	recordHistoricalPeak("cpu-usage", 95)
	recordHistoricalPeak("gpu-effective", 85)
	recordHistoricalPeak("ane-usage", 75)

	renderUnifiedComputeHistory()
	if got, want := unifiedComputeHistoryChart.PeakLabels, []string{"A 60%", "G 40%", "C 20%"}; !slices.Equal(got, want) {
		t.Fatalf("compute peak labels = %v, want visible peaks %v", got, want)
	}
	if !strings.Contains(unifiedComputeHistoryChart.Title, "CPU: 95%") || !strings.Contains(unifiedComputeHistoryChart.Title, "GPU: 85%") || !strings.Contains(unifiedComputeHistoryChart.Title, "ANE: 75%") {
		t.Fatalf("compute title = %q, want retained peaks", unifiedComputeHistoryChart.Title)
	}
}

func TestPeakStepChartDrawsPeakBesideLeftAxis(t *testing.T) {
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
	for y := chart.Inner.Min.Y; y < chart.Inner.Max.Y; y++ {
		if buf.GetCell(image.Pt(chart.Inner.Min.X, y)).Rune == 'C' {
			peakFound = true
		}
	}
	if !peakFound {
		t.Fatal("peak label was not drawn beside the left axis")
	}
	if got := buf.GetCell(image.Pt(19, 6)).Rune; got == 'C' {
		t.Fatal("peak label was drawn at the right edge instead of at its sample")
	}
}

func TestPeakStepChartStacksOverlappingPeaksInTheLeftColumn(t *testing.T) {
	chart := NewPeakStepChart()
	chart.Border = false
	chart.ShowAxes = false
	chart.ShowRightAxis = true
	chart.SetRect(0, 0, 20, 8)
	chart.MaxVal = 10
	chart.Data = [][]float64{{9.2, 1}, {9.0, 2}}
	chart.DataLabels = []string{"D 1.0", "M 1.0"}
	chart.PeakLabels = []string{"P1", "P2"}
	chart.ShowPeakLabels = true

	buf := ui.NewBuffer(image.Rect(0, 0, 20, 8))
	chart.Draw(buf)
	left := chart.Inner.Min
	peakRows := make(map[string]int)
	for y := chart.Inner.Min.Y; y < chart.Inner.Max.Y; y++ {
		if buf.GetCell(image.Pt(left.X, y)).Rune == 'P' {
			peakRows[string([]rune{buf.GetCell(image.Pt(left.X+1, y)).Rune})] = y
		}
	}
	if len(peakRows) != 2 {
		t.Fatalf("left-column peak rows = %v, want two labels", peakRows)
	}
	if peakRows["1"] >= peakRows["2"] {
		t.Fatalf("higher peak P1 must be above lower peak P2: rows %v", peakRows)
	}
	for _, y := range peakRows {
		if got := buf.GetCell(image.Pt(left.X+3, y)).Rune; got == 'P' {
			t.Fatalf("peak label at row %d was placed horizontally beside the left column", y)
		}
	}
}

func TestCurrentLabelPlacementKeepsDRAMLabelsSeparateFromMemoryLabels(t *testing.T) {
	area := image.Rect(0, 0, 20, 4)
	var occupied []image.Rectangle
	placements := make(map[int]chartLabelPlacement)
	for _, line := range currentLabelOrder(4) {
		placement := chooseCurrentLabelPlacement(area, "X 10.0", 2, line, occupied, func(image.Rectangle) int { return 0 })
		placements[line] = placement
		occupied = append(occupied, placement.rect)
	}
	for line, placement := range placements {
		for other, otherPlacement := range placements {
			if line != other && placement.rect.Overlaps(otherPlacement.rect) {
				t.Fatalf("labels %d and %d overlap: %v and %v", line, other, placement.rect, otherPlacement.rect)
			}
		}
	}
	if placements[2].rect.Min.X != area.Max.X-1-runewidth.StringWidth("X 10.0") || placements[3].rect.Min.X != area.Max.X-1-runewidth.StringWidth("X 10.0") {
		t.Fatalf("DRAM labels must retain the right-side current-value column: read=%v write=%v", placements[2].rect, placements[3].rect)
	}
	for line, placement := range placements {
		if placement.rect.Min.X != area.Max.X-1-runewidth.StringWidth("X 10.0") {
			t.Fatalf("label %d moved away from the right axis: %v", line, placement.rect)
		}
	}
}

func TestCurrentLabelPlacementCoversItsLatestLine(t *testing.T) {
	area := image.Rect(0, 0, 20, 5)
	placement := chooseCurrentLabelPlacement(area, "R 10.0", 2, 2, nil, func(rect image.Rectangle) int {
		if rect.Min.Y == 2 {
			return 3
		}
		return 0
	})
	if got, want := placement.rect.Min.Y, 2; got != want {
		t.Fatalf("current-value row = %d, want latest-line row %d", got, want)
	}
}

func TestCurrentLabelPlacementStaysNearItsLineWhenNoLocalGapExists(t *testing.T) {
	area := image.Rect(0, 0, 20, 7)
	placement := chooseCurrentLabelPlacement(area, "M 33.1", 3, 1, nil, func(rect image.Rectangle) int {
		if rect.Min.Y <= 5 { // Every row within the two-row avoidance radius has a line.
			return 1
		}
		return 0
	})
	if got, want := placement.rect.Min.Y, 3; got != want {
		t.Fatalf("label moved away from its current line to row %d, want %d", got, want)
	}
}

func TestCurrentSameUnitLabelsCannotInvertAfterLineAvoidance(t *testing.T) {
	area := image.Rect(0, 0, 20, 5)
	low := chooseCurrentLabelPlacementWithin(area, "A 0.0", 4, 0, nil, 4, func(image.Rectangle) int { return 0 })
	high := chooseCurrentLabelPlacementWithin(area, "G 0.8", 4, 1, []image.Rectangle{low.rect}, low.rect.Min.Y-1, func(image.Rectangle) int { return 1 })
	if high.rect.Min.Y >= low.rect.Min.Y {
		t.Fatalf("higher current value row %d must be above lower row %d", high.rect.Min.Y, low.rect.Min.Y)
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

	if got, want := formatUnifiedProcessRow(process, 59), "    42   0  12.3%   2.5%  1.5G  2.0G long-running-command"; got != want {
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

func TestFormatUnifiedProcessSummary(t *testing.T) {
	if got, want := formatUnifiedProcessSummary(summarizeUnifiedProcesses([]ProcessMetrics{{}, {}, {}})), "Total: 3/3"; got != want {
		t.Fatalf("process summary = %q, want %q", got, want)
	}
}

func TestUnifiedProcessListTitleUsesSummaryOutsideSearch(t *testing.T) {
	origList, origSummary, origSearchMode, origSearchText := unifiedProcessList, latestUnifiedProcessSummary, searchMode, searchText
	t.Cleanup(func() {
		unifiedProcessList, latestUnifiedProcessSummary = origList, origSummary
		searchMode, searchText = origSearchMode, origSearchText
	})

	unifiedProcessList = NewStyledList()
	latestUnifiedProcessSummary = unifiedProcessSummary{Total: 980, PeakTotal: 1000}
	searchMode, searchText = false, ""
	updateUnifiedProcessList([]ProcessMetrics{{PID: 1}})
	if got, want := unifiedProcessList.Title, i18n.T("TUI_ProcessList")+" Total: 980/1000 | / for search"; got != want {
		t.Fatalf("unified process title = %q, want %q", got, want)
	}

	searchText = "Safari"
	updateUnifiedProcessList([]ProcessMetrics{{PID: 1}})
	if strings.Contains(unifiedProcessList.Title, "Total: 980/1000") {
		t.Fatalf("search title %q must not include process summary", unifiedProcessList.Title)
	}
}

func TestUnifiedProcessSearchAppliesOnEnter(t *testing.T) {
	origLayout := currentConfig.DefaultLayout
	origProcessList, origUnifiedList, origProcesses := processList, unifiedProcessList, lastProcesses
	origSearchMode, origSearchDraft, origSearchText, origFiltered := searchMode, searchDraft, searchText, filteredProcesses
	origWidth, origHeight := GetCachedTerminalDimensions()
	t.Cleanup(func() {
		currentConfig.DefaultLayout = origLayout
		processList, unifiedProcessList, lastProcesses = origProcessList, origUnifiedList, origProcesses
		searchMode, searchDraft, searchText, filteredProcesses = origSearchMode, origSearchDraft, origSearchText, origFiltered
		UpdateCachedTerminalDimensions(origWidth, origHeight)
	})

	currentConfig.DefaultLayout = LayoutUnified
	UpdateCachedTerminalDimensions(180, 30)
	processList, unifiedProcessList = w.NewList(), NewStyledList()
	lastProcesses = []ProcessMetrics{{PID: 1, Command: "Safari"}, {PID: 2, Command: "Terminal"}}
	searchMode, searchDraft, searchText, filteredProcesses = false, "", "", nil

	handleProcessListEvents(ui.Event{ID: "/"})
	handleProcessListEvents(ui.Event{ID: "s"})
	if !searchMode || searchDraft != "s" || len(unifiedProcessList.Rows) != 3 {
		t.Fatalf("draft search mode = %t, draft = %q, rows = %d; want active draft with unfiltered rows", searchMode, searchDraft, len(unifiedProcessList.Rows))
	}

	handleProcessListEvents(ui.Event{ID: "<Enter>"})
	if searchMode || searchText != "s" || len(unifiedProcessList.Rows) != 2 || !strings.Contains(unifiedProcessList.Rows[1], "Safari") {
		t.Fatalf("committed search mode = %t, text = %q, rows = %q", searchMode, searchText, unifiedProcessList.Rows)
	}

	handleProcessListEvents(ui.Event{ID: "<Escape>"})
	if searchText != "" || len(unifiedProcessList.Rows) != 3 {
		t.Fatalf("cleared search text = %q, rows = %d; want full list", searchText, len(unifiedProcessList.Rows))
	}
}

func TestUnifiedProcessHeaderShowsSelectedSortColumn(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})

	unifiedProcessSelectedColumn = 1
	unifiedProcessSortReverse = false
	if got := formatUnifiedProcessHeader(40); !strings.Contains(got, "ACT↓") {
		t.Fatalf("header %q does not show selected descending column", got)
	}
	unifiedProcessSortReverse = true
	if got := formatUnifiedProcessHeader(40); !strings.Contains(got, "ACT↑") {
		t.Fatalf("header %q does not show reversed column", got)
	}
}

func TestUnifiedProcessHeaderAlignsWithDataColumns(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})
	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 2, false

	if got, want := formatUnifiedProcessHeader(45), "   PID ACT   CPU↓    GPU   RSS  FOOT CMD"; got != want {
		t.Fatalf("aligned header = %q, want %q", got, want)
	}
	if got, want := formatUnifiedProcessRow(ProcessMetrics{PID: 42, CPU: 12.3, RSS: 1536 * 1024, Footprint: 2048 * 1024, Command: "cmd"}, 49), "    42   0  12.3%   0.0%  1.5G  2.0G cmd"; got != want {
		t.Fatalf("aligned row = %q, want %q", got, want)
	}
}

func TestSortUnifiedProcessesUsesSeparateCPUAndGPU(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})
	processes := []ProcessMetrics{
		{PID: 1, CPU: 20, GPU: 0, RSS: 3 * 1024 * 1024},
		{PID: 2, CPU: 10, GPU: 200, RSS: 1 * 1024 * 1024},
	}

	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 2, false
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 1 {
		t.Fatalf("CPU first PID = %d, want 1", got)
	}
	unifiedProcessSelectedColumn = 3
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 2 {
		t.Fatalf("GPU first PID = %d, want 2", got)
	}
	unifiedProcessSelectedColumn = 4
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

	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 5, false
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 2 {
		t.Fatalf("footprint first PID = %d, want 2", got)
	}
}

func TestCalculateProcessActivityUsesLargestStaticMemoryView(t *testing.T) {
	totalMem := uint64(100 * 1024 * 1024 * 1024)
	process := ProcessMetrics{
		RSS:       10 * 1024 * 1024,
		Footprint: 12 * 1024 * 1024,
	}

	// Twelve GiB is 80% of the 15 GiB static-memory reference. RSS and
	// footprint overlap, so the score must use 12 GiB rather than their sum.
	if got, want := calculateProcessActivity(process, totalMem), 2.1825; math.Abs(got-want) > 0.0001 {
		t.Fatalf("static memory activity = %.4f, want %.4f", got, want)
	}
}

func TestCalculateProcessActivityIncludesPageInsAndDisk(t *testing.T) {
	process := ProcessMetrics{
		PageInsPerSecond:   50,
		DiskBytesPerSecond: 64 * 1024 * 1024,
	}

	if got, want := calculateProcessActivity(process, 0), 7.875; math.Abs(got-want) > 0.0001 {
		t.Fatalf("dynamic memory/disk activity = %.4f, want %.4f", got, want)
	}
}

func TestUnifiedProcessActivitySortUsesFractionalValue(t *testing.T) {
	origColumn, origReverse := unifiedProcessSelectedColumn, unifiedProcessSortReverse
	t.Cleanup(func() {
		unifiedProcessSelectedColumn, unifiedProcessSortReverse = origColumn, origReverse
	})
	processes := []ProcessMetrics{{PID: 1, Activity: 1.49}, {PID: 2, Activity: 1.41}}

	unifiedProcessSelectedColumn, unifiedProcessSortReverse = 3, false
	sortUnifiedProcesses(processes)
	if got := processes[0].PID; got != 1 {
		t.Fatalf("fractional ACT sort first PID = %d, want 1", got)
	}
	if got := formatUnifiedProcessRow(processes[0], 40); !strings.Contains(got, "   1 ") {
		t.Fatalf("ACT row = %q, want rounded integer display", got)
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
