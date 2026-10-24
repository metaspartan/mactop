// Copyright (c) 2024-2026 Carsen Klock under MIT License
// history_charts.go - history chart and series rendering for the TUI
package app

import (
	"fmt"
	"math"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/mactop/v2/internal/i18n"
)

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

// renderDualANEClusterChart draws the per-die ANE0/ANE1 traces for history_soc.
// It only applies on the power-state tiers (shouldRenderDualANEClusters): on a
// multi-die chip with a live PMP/AMC channel the gauge shows residency/bandwidth
// %, so plotting the IORegistry power-state duty here would diverge — return
// false to fall through to the gauge-consistent single-series path.
func renderDualANEClusterChart(cpuMetrics CPUMetrics, visibleWidth int, maxVal, cluster0, cluster1 float64) bool {
	if !isHistorySoCLayout() || !shouldRenderDualANEClusters(cpuMetrics) || len(aneCluster0History) == 0 {
		return false
	}
	visibleC0 := aneCluster0History[len(aneCluster0History)-visibleWidth:]
	visibleC1 := aneCluster1History[len(aneCluster1History)-visibleWidth:]
	for _, series := range [][]float64{visibleC0, visibleC1} {
		for _, v := range series {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	scaleMax := 100.0
	if maxVal <= 25.0 {
		scaleMax = 25.0
	} else if maxVal <= 50.0 {
		scaleMax = 50.0
	}
	displayC0, displayC1 := staggerANEClusterChartSeries(visibleC0, visibleC1, scaleMax)
	aneColor := historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, ui.ColorRed)
	aneHistoryChart.Data = [][]float64{displayC0, displayC1}
	aneHistoryChart.LineColors = []ui.Color{aneColor, aneColor}
	nClusters := cpuMetrics.ANEClusterCount
	if nClusters < 2 {
		nClusters = len(cpuMetrics.ANEClusterActive)
	}
	title, label0, label1 := formatDualANEClusterChartText(cluster0, cluster1, aneClusterLabelModeFor(cpuMetrics), nClusters)
	aneHistoryChart.Title = title
	aneHistoryChart.DataLabels = []string{label0, label1}
	aneHistoryChart.MaxVal = scaleMax
	return true
}

// renderPowerStateANEChart draws the single power-domain trace for the
// ANEExclave / ANEPowered tiers, where a utilization % or wattage would be
// misleading (exclave is binary ON/idle; the ANEPowered fallback leaves ANEW at
// 0). The trace is the 0/100 power series with an ON/idle or powered/idle label.
// Returns false when not a power-state tier, leaving the %-form path to the
// caller. Assumes aneHistoryChart.Data is already set to the visible series.
func renderPowerStateANEChart(cpuMetrics CPUMetrics, anePct, aneWatts, aneBW float64, bwMode bool, scaleMax float64) bool {
	if !cpuMetrics.ANEExclave && !(cpuMetrics.ANEPowered && !bwMode) {
		return false
	}
	state := anePoweredLabel(anePct)
	if cpuMetrics.ANEExclave {
		state = aneOnOffLabel(anePct)
	}
	aneColor := ui.ColorRed
	if !isHistorySoCLayout() {
		aneColor = ui.ColorMagenta
	}
	aneHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, aneColor)}
	aneHistoryChart.DataLabels = []string{state}
	aneHistoryChart.Title = aneChartTitle(cpuMetrics, anePct, 0, aneWatts, aneBW, bwMode, isHistorySoCLayout())
	aneHistoryChart.MaxVal = scaleMax
	return true
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

	// Per-die dual-cluster trace (only on the power-state tiers — see the helper)
	// and the power-state single trace are gauge-consistent early returns.
	if renderDualANEClusterChart(cpuMetrics, visibleWidth, maxVal, cluster0, cluster1) {
		return
	}

	aneHistoryChart.Data = [][]float64{visibleRaw}
	if renderPowerStateANEChart(cpuMetrics, anePct, aneWatts, aneBW, bwMode, scaleMax) {
		return
	}
	aneHistoryChart.DataLabels = []string{fmt.Sprintf("%.1f%%", anePct)}
	if isHistorySoCLayout() {
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
		aneHistoryChart.Title = aneChartTitle(cpuMetrics, anePct, currentPeak, aneWatts, aneBW, bwMode, true)
	} else {
		aneHistoryChart.LineColors = []ui.Color{historyLineColor(func(t *CustomThemeConfig) string { return t.ANE }, ui.ColorMagenta)}
		aneHistoryChart.Title = aneChartTitle(cpuMetrics, anePct, 0, aneWatts, aneBW, bwMode, false)
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
		if cols := historySoCBottomColumns(); cols > 0 {
			// Match the bottom-row column count (3 on history_soc, 2 on
			// history_soc_mem) so the time window lines up with the panel.
			visibleWidth = (termWidth / cols) - 4
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
		if isHistorySoCLayout() {
			visibleTotal = make([]float64, len(visibleRead))
			for i := range visibleRead {
				visibleTotal[i] = visibleRead[i] + visibleWrite[i]
			}
			scaleSeries = append(scaleSeries, visibleTotal)
		}
		scaleMax := bandwidthScaleMax(scaleSeries...)

		// In history_soc layout, force a minimum visible scale so the graph
		// doesn't look completely dead when bandwidth is low (common even with high GPU/ANE load)
		if isHistorySoCLayout() && scaleMax < 8.0 {
			scaleMax = 8.0
		}

		if isHistorySoCLayout() {
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
		if isHistorySoCLayout() {
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
