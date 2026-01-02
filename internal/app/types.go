package app

import (
	"fmt"
	"image"
	"time"

	ui "github.com/metaspartan/gotui/v5"
)

type CPUUsage struct {
	User   float64
	System float64
	Idle   float64
	Nice   float64
}

type CPUMetrics struct {
	EClusterActive, EClusterFreqMHz, PClusterActive, PClusterFreqMHz int
	ECores, PCores                                                   []int
	CoreMetrics                                                      map[string]int
	ANEW, CPUW, GPUW, DRAMW, GPUSRAMW, PackageW, SystemW             float64
	CoreUsages                                                       []float64
	Throttled                                                        bool
	CPUTemp                                                          float64
	GPUTemp                                                          float64
}

type SystemInfo struct {
	Name         string `json:"name"`
	CoreCount    int    `json:"core_count"`
	ECoreCount   int    `json:"e_core_count"`
	PCoreCount   int    `json:"p_core_count"`
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
	Power         float64
	Temp          float32
}

type ProcessMetrics struct {
	PID                                      int
	CPU, LastTime, Memory, GPU               float64 // GPU is ms/s of GPU time
	VSZ, RSS                                 int64
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
	timer       *time.Timer
	gracePeriod time.Duration
	C           chan struct{}
}

type CPUCoreWidget struct {
	*ui.Block
	cores                  []float64
	labels                 []string
	eCoreCount, pCoreCount int
	modelName              string
	cpuIndexMap            []int // maps display index -> hardware CPU index
}

func NewEventThrottler(gracePeriod time.Duration) *EventThrottler {
	return &EventThrottler{
		timer:       nil,
		gracePeriod: gracePeriod,
		C:           make(chan struct{}, 1),
	}
}

func NewCPUMetrics() CPUMetrics {
	return CPUMetrics{
		CoreMetrics: make(map[string]int),
		ECores:      make([]int, 0),
		PCores:      make([]int, 0),
	}
}

func (e *EventThrottler) Notify() {
	if e.timer != nil {
		return
	}

	e.timer = time.AfterFunc(e.gracePeriod, func() {
		e.timer = nil
		select {
		case e.C <- struct{}{}:
		default:
		}
	})
}

func NewCPUCoreWidget(modelInfo SystemInfo) *CPUCoreWidget {
	modelName := modelInfo.Name

	// Use dynamic core topology detection from IORegistry
	labels, eCount, pCount, cpuIndexMap := BuildCoreLabels()

	if labels == nil || len(labels) == 0 {
		// Fallback to sysctl-based counts (old behavior)
		eCoreCount := modelInfo.ECoreCount
		pCoreCount := modelInfo.PCoreCount
		totalCores := eCoreCount + pCoreCount

		labels = make([]string, totalCores)
		cpuIndexMap = make([]int, totalCores)
		for i := 0; i < eCoreCount; i++ {
			labels[i] = fmt.Sprintf("E%d", i)
			cpuIndexMap[i] = i // 1:1 mapping for fallback
		}
		for i := 0; i < pCoreCount; i++ {
			labels[i+eCoreCount] = fmt.Sprintf("P%d", i)
			cpuIndexMap[i+eCoreCount] = i + eCoreCount
		}
		eCount = eCoreCount
		pCount = pCoreCount
	}

	totalCores := len(labels)

	return &CPUCoreWidget{
		Block:       ui.NewBlock(),
		cores:       make([]float64, totalCores),
		labels:      labels,
		eCoreCount:  eCount,
		pCoreCount:  pCount,
		modelName:   modelName,
		cpuIndexMap: cpuIndexMap,
	}
}

func (w *CPUCoreWidget) UpdateUsage(usage []float64) {
	// Remap usage data from hardware order to display order (E cores first, then P)
	if w.cpuIndexMap != nil && len(w.cpuIndexMap) > 0 {
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
	innerBarWidth := availWidth - 2 - textWidth
	if innerBarWidth < 0 {
		innerBarWidth = 0
	}
	usedWidth := int((usage / 100.0) * float64(innerBarWidth))

	buf.SetString("[", ui.NewStyle(BracketColor, CurrentBgColor), image.Pt(x+labelWidth, y))

	for bx := 0; bx < innerBarWidth; bx++ {
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
	totalCores := len(w.cores)
	availableWidth := w.Inner.Dx()
	availableHeight := w.Inner.Dy()

	cols, rows, colXs, colWidths := w.calculateLayout(availableWidth, availableHeight, totalCores)
	fullCols := totalCores - (rows-1)*cols

	for i := 0; i < totalCores; i++ {
		col := i % cols
		row := i / cols
		actualIndex := col*rows + row - max(0, col-fullCols)

		if actualIndex >= totalCores || row >= rows {
			continue
		}

		x := w.Inner.Min.X + colXs[col]
		y := w.Inner.Min.Y + row

		if y >= w.Inner.Max.Y {
			continue
		}

		w.drawCore(buf, x, y, colWidths[col], actualIndex, w.cores[actualIndex], themeColor)
	}
}
