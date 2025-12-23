package app

import (
	"fmt"
	"strings"

	ui "github.com/metaspartan/gotui/v4"
)

var colorMap = map[string]ui.Color{
	"green":                ui.ColorGreen,
	"red":                  ui.ColorRed,
	"blue":                 ui.ColorBlue,
	"skyblue":              ui.ColorSkyBlue,
	"magenta":              ui.ColorMagenta,
	"yellow":               ui.ColorYellow,
	"gold":                 ui.ColorGold,
	"silver":               ui.ColorSilver,
	"white":                ui.ColorWhite,
	"lime":                 ui.ColorLime,
	"orange":               ui.ColorOrange,
	"violet":               ui.ColorViolet,
	"pink":                 ui.ColorPink,
	"catppuccin-latte":     CatppuccinLatte.Peach,
	"catppuccin-frappe":    CatppuccinFrappe.Peach,
	"catppuccin-macchiato": CatppuccinMacchiato.Peach,
	"catppuccin-mocha":     CatppuccinMocha.Peach,
}

var colorNames = []string{
	"green",
	"red",
	"blue",
	"skyblue",
	"magenta",
	"yellow",
	"gold",
	"silver",
	"white",
	"lime",
	"orange",
	"violet",
	"pink",
	"1977",
	"catppuccin-latte",
	"catppuccin-frappe",
	"catppuccin-macchiato",
	"catppuccin-mocha",
}

var (
	BracketColor       ui.Color = ui.ColorWhite
	SecondaryTextColor ui.Color = 245
	IsLightMode        bool     = false
)

func getCPUColor() ui.Color {
	return ui.ColorGreen
}

func getGPUColor() ui.Color {
	return ui.ColorMagenta
}

func getMemoryColor() ui.Color {
	return ui.ColorBlue
}

func getANEColor() ui.Color {
	return ui.ColorRed
}

func update1977GaugeColors() {
	if cpuGauge != nil {
		cpuColor := getCPUColor()
		cpuGauge.BarColor = cpuColor
		cpuGauge.BorderStyle.Fg = cpuColor
		cpuGauge.TitleStyle.Fg = cpuColor
		cpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}

	if gpuGauge != nil {
		gpuColor := getGPUColor()
		gpuGauge.BarColor = gpuColor
		gpuGauge.BorderStyle.Fg = gpuColor
		gpuGauge.TitleStyle.Fg = gpuColor
		gpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}

	if memoryGauge != nil {
		memColor := getMemoryColor()
		memoryGauge.BarColor = memColor
		memoryGauge.BorderStyle.Fg = memColor
		memoryGauge.TitleStyle.Fg = memColor
		memoryGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}

	if aneGauge != nil {
		aneColor := getANEColor()
		aneGauge.BarColor = aneColor
		aneGauge.BorderStyle.Fg = aneColor
		aneGauge.TitleStyle.Fg = aneColor
		aneGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}
}

func applyThemeToGauges(color ui.Color) {
	if cpuGauge != nil {
		cpuGauge.BarColor = color
		cpuGauge.BorderStyle.Fg = color
		cpuGauge.TitleStyle.Fg = color
		cpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)

		gpuGauge.BarColor = color
		gpuGauge.BorderStyle.Fg = color
		gpuGauge.TitleStyle.Fg = color
		gpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)

		memoryGauge.BarColor = color
		memoryGauge.BorderStyle.Fg = color
		memoryGauge.TitleStyle.Fg = color
		memoryGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)

		aneGauge.BarColor = color
		aneGauge.BorderStyle.Fg = color
		aneGauge.TitleStyle.Fg = color
		aneGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}
}

