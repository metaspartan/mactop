package app

import (
	"fmt"
	"image"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
)

type CPUUsage struct {
	User   float64
	System float64
	Idle   float64
	Nice   float64
}

type CPUMetrics struct {
	EClusterActive, EClusterFreqMHz, PClusterActive, PClusterFreqMHz int
	SClusterActive, SClusterFreqMHz                                  int
	ECores, PCores, SCores                                           []int
	CoreMetrics                                                      map[string]int
	ANEW, CPUW, GPUW, DRAMW, GPUSRAMW, PackageW, SystemW             float64
	ANEActive                                                        float64
	ANEReadBW                                                        float64
	ANEWriteBW                                                       float64
	CoreUsages                                                       []float64
	AvgUsage                                                         float64
	Throttled                                                        bool
	CPUTemp                                                          float64
	GPUTemp                                                          float64
	DRAMReadBW                                                       float64
	DRAMWriteBW                                                      float64
	DRAMBWCombined                                                   float64
	DRAMBandwidthSource                                              DRAMBandwidthSource
	ANEBW                                                            float64
	Fans                                                             []FanInfo
	TempSensors                                                      []TempSensor
}

// DRAMBandwidthSource states whether DRAM directions are measured or derived.
// The zero value preserves legacy callers while native samples use unavailable
// until a source becomes usable.
type DRAMBandwidthSource string

const (
	DRAMBandwidthUnavailable   DRAMBandwidthSource = "unavailable"
	DRAMBandwidthDirectional   DRAMBandwidthSource = "directional"
	DRAMBandwidthCombined      DRAMBandwidthSource = "combined"
	DRAMBandwidthPowerEstimate DRAMBandwidthSource = "power_estimate"
)

func (s DRAMBandwidthSource) IsNonDirectional() bool {
	return s == DRAMBandwidthCombined || s == DRAMBandwidthPowerEstimate
}

func (s DRAMBandwidthSource) IsAvailable() bool {
	// Empty is retained as directional for pre-source callers and tests. Native
	// sampling always returns the explicit unavailable value before calibration.
	return s != DRAMBandwidthUnavailable
}

type SystemInfo struct {
	Name         string `json:"name"`
	CoreCount    int    `json:"core_count"`
	ECoreCount   int    `json:"e_core_count,omitempty"`
	PCoreCount   int    `json:"p_core_count"`
	SCoreCount   int    `json:"s_core_count,omitempty"`
	GPUCoreCount int    `json:"gpu_core_count"`
}

type NetDiskMetrics struct {
	OutPacketsPerSec  float64 `json:"out_packets_per_sec"`
	OutBytesPerSec    float64 `json:"out_bytes_per_sec"`
	InPacketsPerSec   float64 `json:"in_packets_per_sec"`
	InBytesPerSec     float64 `json:"in_bytes_per_sec"`
	ReadOpsPerSec     float64 `json:"read_ops_per_sec"`
	WriteOpsPerSec    float64 `json:"write_ops_per_sec"`
	ReadKBytesPerSec  float64 `json:"read_kbytes_per_sec"`
	WriteKBytesPerSec float64 `json:"write_kbytes_per_sec"`
}

type GPUMetrics struct {
	FreqMHz       int
	ActivePercent float64
	EffectiveLoad float64 // Frequency-adjusted load: Active% * (current / max)
	Power         float64
	Temp          float32
}

type ProcessMetrics struct {
	PID                                      int
	CPU, LastTime, Memory, GPU               float64 // GPU is ms/s of GPU time
	VSZ, RSS, Footprint                      int64
	User, TTY, State, Started, Time, Command string
	LastUpdated                              time.Time
}

type MemoryMetrics struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
	SwapTotal uint64 `json:"swap_total"`
	SwapUsed  uint64 `json:"swap_used"`
}

type EventThrottler struct {
	pending     atomic.Bool
	gracePeriod time.Duration
	C           chan struct{}
}

// PeakStepChart augments StepChart with labels placed beside the actual maximum
// sample of each rendered series. The regular DataLabels remain current values
// at the right edge, while peak labels are opt-in for the unified dashboard.
type PeakStepChart struct {
	*w.StepChart
	PeakLabels        []string
	ShowPeakLabels    bool
	CurrentLabelOrder []int    // Optional draw priority for right-side current labels.
	SeriesGroups      []string // Optional unit groups; equal groups sort labels by value.
}

