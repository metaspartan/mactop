package app

import (
	"testing"
)

func TestMemoryPressureState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level int
		want  string
	}{
		{MemoryPressureLevelNormal, MemoryPressureStateNormal},
		{MemoryPressureLevelWarning, MemoryPressureStateWarning},
		{MemoryPressureLevelCritical, MemoryPressureStateCritical},
		{0, MemoryPressureStateUnknown},
		{3, MemoryPressureStateWarning},
		{8, MemoryPressureStateCritical},
	}
	for _, tt := range tests {
		if got := memoryPressureState(tt.level); got != tt.want {
			t.Fatalf("memoryPressureState(%d)=%q want %q", tt.level, got, tt.want)
		}
	}
}

func TestMemoryPressureSeverityAndColor(t *testing.T) {
	t.Parallel()
	if memoryPressureSeverity(1) != 0 || memoryPressureColorName(1) != "green" {
		t.Fatal("normal severity/color mismatch")
	}
	if memoryPressureSeverity(2) != 1 || memoryPressureColorName(2) != "yellow" {
		t.Fatal("warning severity/color mismatch")
	}
	if memoryPressureSeverity(4) != 2 || memoryPressureColorName(4) != "red" {
		t.Fatal("critical severity/color mismatch")
	}
	if memoryPressureSeverity(0) != -1 || memoryPressureColorName(0) != "white" {
		t.Fatal("unknown severity/color mismatch")
	}
}

func TestMemoryPressureApproxBands(t *testing.T) {
	t.Parallel()
	// High residency with NORMAL pressure should stay well below warning band.
	normalHighUse := memoryPressureApprox(1, 70*1024*1024*1024, 96*1024*1024*1024, 0, 2*1024*1024*1024)
	if normalHighUse >= 50 {
		t.Fatalf("normal high-use approx too high: %.1f", normalHighUse)
	}

	warn := memoryPressureApprox(2, 40*1024*1024*1024, 96*1024*1024*1024, 4*1024*1024*1024, 8*1024*1024*1024)
	if warn < 50 || warn > 85 {
		t.Fatalf("warning approx out of band: %.1f", warn)
	}

	crit := memoryPressureApprox(4, 90*1024*1024*1024, 96*1024*1024*1024, 20*1024*1024*1024, 20*1024*1024*1024)
	if crit < 80 {
		t.Fatalf("critical approx too low: %.1f", crit)
	}

	if clampPressureApprox(-5) != 0 || clampPressureApprox(150) != 100 {
		t.Fatal("clampPressureApprox bounds failed")
	}
}

func TestMemoryPressureGaugePercent(t *testing.T) {
	t.Parallel()
	if memoryPressureGaugePercent(1) != 25 {
		t.Fatal("normal gauge percent")
	}
	if memoryPressureGaugePercent(2) != 65 {
		t.Fatal("warning gauge percent")
	}
	if memoryPressureGaugePercent(4) != 95 {
		t.Fatal("critical gauge percent")
	}
	if memoryPressureGaugePercent(0) != 0 {
		t.Fatal("unknown gauge percent")
	}
}

func TestLayoutMemoryInOrder(t *testing.T) {
	found := false
	for _, layout := range layoutOrder {
		if layout == LayoutMemory {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("LayoutMemory missing from layoutOrder")
	}
}

func TestGetMemoryMetricsIncludesPressure(t *testing.T) {
	m := getMemoryMetrics()
	if m.Total == 0 {
		t.Fatal("expected total memory")
	}
	if m.PressureState == "" {
		t.Fatal("expected pressure state")
	}
	if m.PressureApprox < 0 || m.PressureApprox > 100 {
		t.Fatalf("pressure approx out of range: %v", m.PressureApprox)
	}
	// Live systems typically report NORMAL=1; accept known levels.
	switch m.PressureLevel {
	case 0, 1, 2, 4:
	default:
		// Intermediate values are mapped by helpers; still allow non-zero.
		if m.PressureLevel < 0 {
			t.Fatalf("unexpected pressure level: %d", m.PressureLevel)
		}
	}
}
