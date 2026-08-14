package app

import (
	"fmt"
	"image"
	"strings"
	"sync/atomic"
	"time"

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
	ANEBW                                                            float64
	Fans                                                             []FanInfo
	TempSensors                                                      []TempSensor
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
	PeakLabels     []string
	ShowPeakLabels bool
}

func NewPeakStepChart() *PeakStepChart {
	return &PeakStepChart{StepChart: w.NewStepChart()}
}

func (sc *PeakStepChart) Draw(buf *ui.Buffer) {
	sc.StepChart.Draw(buf)
	if !sc.ShowPeakLabels || len(sc.PeakLabels) == 0 {
		return
	}
	sc.drawPeakLabels(buf)
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

	scale := max(sc.HorizontalScale, 1)
	// Match StepChart.drawLine: a sample is visible only while its x position
	// is strictly inside drawArea's right edge.
	maxSamples := min((drawArea.Dx()-1)/scale+1, len(sc.Data[0]))
	occupied := make(map[int][]image.Rectangle)
	for lineIdx, series := range sc.Data {
		if lineIdx >= len(sc.PeakLabels) || sc.PeakLabels[lineIdx] == "" || len(series) == 0 {
			continue
		}
		visible := series[:min(maxSamples, len(series))]
		peakIdx, peak := 0, visible[0]
		for i, value := range visible[1:] {
			if value > peak {
				peakIdx, peak = i+1, value
			}
		}

		x := drawArea.Min.X + peakIdx*scale
		y := drawArea.Max.Y - 1 - int((peak/maxVal)*float64(drawArea.Dy()-1))
		y = max(drawArea.Min.Y, min(y, drawArea.Max.Y-1))
		label := sc.PeakLabels[lineIdx]
		labelX := x + 1
		if labelX+len(label) > drawArea.Max.X {
			labelX = x - len(label)
		}
		labelX = max(drawArea.Min.X, min(labelX, drawArea.Max.X-len(label)))

		labelY := sc.peakLabelY(y, labelX, len(label), drawArea, occupied)
		occupied[labelY] = append(occupied[labelY], image.Rect(labelX, labelY, labelX+len(label), labelY+1))
		style := ui.NewStyle(ui.SelectColor(sc.LineColors, lineIdx), sc.BorderStyle.Bg)
		buf.SetString(label, style, image.Pt(labelX, labelY))
	}
}

func (sc *PeakStepChart) peakLabelY(peakY, x, width int, area image.Rectangle, occupied map[int][]image.Rectangle) int {
	for _, offset := range []int{-1, 1, -2, 2, 0} {
		y := peakY + offset
		if y < area.Min.Y || y >= area.Max.Y {
			continue
		}
		candidate := image.Rect(x, y, x+width, y+1)
		collision := false
		for _, used := range occupied[y] {
			if candidate.Overlaps(used) {
				collision = true
				break
			}
		}
		if !collision {
			return y
		}
	}
	return peakY
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