func NewPeakStepChart() *PeakStepChart {
	return &PeakStepChart{StepChart: w.NewStepChart()}
}

func (sc *PeakStepChart) Draw(buf *ui.Buffer) {
	// StepChart draws every current label at its sample's Y coordinate. That
	// makes coincident series overwrite each other, so reserve those labels for
	// our collision-aware pass after the lines and peak labels are rendered.
	labels := sc.DataLabels
	if sc.ShowRightAxis {
		sc.DataLabels = make([]string, len(sc.Data))
	}
	sc.StepChart.Draw(buf)
	sc.DataLabels = labels
	if !sc.ShowPeakLabels || len(sc.PeakLabels) == 0 {
		sc.drawCurrentLabels(buf, labels)
		return
	}
	sc.drawPeakLabels(buf)
	sc.drawCurrentLabels(buf, labels)
}

type chartLabelPlacement struct {
	text string
	rect image.Rectangle
	line int
}

func (sc *PeakStepChart) drawCurrentLabels(buf *ui.Buffer, labels []string) {
	if !sc.ShowRightAxis || len(sc.Data) == 0 {
		return
	}

	area := sc.currentLabelDrawArea()
	if area.Dx() <= 0 || area.Dy() <= 0 {
		return
	}
	maxVal := sc.MaxVal
	if maxVal <= 0 {
		for _, series := range sc.Data {
			maxVal = max(maxVal, seriesMax(series))
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	// Resolve same-unit labels from low to high. The lower value claims its
	// natural row first; a colliding higher value is then displaced upward,
	// preserving the final top-to-bottom numeric order at the chart bottom.
	order := sc.currentLabelOrder()
	occupied := make([]image.Rectangle, 0, len(sc.Data))
	groupCeiling := make(map[string]int, len(sc.Data))
	for _, lineIdx := range order {
		series := sc.Data[lineIdx]
		if len(series) == 0 {
			continue
		}
		label := fmt.Sprintf("%.2f", series[len(series)-1])
		if lineIdx < len(labels) && labels[lineIdx] != "" {
			label = labels[lineIdx]
		}
		idealY := area.Max.Y - 1 - int((series[len(series)-1]/maxVal)*float64(area.Dy()-1))
		idealY = max(area.Min.Y, min(idealY, area.Max.Y-1))
		ceiling, ok := groupCeiling[sc.seriesGroup(lineIdx)]
		if !ok {
			ceiling = area.Max.Y - 1
		}
		placement := chooseCurrentLabelPlacementWithin(area, label, idealY, lineIdx, occupied, ceiling, func(rect image.Rectangle) int {
			return chartLineCells(buf, rect)
		})
		occupied = append(occupied, placement.rect)
		// Same-unit labels are placed low to high, so every next label must be
		// physically above the previous one, even when line avoidance moves it.
		groupCeiling[sc.seriesGroup(lineIdx)] = placement.rect.Min.Y - 1
		style := ui.NewStyle(ui.SelectColor(sc.LineColors, lineIdx), sc.BorderStyle.Bg)
		buf.SetString(label, style, placement.rect.Min)
	}
}

func (sc *PeakStepChart) currentLabelOrder() []int {
	base := currentLabelOrder(len(sc.Data))
	if len(sc.CurrentLabelOrder) == len(sc.Data) {
		seen := make([]bool, len(sc.Data))
		for _, idx := range sc.CurrentLabelOrder {
			if idx < 0 || idx >= len(sc.Data) || seen[idx] {
				return sc.orderCurrentLabelsByGroup(base)
			}
			seen[idx] = true
		}
		base = sc.CurrentLabelOrder
	}
	return sc.orderCurrentLabelsByGroup(base)
}

func (sc *PeakStepChart) orderCurrentLabelsByGroup(base []int) []int {
	result := make([]int, 0, len(base))
	usedGroups := make(map[string]bool, len(base))
	for _, first := range base {
		group := sc.seriesGroup(first)
		if usedGroups[group] {
			continue
		}
		usedGroups[group] = true
		members := make([]int, 0, len(base))
		for _, idx := range base {
			if sc.seriesGroup(idx) == group {
				members = append(members, idx)
			}
		}
		sort.SliceStable(members, func(i, j int) bool {
			left, right := sc.Data[members[i]], sc.Data[members[j]]
			leftValue, rightValue := 0.0, 0.0
			if len(left) > 0 {
				leftValue = left[len(left)-1]
			}
			if len(right) > 0 {
				rightValue = right[len(right)-1]
			}
			return leftValue < rightValue
		})
		result = append(result, members...)
	}
	return result
}

func (sc *PeakStepChart) seriesGroup(index int) string {
	if index >= 0 && index < len(sc.SeriesGroups) && sc.SeriesGroups[index] != "" {
		return sc.SeriesGroups[index]
	}
	return "default"
}

func (sc *PeakStepChart) currentLabelDrawArea() image.Rectangle {
	if sc.ShowAxes {
		return image.Rect(sc.Inner.Min.X+5, sc.Inner.Min.Y, sc.Inner.Max.X, sc.Inner.Max.Y-2)
	}
	return sc.Inner
}

func currentLabelOrder(seriesCount int) []int {
	order := make([]int, 0, seriesCount)
	if seriesCount < 4 {
		for idx := 0; idx < seriesCount; idx++ {
			order = append(order, idx)
		}
		return order
	}
	// The mixed unified memory chart has memory/swap in 0/1 and DRAM in 2/3.
	for _, idx := range []int{2, 3} {
		if idx < seriesCount {
			order = append(order, idx)
		}
	}
	for idx := 0; idx < seriesCount; idx++ {
		if idx < 2 {
			order = append(order, idx)
		}
	}
	return order
}

func chooseCurrentLabelPlacement(area image.Rectangle, label string, idealY, line int, occupied []image.Rectangle, lineCells func(image.Rectangle) int) chartLabelPlacement {
	return chooseCurrentLabelPlacementWithin(area, label, idealY, line, occupied, area.Max.Y-1, lineCells)
}

func chooseCurrentLabelPlacementWithin(area image.Rectangle, label string, idealY, line int, occupied []image.Rectangle, maxY int, _ func(image.Rectangle) int) chartLabelPlacement {
	width := runewidth.StringWidth(label)
	if width <= 0 {
		return chartLabelPlacement{line: line}
	}
	// Reserve the rightmost chart cell for the latest plotted sample.
	xs := []int{max(area.Min.X, area.Max.X-1-width)}
	for distance := 0; distance < area.Dy(); distance++ {
		for _, y := range []int{idealY + distance, idealY - distance} {
			if y < area.Min.Y || y >= area.Max.Y || y > maxY || (distance == 0 && y != idealY) {
				continue
			}
			for _, x := range xs {
				rect := image.Rect(x, y, x+width, y+1)
				if overlapsAny(rect, occupied) {
					continue
				}
				return chartLabelPlacement{text: label, rect: rect, line: line}
			}
		}
	}
	// A one-row chart cannot fit all labels. Keep the high-priority label
	// readable even when the terminal leaves no non-overlapping placement.
	fallbackY := max(area.Min.Y, min(idealY, maxY))
	return chartLabelPlacement{text: label, rect: image.Rect(xs[0], fallbackY, xs[0]+width, fallbackY+1), line: line}
}

func overlapsAny(candidate image.Rectangle, occupied []image.Rectangle) bool {
	for _, used := range occupied {
		if candidate.Overlaps(used) {
			return true
		}
	}
	return false
}

func chartLineCells(buf *ui.Buffer, rect image.Rectangle) int {
	count := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			switch buf.GetCell(image.Pt(x, y)).Rune {
			case ui.HORIZONTAL_LINE, ui.VERTICAL_LINE, ui.TOP_RIGHT, ui.BOTTOM_RIGHT, ui.TOP_LEFT, ui.BOTTOM_LEFT:
				count++
			}
		}
	}
	return count
}

