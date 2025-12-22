package app

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v4"
)

const (
	LayoutDefault         = "default"
	LayoutAlternative     = "alternative"
	LayoutAlternativeFull = "alternative_full"
	LayoutVertical        = "vertical"
	LayoutCompact         = "compact"
	LayoutDashboard       = "dashboard"
	LayoutGaugesOnly      = "gauges_only"
	LayoutGPUFocus        = "gpu_focus"
	LayoutCPUFocus        = "cpu_focus"
	LayoutSmall           = "small"
	LayoutNetworkIO       = "network_io"
	LayoutInfo            = "info"
)

var layoutOrder = []string{LayoutDefault, LayoutAlternative, LayoutAlternativeFull, LayoutVertical, LayoutCompact, LayoutDashboard, LayoutGaugesOnly, LayoutGPUFocus, LayoutCPUFocus, LayoutNetworkIO, LayoutSmall}

func setupGrid() {
	totalLayouts = len(layoutOrder)
	for i, layout := range layoutOrder {
		if layout == currentConfig.DefaultLayout {
			currentLayoutNum = i
			break
		}
	}
	applyLayout(currentConfig.DefaultLayout)
}

func cycleLayout() {
	currentIndex := 0
	for i, layout := range layoutOrder {
		if layout == currentConfig.DefaultLayout {
			currentIndex = i
			break
		}
	}
	nextIndex := (currentIndex + 1) % len(layoutOrder)
	currentConfig.DefaultLayout = layoutOrder[nextIndex]
	currentLayoutNum = nextIndex
	totalLayouts = len(layoutOrder)
	applyLayout(currentConfig.DefaultLayout)
	updateHelpText()
}

func applyLayout(layoutName string) {
	termWidth, termHeight := ui.TerminalDimensions()
	if mainBlock != nil {
		mainBlock.SetRect(0, 0, termWidth, termHeight)
		mainBlock.TitleBottomLeft = fmt.Sprintf(" %d/%d layout (%s) ", currentLayoutNum+1, totalLayouts, currentColorName)
		if termWidth < 93 {
			mainBlock.TitleBottom = ""
		} else {
			mainBlock.TitleBottom = " Help: h | Info: i | Layout: l | Color: c | Exit: q "
		}
	}
	grid = ui.NewGrid()

	setLayoutGrid(layoutName)

	if termWidth > 2 && termHeight > 2 {
		grid.SetRect(1, 1, termWidth-1, termHeight-1)
	}
}

func setLayoutGrid(layoutName string) {
	switch layoutName {
	case LayoutAlternative:
		grid.Set(
			ui.NewRow(1.0/2,
				ui.NewCol(1.0/2, cpuCoreWidget),
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, gpuGauge),
					ui.NewCol(1.0, ui.NewRow(1.0, memoryGauge)),
				),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, modelText),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/4, PowerChart),
				ui.NewCol(1.0/4, sparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutAlternativeFull:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, cpuCoreWidget),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, memoryGauge),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, modelText),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/4, PowerChart),
				ui.NewCol(1.0/4, sparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutVertical:
		grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(0.4,
					ui.NewRow(1.0/8, cpuGauge),
					ui.NewRow(1.0/8, gpuGauge),
					ui.NewRow(1.0/8, aneGauge),
					ui.NewRow(1.5/8, memoryGauge),
					ui.NewRow(1.5/8, NetworkInfo),
					ui.NewRow(2.0/8, modelText),
				),
				ui.NewCol(0.6,
					ui.NewRow(3.0/4, processList),
					ui.NewRow(1.0/4,
						ui.NewCol(1.0/2, PowerChart),
						ui.NewCol(1.0/2, sparklineGroup),
					),
				),
			),
		)
	case LayoutCompact:
		grid.Set(
			ui.NewRow(2.0/8,
				ui.NewCol(1.0/4, cpuGauge),
				ui.NewCol(1.0/4, gpuGauge),
				ui.NewCol(1.0/4, memoryGauge),
				ui.NewCol(1.0/4, aneGauge),
			),
			ui.NewRow(2.0/8,
				ui.NewCol(1.0/3, modelText),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/3, PowerChart),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutDashboard:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/4, cpuGauge),
				ui.NewCol(1.0/4, gpuGauge),
				ui.NewCol(1.0/4, memoryGauge),
				ui.NewCol(1.0/4, aneGauge),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, sparklineGroup),
				ui.NewCol(1.0/2, gpuSparklineGroup),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutGaugesOnly:
		grid.Set(
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, cpuGauge),
				ui.NewCol(1.0/2, memoryGauge),
			),
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, aneGauge),
			),
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, gpuSparklineGroup),
				ui.NewCol(1.0/2, sparklineGroup),
			),
		)
	case LayoutGPUFocus:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, gpuSparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/4, cpuGauge),
				ui.NewCol(1.0/4, memoryGauge),
				ui.NewCol(1.0/4, NetworkInfo),
				ui.NewCol(1.0/4, modelText),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutCPUFocus:
		grid.Set(
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/3, cpuGauge),
				ui.NewCol(2.0/3, cpuCoreWidget),
			),
			ui.NewRow(1.0/6,
				ui.NewCol(1.0/4, gpuGauge),
				ui.NewCol(1.0/4, memoryGauge),
				ui.NewCol(1.0/4, sparklineGroup),
				ui.NewCol(1.0/4, PowerChart),
			),
			ui.NewRow(3.0/6,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutNetworkIO:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/3, gpuSparklineGroup),
				ui.NewCol(1.0/3, sparklineGroup),
				ui.NewCol(1.0/3, NetworkInfo),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, memoryGauge),
					ui.NewRow(1.0/2, gpuGauge),
				),
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, tbInfoParagraph),
					ui.NewRow(1.0/2, tbNetPlot),
				),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutSmall:
		grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(1.0,
					ui.NewRow(1.0/4, cpuGauge),
					ui.NewRow(1.0/4, gpuGauge),
					ui.NewRow(1.0/4, memoryGauge),
					ui.NewRow(1.0/4, aneGauge),
				),
			),
		)
	case LayoutInfo:
		grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(1.0, infoParagraph),
			),
		)
	default: // LayoutDefault
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, cpuGauge),
				ui.NewCol(1.0/2, gpuGauge),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, aneGauge),
					ui.NewRow(1.0/2,
						ui.NewCol(1.0/2, PowerChart),
						ui.NewCol(1.0/2, sparklineGroup),
					),
				),
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, memoryGauge),
					ui.NewRow(1.0/2,
						ui.NewCol(1.0/3, modelText),
						ui.NewCol(2.0/3, NetworkInfo),
					),
				),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	}
}
