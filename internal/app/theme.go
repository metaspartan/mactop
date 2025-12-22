package app

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v4"
)

var colorMap = map[string]ui.Color{
	"green":   ui.ColorGreen,
	"red":     ui.ColorRed,
	"blue":    ui.ColorBlue,
	"skyblue": ui.ColorSkyBlue,
	"magenta": ui.ColorMagenta,
	"yellow":  ui.ColorYellow,
	"gold":    ui.ColorGold,
	"silver":  ui.ColorSilver,
	"white":   ui.ColorWhite,
	"lime":    ui.ColorLime,
	"orange":  ui.ColorOrange,
	"violet":  ui.ColorViolet,
	"pink":    ui.ColorPink,
}

var colorNames = []string{"green", "red", "blue", "skyblue", "magenta", "yellow", "gold", "silver", "white", "lime", "orange", "violet", "pink", "custom"}

var (
	BracketColor       ui.Color = ui.ColorWhite
	SecondaryTextColor ui.Color = 245
	IsLightMode        bool     = false
)

// RGB color helper function - creates a 24-bit RGB color for terminals with TrueColor support
func rgbColor(r, g, b uint8) ui.Color {
	// TrueColor format: 0x1000000 + (r << 16) + (g << 8) + b
	return ui.Color(0x1000000 + (uint32(r) << 16) + (uint32(g) << 8) + uint32(b))
}

// Adjusts saturation of an RGB color based on a percentage (0-100)
// At 0%, the color is at 50% saturation; at 100%, it's at full saturation
func adjustColorSaturation(baseR, baseG, baseB uint8, percent int) ui.Color {
	// Clamp percent to 0-100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	// Map 0-100% usage to 50-100% saturation
	// saturation = 0.5 + (percent / 100) * 0.5
	saturationFactor := 0.5 + (float64(percent) / 100.0 * 0.5)

	// Calculate grayscale value for desaturation
	gray := uint8(0.299*float64(baseR) + 0.587*float64(baseG) + 0.114*float64(baseB))

	// Interpolate between grayscale and full color based on saturation
	r := uint8(float64(gray) + saturationFactor*(float64(baseR)-float64(gray)))
	g := uint8(float64(gray) + saturationFactor*(float64(baseG)-float64(gray)))
	b := uint8(float64(gray) + saturationFactor*(float64(baseB)-float64(gray)))

	return rgbColor(r, g, b)
}

// Individual gauge color generators - using standard colors for better compatibility
func getCPUColor(percent int) ui.Color {
	// CPU = Green
	return ui.ColorGreen
}

func getGPUColor(percent int) ui.Color {
	// GPU = Magenta (closest to purple in standard colors)
	return ui.ColorMagenta
}

func getMemoryColor(percent int) ui.Color {
	// Memory = Blue
	return ui.ColorBlue
}

func getANEColor(percent int) ui.Color {
	// ANE = Red
	return ui.ColorRed
}

// Updates gauge colors with custom individual colors and dynamic saturation
func updateCustomGaugeColors() {
	if cpuGauge != nil {
		cpuColor := getCPUColor(cpuGauge.Percent)
		cpuGauge.BarColor = cpuColor
		cpuGauge.BorderStyle.Fg = cpuColor
		cpuGauge.TitleStyle.Fg = cpuColor
		cpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}

	if gpuGauge != nil {
		gpuColor := getGPUColor(gpuGauge.Percent)
		gpuGauge.BarColor = gpuColor
		gpuGauge.BorderStyle.Fg = gpuColor
		gpuGauge.TitleStyle.Fg = gpuColor
		gpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}

	if memoryGauge != nil {
		memColor := getMemoryColor(memoryGauge.Percent)
		memoryGauge.BarColor = memColor
		memoryGauge.BorderStyle.Fg = memColor
		memoryGauge.TitleStyle.Fg = memColor
		memoryGauge.LabelStyle = ui.NewStyle(SecondaryTextColor)
	}

	if aneGauge != nil {
		aneColor := getANEColor(aneGauge.Percent)
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
	isCustom := colorName == "custom"
	color, ok := colorMap[colorName]
	if !ok && !isCustom {
		color = ui.ColorGreen
		colorName = "green"
	} else if isCustom {
		// For custom theme, use green as base color for other UI elements
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

	if isCustom {
		updateCustomGaugeColors()
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
			return currentConfig.Theme
		}
		return "240"
	}

	if isCurrentUser {
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
		mainBlock.TitleBottomLeft = fmt.Sprintf(" %d/%d layout (%s) ", currentLayoutNum+1, totalLayouts, currentColorName)
	}
}