func (sc *PeakStepChart) drawPeakLabels(buf *ui.Buffer) {
	if len(sc.Data) == 0 {
		return
	}
	maxVal := sc.MaxVal
	if maxVal <= 0 {
		for _, series := range sc.Data {
			maxVal = max(maxVal, seriesMax(series))
		}
	}
	if maxVal <= 0 {
		return
	}

	drawArea := sc.Inner
	if sc.ShowAxes {
		drawArea = image.Rect(sc.Inner.Min.X+5, sc.Inner.Min.Y, sc.Inner.Max.X, sc.Inner.Max.Y-2)
	}
	if drawArea.Dx() <= 0 || drawArea.Dy() <= 0 {
		return
	}
	// Match StepChart.drawLine: a sample is visible only while its x position
	// is strictly inside drawArea's right edge.
	scale := max(sc.HorizontalScale, 1)
	maxSamples := min((drawArea.Dx()-1)/scale+1, len(sc.Data[0]))
	type peakLabel struct {
		line  int
		value float64
		y     int
		text  string
	}
	labels := make([]peakLabel, 0, len(sc.Data))
	for lineIdx, series := range sc.Data {
		if lineIdx >= len(sc.PeakLabels) || sc.PeakLabels[lineIdx] == "" || len(series) == 0 {
			continue
		}
		visible := series[:min(maxSamples, len(series))]
		peak := visible[0]
		for _, value := range visible[1:] {
			if value > peak {
				peak = value
			}
		}

		y := drawArea.Max.Y - 1 - int((peak/maxVal)*float64(drawArea.Dy()-1))
		y = max(drawArea.Min.Y, min(y, drawArea.Max.Y-1))
		text := sc.PeakLabels[lineIdx]
		if runewidth.StringWidth(text) <= 0 {
			continue
		}
		labels = append(labels, peakLabel{line: lineIdx, value: peak, y: y, text: text})
	}

	// Resolve collisions from the lowest peak up. At the bottom edge a lower
	// peak can retain its natural row while each higher collision moves upward,
	// preserving the final top-to-bottom numeric order.
	sort.SliceStable(labels, func(i, j int) bool {
		if labels[i].value == labels[j].value {
			return labels[i].line < labels[j].line
		}
		return labels[i].value < labels[j].value
	})
	usedRows := make(map[int]bool, len(labels))
	for _, label := range labels {
		if drawArea.Min.X+runewidth.StringWidth(label.text) > drawArea.Max.X {
			continue
		}
		y, ok := nearestPeakLabelRow(label.y, drawArea, usedRows)
		if !ok {
			// There are more peak labels than display rows. Keep the label in the
			// left column rather than moving it horizontally.
			y = label.y
		}
		usedRows[y] = true
		style := ui.NewStyle(ui.SelectColor(sc.LineColors, label.line), sc.BorderStyle.Bg)
		buf.SetString(label.text, style, image.Pt(drawArea.Min.X, y))
	}
}

