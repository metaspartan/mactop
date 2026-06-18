package app

import (
	"math"
	"testing"
	"time"

	ui "github.com/metaspartan/gotui/v5"
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

// TestActivityMonitorMemoryUsed covers the macOS memory-used formula: anonymous
// app memory + wired + compressed, with underflow guards. The headline case is
// large anonymous memory (MLX weights) that macOS has aged into the inactive
// queue — the old free+inactive formula collapsed toward 0 there.
func TestActivityMonitorMemoryUsed(t *testing.T) {
	const gb = 1 << 30
	cases := []struct {
		name                                           string
		anonymous, purgeable, wired, compressed, total uint64
		want                                           uint64
	}{
		{"typical", 40 * gb, 1 * gb, 8 * gb, 2 * gb, 64 * gb, 49 * gb},
		// 60GB anonymous (model weights) even if macOS queues it as inactive:
		// must count as used, not vanish.
		{"mlx-heavy", 60 * gb, 0, 6 * gb, 4 * gb, 96 * gb, 70 * gb},
		{"purgeable-exceeds-anon", 1 * gb, 3 * gb, 2 * gb, 0, 16 * gb, 2 * gb},
		{"clamp-to-total", 200 * gb, 0, 100 * gb, 50 * gb, 128 * gb, 128 * gb},
	}
	for _, c := range cases {
		if got := activityMonitorMemoryUsed(c.anonymous, c.purgeable, c.wired, c.compressed, c.total); got != c.want {
			t.Errorf("%s: got %d GB, want %d GB", c.name, got/gb, c.want/gb)
		}
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

// TestANEExclaveBinary covers the exclave ANE path (M5 / M5 Max): the
// power-domain signal is binary, must collapse to 0/100, must render an ON/idle
// label, and must NOT latch the bandwidth mode (which would make the gauge show
// the binary value as a misleading percentage).
func TestANEExclaveBinary(t *testing.T) {
	resetANETestState(t)

	// Powered: any non-zero duty cycle collapses to 100 and reads "ON".
	on := CPUMetrics{ANEExclave: true, ANEPowered: true, ANEActive: 73}
	if got := aneUtilizationPercent(on); got != 100 {
		t.Fatalf("exclave powered: got %v, want 100", got)
	}
	if got := aneOnOffLabel(aneUtilizationPercent(on)); got != "ON" {
		t.Fatalf("exclave powered label: got %q, want ON", got)
	}
	// Crucially, the exclave path must not latch the GB/s label — otherwise the
	// gauge falls through to a %-form template and prints the binary value as %.
	if aneBWModeLatched.Load() || aneBWLabelMode(on) {
		t.Fatal("exclave path must not latch bandwidth mode")
	}

	// Idle: reads 0 and "idle".
	resetANETestState(t)
	off := CPUMetrics{ANEExclave: true, ANEPowered: true, ANEActive: 0}
	if got := aneUtilizationPercent(off); got != 0 {
		t.Fatalf("exclave idle: got %v, want 0", got)
	}
	if got := aneOnOffLabel(aneUtilizationPercent(off)); got != "idle" {
		t.Fatalf("exclave idle label: got %q, want idle", got)
	}
}
