package app

import (
	"testing"
	"time"
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

func TestMax(t *testing.T) {
	tests := []struct {
		name string
		nums []int
		want int
	}{
		{"Single positive", []int{5}, 5},
		{"Multiple positive", []int{1, 5, 3}, 5},
		{"Negative numbers", []int{-1, -5, -3}, -1},
		{"Mixed numbers", []int{-5, 0, 5}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := max(tt.nums...); got != tt.want {
				t.Errorf("max() = %v, want %v", got, tt.want)
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

	totalFromWidget := w.eCoreCount + w.pCoreCount
	if totalFromWidget == 0 {
		t.Error("Expected non-zero core counts")
	}
	if len(w.cores) != totalFromWidget {
		t.Errorf("Expected len(cores) %d to match eCoreCount+pCoreCount, got %d", totalFromWidget, len(w.cores))
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