func nearestPeakLabelRow(target int, area image.Rectangle, used map[int]bool) (int, bool) {
	for distance := 0; distance < area.Dy(); distance++ {
		if distance == 0 {
			if !used[target] {
				return target, true
			}
			continue
		}
		// Higher labels are placed after their lower colliding peers, so prefer
		// the row above to retain descending values from top to bottom.
		for _, y := range []int{target - distance, target + distance} {
			if y >= area.Min.Y && y < area.Max.Y && !used[y] {
				return y, true
			}
		}
	}
	return 0, false
}

type CPUCoreWidget struct {
	*ui.Block
	cores                              []float64
	labels                             []string
	eCoreCount, pCoreCount, sCoreCount int
	modelName                          string
	cpuIndexMap                        []int // maps display index -> hardware CPU index
	groupByType                        bool
}

func NewEventThrottler(gracePeriod time.Duration) *EventThrottler {
	return &EventThrottler{
		gracePeriod: gracePeriod,
		C:           make(chan struct{}, 1),
	}
}

func NewCPUMetrics() CPUMetrics {
	return CPUMetrics{
		CoreMetrics: make(map[string]int),
		ECores:      make([]int, 0),
		PCores:      make([]int, 0),
		SCores:      make([]int, 0),
	}
}

func (e *EventThrottler) Notify() {
	// CAS so only one grace window can be armed at a time: the previous
	// unsynchronized timer-pointer check raced with the callback goroutine
	// (caught by go test -race) and could double-arm on concurrent first
	// notifications. Clearing pending before the send preserves the old
	// ordering: a Notify arriving between the two re-arms a fresh window.
	if !e.pending.CompareAndSwap(false, true) {
		return
	}

	time.AfterFunc(e.gracePeriod, func() {
		e.pending.Store(false)
		select {
		case e.C <- struct{}{}:
		default:
		}
	})
}