func applyCatppuccinThemeToGauges(palette *CatppuccinPalette) {
	if cpuGauge != nil {
		// CPU = Peach (warm, distinct from standard green)
		cpuGauge.BarColor = palette.Peach
		cpuGauge.BorderStyle.Fg = palette.Peach
		cpuGauge.TitleStyle.Fg = palette.Peach
		cpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)

		// GPU = Mauve (purple-ish)
		gpuGauge.BarColor = palette.Mauve
		gpuGauge.BorderStyle.Fg = palette.Mauve
		gpuGauge.TitleStyle.Fg = palette.Mauve
		gpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)

		// Memory = Pink (soft, pleasant)
		memoryGauge.BarColor = palette.Pink
		memoryGauge.BorderStyle.Fg = palette.Pink
		memoryGauge.TitleStyle.Fg = palette.Pink
		memoryGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)

		// ANE = Maroon (red-ish but earthy)
		aneGauge.BarColor = palette.Maroon
		aneGauge.BorderStyle.Fg = palette.Maroon
		aneGauge.TitleStyle.Fg = palette.Maroon
		aneGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}
}

func applyThemeToSparklines(color ui.Color) {
	if sparkline != nil {
		sparkline.LineColor = color
		sparkline.TitleStyle = ui.NewStyle(color)
	}
	if sparklineGroup != nil {
		sparklineGroup.BorderStyle.Fg = color
		sparklineGroup.TitleStyle.Fg = color
	}
	if gpuSparkline != nil {
		gpuSparkline.LineColor = color
		gpuSparkline.TitleStyle = ui.NewStyle(color)
	}
	if gpuSparklineGroup != nil {
		gpuSparklineGroup.BorderStyle.Fg = color
		gpuSparklineGroup.TitleStyle.Fg = color
	}
}

func applyThemeToWidgets(color ui.Color, lightMode bool) {
	if processList != nil {
		processList.TextStyle = ui.NewStyle(color)
		selectedFg := ui.ColorBlack
		if lightMode && color == ui.ColorBlack {
			selectedFg = ui.ColorWhite
		}
		processList.SelectedStyle = ui.NewStyle(selectedFg, color)
		processList.BorderStyle.Fg = color
		processList.TitleStyle.Fg = color
	}
	if NetworkInfo != nil {
		NetworkInfo.TextStyle = ui.NewStyle(color)
		NetworkInfo.BorderStyle.Fg = color
		NetworkInfo.TitleStyle.Fg = color
	}
	if PowerChart != nil {
		PowerChart.TextStyle = ui.NewStyle(color)
		PowerChart.BorderStyle.Fg = color
		PowerChart.TitleStyle.Fg = color
	}
	if cpuCoreWidget != nil {
		cpuCoreWidget.BorderStyle.Fg = color
		cpuCoreWidget.TitleStyle.Fg = color
	}
	if modelText != nil {
		modelText.BorderStyle.Fg = color
		modelText.TitleStyle.Fg = color
		modelText.TextStyle = ui.NewStyle(color)
	}
	if helpText != nil {
		helpText.BorderStyle.Fg = color
		helpText.TitleStyle.Fg = color
		helpText.TextStyle = ui.NewStyle(color)
	}
	if mainBlock != nil {
		mainBlock.BorderStyle.Fg = color
		mainBlock.TitleStyle.Fg = color
		mainBlock.TitleBottomStyle.Fg = color
	}
}

