package app

import (
	"math"
)

// Kernel memorystatus_vm_pressure_level values (XNU memorystatus).
// Documented as bit-style levels: 1=NORMAL, 2=WARN, 4=CRITICAL.
const (
	MemoryPressureLevelNormal   = 1
	MemoryPressureLevelWarning  = 2
	MemoryPressureLevelCritical = 4
)

const (
	MemoryPressureStateNormal   = "Normal"
	MemoryPressureStateWarning  = "Warning"
	MemoryPressureStateCritical = "Critical"
	MemoryPressureStateUnknown  = "Unknown"
)

// memoryPressureState maps a kernel pressure level to an Activity Monitor-style label.
func memoryPressureState(level int) string {
	switch level {
	case MemoryPressureLevelNormal:
		return MemoryPressureStateNormal
	case MemoryPressureLevelWarning:
		return MemoryPressureStateWarning
	case MemoryPressureLevelCritical:
		return MemoryPressureStateCritical
	default:
		// Some kernels may report intermediate values; treat high bits as critical.
		if level >= MemoryPressureLevelCritical {
			return MemoryPressureStateCritical
		}
		if level >= MemoryPressureLevelWarning {
			return MemoryPressureStateWarning
		}
		if level > 0 {
			return MemoryPressureStateNormal
		}
		return MemoryPressureStateUnknown
	}
}

// memoryPressureSeverity ranks pressure for gauges/colors: 0 normal, 1 warning, 2 critical, -1 unknown.
func memoryPressureSeverity(level int) int {
	switch memoryPressureState(level) {
	case MemoryPressureStateNormal:
		return 0
	case MemoryPressureStateWarning:
		return 1
	case MemoryPressureStateCritical:
		return 2
	default:
		return -1
	}
}

// memoryPressureColorName returns a terminal color name for the pressure state.
func memoryPressureColorName(level int) string {
	switch memoryPressureSeverity(level) {
	case 0:
		return "green"
	case 1:
		return "yellow"
	case 2:
		return "red"
	default:
		return "white"
	}
}

// memoryPressureApprox builds a 0-100 history signal.
// Apple does not publish Activity Monitor's exact formula; this uses the discrete
// kernel level as the primary signal and nudges with used/swap/compression fractions
// so the chart is smooth and useful for MLX/LLM-style high-usage, low-pressure cases.
func memoryPressureApprox(level int, used, total, swapUsed, compressed uint64) float64 {
	var base float64
	switch memoryPressureSeverity(level) {
	case 0:
		base = 18.0
	case 1:
		base = 62.0
	case 2:
		base = 90.0
	default:
		base = 25.0
	}
	if total == 0 {
		return clampPressureApprox(base)
	}

	usedFrac := float64(used) / float64(total)
	swapFrac := float64(swapUsed) / float64(total)
	compFrac := float64(compressed) / float64(total)

	// Keep usage/swap as secondary so high Apple Silicon residency stays green
	// while the kernel level is still NORMAL.
	score := base*0.72 +
		usedFrac*100.0*0.14 +
		math.Min(swapFrac*100.0, 45.0)*0.09 +
		math.Min(compFrac*100.0, 45.0)*0.05

	return clampPressureApprox(score)
}

func clampPressureApprox(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// memoryPressureGaugePercent maps discrete kernel levels to a gauge fill.
func memoryPressureGaugePercent(level int) int {
	switch memoryPressureSeverity(level) {
	case 0:
		return 25
	case 1:
		return 65
	case 2:
		return 95
	default:
		return 0
	}
}