func NewCPUCoreWidget(modelInfo SystemInfo) *CPUCoreWidget {
	modelName := modelInfo.Name

	// Use dynamic core topology detection from IORegistry
	labels, eCount, pCount, sCount, cpuIndexMap := BuildCoreLabels()

	if len(labels) == 0 {
		// Fallback to sysctl-based counts (old behavior)
		eCoreCount := modelInfo.ECoreCount
		pCoreCount := modelInfo.PCoreCount
		sCoreCount := modelInfo.SCoreCount
		totalCores := eCoreCount + pCoreCount + sCoreCount

		labels = make([]string, totalCores)
		cpuIndexMap = make([]int, totalCores)
		idx := 0
		for i := range eCoreCount {
			labels[idx] = fmt.Sprintf("E%d", i)
			cpuIndexMap[idx] = idx
			idx++
		}
		for i := range pCoreCount {
			labels[idx] = fmt.Sprintf("P%d", i)
			cpuIndexMap[idx] = idx
			idx++
		}
		for i := range sCoreCount {
			labels[idx] = fmt.Sprintf("S%d", i)
			cpuIndexMap[idx] = idx
			idx++
		}
		eCount = eCoreCount
		pCount = pCoreCount
		sCount = sCoreCount
	}

	totalCores := len(labels)

	return &CPUCoreWidget{
		Block:       ui.NewBlock(),
		cores:       make([]float64, totalCores),
		labels:      labels,
		eCoreCount:  eCount,
		pCoreCount:  pCount,
		sCoreCount:  sCount,
		modelName:   modelName,
		cpuIndexMap: cpuIndexMap,
	}
}

func (w *CPUCoreWidget) UpdateUsage(usage []float64) {
	// Remap usage data from hardware order to display order (E cores first, then P)
	if len(w.cpuIndexMap) > 0 {
		w.cores = make([]float64, len(w.cpuIndexMap))
		for displayIdx, cpuIdx := range w.cpuIndexMap {
			if cpuIdx < len(usage) {
				w.cores[displayIdx] = usage[cpuIdx]
			}
		}
	} else {
		// No remapping needed
		w.cores = make([]float64, len(usage))
		copy(w.cores, usage)
	}
}

func (w *CPUCoreWidget) calculateLayout(availableWidth, availableHeight, totalCores int) (int, int, []int, []int) {
	cols := 4
	if totalCores > 16 {
		cols = 8
	}
	minColWidth := 20
	if (availableWidth / cols) < minColWidth {
		cols = max(1, availableWidth/minColWidth)
	}
	rows := (totalCores + cols - 1) / cols
	if rows > availableHeight {
		rows = availableHeight
		if rows == 0 {
			rows = 1
		}
		cols = (totalCores + rows - 1) / rows
		rows = (totalCores + cols - 1) / cols
	}

	colWidths := make([]int, cols)
	colXs := make([]int, cols)
	baseWidth := availableWidth / cols
	remainder := availableWidth % cols
	currentX := 0
	for c := 0; c < cols; c++ {
		colXs[c] = currentX
		w := baseWidth
		if c < remainder {
			w++
		}
		colWidths[c] = w
		currentX += w
	}
	return cols, rows, colXs, colWidths
}

func (w *CPUCoreWidget) drawCore(buf *ui.Buffer, x, y, barWidth, index int, usage float64, themeColor ui.Color) {
	labelWidth := 3
	label := fmt.Sprintf("%d", index)
	if index < len(w.labels) {
		label = w.labels[index]
	}
	if len(label) < labelWidth {
		label = fmt.Sprintf("%-*s", labelWidth, label)
	}
	buf.SetString(label, ui.NewStyle(themeColor, CurrentBgColor), image.Pt(x, y))

	availWidth := barWidth - labelWidth
	if x+labelWidth+availWidth > w.Inner.Max.X {
		availWidth = w.Inner.Max.X - x - labelWidth
	}

	if availWidth < 9 {
		return
	}

	textWidth := 7
	innerBarWidth := max(availWidth-2-textWidth, 0)
	usedWidth := int((usage / 100.0) * float64(innerBarWidth))

	buf.SetString("[", ui.NewStyle(BracketColor, CurrentBgColor), image.Pt(x+labelWidth, y))

	for bx := range innerBarWidth {
		char := " "
		var color ui.Color
		if bx < usedWidth {
			char = "❚"
			switch {
			case usage >= 60:
				color = ui.ColorRed
			case usage >= 40:
				color = ui.ColorYellow
			case usage >= 30:
				color = ui.ColorSkyBlue
			default:
				color = themeColor
			}
		} else {
			color = themeColor
		}
		buf.SetString(char, ui.NewStyle(color, CurrentBgColor), image.Pt(x+labelWidth+1+bx, y))
	}

	percentage := fmt.Sprintf("%5.1f%%", usage)
	buf.SetString(percentage, ui.NewStyle(SecondaryTextColor, CurrentBgColor), image.Pt(x+labelWidth+1+innerBarWidth, y))
	buf.SetString("]", ui.NewStyle(BracketColor, CurrentBgColor), image.Pt(x+labelWidth+availWidth-1, y))
}