func applyTheme(colorName string, lightMode bool) {
	is1977 := colorName == "1977"
	color, ok := colorMap[colorName]
	if !ok && !is1977 {
		color = ui.ColorGreen
		colorName = "green"
	} else if is1977 {
		color = ui.ColorGreen
	}

	currentConfig.Theme = colorName

	if lightMode {
		BracketColor = ui.ColorBlack
		SecondaryTextColor = ui.ColorBlack
		if color == ui.ColorWhite {
			color = ui.ColorBlack
		}
	} else {
		BracketColor = ui.ColorWhite
		SecondaryTextColor = 245
	}

	ui.Theme.Block.Title.Fg = color
	ui.Theme.Block.Border.Fg = color
	ui.Theme.Paragraph.Text.Fg = color
	ui.Theme.Gauge.Label.Fg = color
	ui.Theme.Gauge.Bar = color
	ui.Theme.BarChart.Bars = []ui.Color{color}

	if is1977 {
		update1977GaugeColors()
	} else if catppuccinPalette := GetCatppuccinPalette(colorName); catppuccinPalette != nil {
		primaryColor := catppuccinPalette.Peach

		ui.Theme.Block.Title.Fg = primaryColor
		ui.Theme.Block.Border.Fg = catppuccinPalette.Overlay2
		ui.Theme.Block.Border.Fg = primaryColor
		ui.Theme.Paragraph.Text.Fg = catppuccinPalette.Text
		ui.Theme.Gauge.Label.Fg = catppuccinPalette.Subtext1
		ui.Theme.BarChart.Bars = []ui.Color{catppuccinPalette.Blue}

		applyCatppuccinThemeToGauges(catppuccinPalette)
		applyThemeToSparklines(primaryColor)
		applyThemeToWidgets(primaryColor, lightMode)

		if mainBlock != nil {
			mainBlock.BorderStyle.Fg = primaryColor
			mainBlock.TitleStyle.Fg = primaryColor
			mainBlock.TitleBottomStyle.Fg = primaryColor
		}
		if processList != nil {
			processList.TextStyle = ui.NewStyle(primaryColor)
			selectedFg := catppuccinPalette.Base
			if colorName == "catppuccin-latte" {
				selectedFg = catppuccinPalette.Text
			}
			processList.SelectedStyle = ui.NewStyle(selectedFg, primaryColor)
			processList.BorderStyle.Fg = primaryColor
			processList.TitleStyle.Fg = primaryColor
		}
		return
	} else {
		applyThemeToGauges(color)
	}
	applyThemeToSparklines(color)
	applyThemeToWidgets(color, lightMode)
}

func GetThemeColor(colorName string) ui.Color {
	color, ok := colorMap[colorName]
	if !ok {
		return ui.ColorGreen
	}
	return color
}

func GetThemeColorWithLightMode(colorName string, lightMode bool) ui.Color {
	color := GetThemeColor(colorName)
	if lightMode && color == ui.ColorWhite {
		return ui.ColorBlack
	}
	return color
}

func GetProcessTextColor(isCurrentUser bool) string {
	if IsLightMode {
		if isCurrentUser {
			color := GetThemeColorWithLightMode(currentConfig.Theme, true)
			if color == ui.ColorBlack {
				return "black"
			}
			if currentConfig.Theme == "1977" {
				return "green"
			}
			if strings.HasPrefix(currentConfig.Theme, "catppuccin-") {
				return GetCatppuccinHex(currentConfig.Theme, "Text")
			}
			return currentConfig.Theme
		}
		return "240"
	}

	if isCurrentUser {
		if strings.HasPrefix(currentConfig.Theme, "catppuccin-") {
			return GetCatppuccinHex(currentConfig.Theme, "Peach")
		}
		switch currentConfig.Theme {
		case "lime":
			return "lime"
		case "orange":
			return "orange"
		case "violet":
			return "violet"
		case "pink":
			return "pink"
		default:
			if currentConfig.Theme == "1977" {
				return "green"
			}
			return currentConfig.Theme
		}
	}
	return "white"
}

func cycleTheme() {
	currentIndex := 0
	for i, name := range colorNames {
		if name == currentConfig.Theme {
			currentIndex = i
			break
		}
	}
	nextIndex := (currentIndex + 1) % len(colorNames)
	currentColorName = colorNames[nextIndex]
	applyTheme(colorNames[nextIndex], IsLightMode)
	if mainBlock != nil {
		displayColorName := currentColorName
		if IsLightMode && currentColorName == "white" {
			displayColorName = "black"
		}
		mainBlock.TitleBottomLeft = fmt.Sprintf(" %d/%d layout (%s) ", currentLayoutNum+1, totalLayouts, displayColorName)
	}
}