func (w *CPUCoreWidget) Draw(buf *ui.Buffer) {
	w.Block.Draw(buf)
	if len(w.cores) == 0 {
		return
	}
	themeColor := w.BorderStyle.Fg
	if w.groupByType && w.drawGroupedCores(buf, themeColor) {
		return
	}
	w.drawCores(buf, themeColor, 0, len(w.cores), w.Inner)
}

type cpuCoreGroup struct {
	label      string
	start, end int
}

func (w *CPUCoreWidget) coreGroups() []cpuCoreGroup {
	groups := make([]cpuCoreGroup, 0, 3)
	start := 0
	for _, tier := range []struct {
		label string
		count int
	}{
		{"E", w.eCoreCount},
		{"P", w.pCoreCount},
		{"S", w.sCoreCount},
	} {
		if tier.count <= 0 {
			continue
		}
		end := min(start+tier.count, len(w.cores))
		if end > start {
			groups = append(groups, cpuCoreGroup{
				label: fmt.Sprintf("%s %d", tier.label, end-start),
				start: start,
				end:   end,
			})
		}
		start = end
	}
	return groups
}

func (w *CPUCoreWidget) drawGroupedCores(buf *ui.Buffer, themeColor ui.Color) bool {
	groups := w.coreGroups()
	if len(groups) <= 1 || w.Inner.Dx() < len(groups)*14 || w.Inner.Dy() < 4 {
		return false
	}

	gapWidth := len(groups) - 1
	availableWidth := w.Inner.Dx() - gapWidth
	totalCores := len(w.cores)
	x := w.Inner.Min.X
	remainingWidth := availableWidth

	for i, group := range groups {
		groupCores := group.end - group.start
		groupWidth := remainingWidth
		if i < len(groups)-1 {
			groupWidth = max(14, availableWidth*groupCores/totalCores)
			groupWidth = min(groupWidth, remainingWidth-14*(len(groups)-i-1))
		}
		if groupWidth <= 0 {
			return false
		}

		area := image.Rect(x, w.Inner.Min.Y+1, x+groupWidth, w.Inner.Max.Y)
		buf.SetString(group.label, ui.NewStyle(SecondaryTextColor, CurrentBgColor), image.Pt(x, w.Inner.Min.Y))
		w.drawCores(buf, themeColor, group.start, group.end, area)

		x += groupWidth + 1
		remainingWidth -= groupWidth
	}
	return true
}

func (w *CPUCoreWidget) drawCores(buf *ui.Buffer, themeColor ui.Color, start, end int, area image.Rectangle) {
	totalCores := end - start
	if totalCores <= 0 {
		return
	}
	cols, rows, colXs, colWidths := w.calculateLayout(area.Dx(), area.Dy(), totalCores)
	fullCols := totalCores - (rows-1)*cols

	for i := range totalCores {
		col := i % cols
		row := i / cols
		actualIndex := start + col*rows + row - max(0, col-fullCols)

		if actualIndex >= end || row >= rows {
			continue
		}

		x := area.Min.X + colXs[col]
		y := area.Min.Y + row

		if y >= area.Max.Y {
			continue
		}

		w.drawCore(buf, x, y, colWidths[col], actualIndex, w.cores[actualIndex], themeColor)
	}
}

// FormatCoreSummary builds a dynamic core summary string like "(6E/4P)" or "(12P/6S)"
// Only includes core types that have non-zero counts.
func FormatCoreSummary(eCount, pCount, sCount int) string {
	var parts []string
	if eCount > 0 {
		parts = append(parts, fmt.Sprintf("%dE", eCount))
	}
	if pCount > 0 {
		parts = append(parts, fmt.Sprintf("%dP", pCount))
	}
	if sCount > 0 {
		parts = append(parts, fmt.Sprintf("%dS", sCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return "(" + strings.Join(parts, "/") + ")"
}
