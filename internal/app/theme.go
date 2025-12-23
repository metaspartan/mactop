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
	"catppuccin-latte":     CatppuccinLatte.Lavender,     // Purple-blue
	"catppuccin-frappe":    CatppuccinFrappe.Mauve,       // Purple
	"catppuccin-macchiato": CatppuccinMacchiato.Sapphire, // Blue
	"catppuccin-mocha":     CatppuccinMocha.Peach,        // Peach (orange)
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
	CurrentBgColor     ui.Color = ui.ColorClear // Current background color for widgets
)

// Background colors to cycle through with 'b' key
// "clear" means terminal default (transparent)
var bgColorNames = []string{
	"clear",
	"mocha-base",     // #1e1e2e
	"mocha-mantle",   // #181825
	"mocha-crust",    // #11111b
	"macchiato-base", // #24273a
	"frappe-base",    // #303446
}

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
		// CPU = Green (success/performance - per Catppuccin style guide)
		cpuGauge.BarColor = palette.Green
		cpuGauge.BorderStyle.Fg = palette.Green
		cpuGauge.TitleStyle.Fg = palette.Green
		cpuGauge.LabelStyle = ui.NewStyle(palette.Subtext0)

		// GPU = Blue (info/secondary compute - per Catppuccin style guide)
		gpuGauge.BarColor = palette.Blue
		gpuGauge.BorderStyle.Fg = palette.Blue
		gpuGauge.TitleStyle.Fg = palette.Blue
		gpuGauge.LabelStyle = ui.NewStyle(palette.Subtext0)

		// Memory = Yellow (warning/resource usage - per Catppuccin style guide)
		memoryGauge.BarColor = palette.Yellow
		memoryGauge.BorderStyle.Fg = palette.Yellow
		memoryGauge.TitleStyle.Fg = palette.Yellow
		memoryGauge.LabelStyle = ui.NewStyle(palette.Subtext0)

		// ANE = Lavender (AI/neural - distinctive accent)
		aneGauge.BarColor = palette.Lavender
		aneGauge.BorderStyle.Fg = palette.Lavender
		aneGauge.TitleStyle.Fg = palette.Lavender
		aneGauge.LabelStyle = ui.NewStyle(palette.Subtext0)
	}
}

func applyThemeToSparklines(color ui.Color) {
	if sparkline != nil {
		sparkline.LineColor = color
		sparkline.TitleStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if sparklineGroup != nil {
		sparklineGroup.BorderStyle.Fg = color
		sparklineGroup.TitleStyle.Fg = color
		sparklineGroup.TitleStyle.Bg = CurrentBgColor
	}
	if gpuSparkline != nil {
		gpuSparkline.LineColor = color
		gpuSparkline.TitleStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if gpuSparklineGroup != nil {
		gpuSparklineGroup.BorderStyle.Fg = color
		gpuSparklineGroup.TitleStyle.Fg = color
		gpuSparklineGroup.TitleStyle.Bg = CurrentBgColor
	}

	if tbNetSparklineIn != nil {
		tbNetSparklineIn.LineColor = color
		tbNetSparklineIn.TitleStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if tbNetSparklineOut != nil {
		tbNetSparklineOut.LineColor = color
		tbNetSparklineOut.TitleStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if tbNetSparklineGroup != nil {
		tbNetSparklineGroup.BorderStyle.Fg = color
		tbNetSparklineGroup.TitleStyle.Fg = color
		tbNetSparklineGroup.TitleStyle.Bg = CurrentBgColor
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
	if tbInfoParagraph != nil {
		tbInfoParagraph.BorderStyle.Fg = color
		tbInfoParagraph.TitleStyle.Fg = color
		tbInfoParagraph.TextStyle = ui.NewStyle(color)
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
		// Use distinct accent colors for each Catppuccin flavor
		var primaryColor ui.Color
		switch colorName {
		case "catppuccin-latte":
			primaryColor = catppuccinPalette.Lavender // Purple-blue
		case "catppuccin-frappe":
			primaryColor = catppuccinPalette.Mauve // Purple
		case "catppuccin-macchiato":
			primaryColor = catppuccinPalette.Sapphire // Blue
		case "catppuccin-mocha":
			primaryColor = catppuccinPalette.Peach // Peach (orange)
		default:
			primaryColor = catppuccinPalette.Lavender
		}

		ui.Theme.Block.Title.Fg = primaryColor
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
			return GetCatppuccinHex(currentConfig.Theme, "Primary")
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

// cycleBackground cycles through background colors
func cycleBackground() {
	currentBgIndex = (currentBgIndex + 1) % len(bgColorNames)
	applyBackground(bgColorNames[currentBgIndex])
}

// applyBackground sets the terminal background color
func applyBackground(bgName string) {
	var bgColor ui.Color
	switch bgName {
	case "clear":
		bgColor = ui.ColorClear
	case "mocha-base":
		bgColor = CatppuccinMocha.Base // #1e1e2e
	case "mocha-mantle":
		bgColor = CatppuccinMocha.Mantle // #181825
	case "mocha-crust":
		bgColor = CatppuccinMocha.Crust // #11111b
	case "macchiato-base":
		bgColor = CatppuccinMacchiato.Base // #24273a
	case "frappe-base":
		bgColor = CatppuccinFrappe.Base // #303446
	default:
		bgColor = ui.ColorClear
	}

	// Store current background color globally
	CurrentBgColor = bgColor

	// Set global theme background
	ui.Theme.Default.Bg = bgColor
	ui.Theme.Block.Border.Bg = bgColor
	ui.Theme.Block.Title.Bg = bgColor
	ui.Theme.Paragraph.Text.Bg = bgColor
	ui.Theme.Sparkline.Title.Bg = bgColor

	// Update main block
	if mainBlock != nil {
		mainBlock.BackgroundColor = bgColor
		mainBlock.BorderStyle.Bg = bgColor
		mainBlock.TitleStyle.Bg = bgColor
		mainBlock.TitleBottomStyle.Bg = bgColor
	}

	// Update process list
	if processList != nil {
		processList.BackgroundColor = bgColor
		processList.BorderStyle.Bg = bgColor
		processList.TitleStyle.Bg = bgColor
		processList.TextStyle.Bg = bgColor
	}

	// Update gauges
	if cpuGauge != nil {
		cpuGauge.BackgroundColor = bgColor
		cpuGauge.BorderStyle.Bg = bgColor
		cpuGauge.TitleStyle.Bg = bgColor
		cpuGauge.LabelStyle.Bg = bgColor
	}
	if gpuGauge != nil {
		gpuGauge.BackgroundColor = bgColor
		gpuGauge.BorderStyle.Bg = bgColor
		gpuGauge.TitleStyle.Bg = bgColor
		gpuGauge.LabelStyle.Bg = bgColor
	}
	if memoryGauge != nil {
		memoryGauge.BackgroundColor = bgColor
		memoryGauge.BorderStyle.Bg = bgColor
		memoryGauge.TitleStyle.Bg = bgColor
		memoryGauge.LabelStyle.Bg = bgColor
	}
	if aneGauge != nil {
		aneGauge.BackgroundColor = bgColor
		aneGauge.BorderStyle.Bg = bgColor
		aneGauge.TitleStyle.Bg = bgColor
		aneGauge.LabelStyle.Bg = bgColor
	}

	// Update paragraphs
	if PowerChart != nil {
		PowerChart.BackgroundColor = bgColor
		PowerChart.BorderStyle.Bg = bgColor
		PowerChart.TitleStyle.Bg = bgColor
		PowerChart.TextStyle.Bg = bgColor
	}
	if NetworkInfo != nil {
		NetworkInfo.BackgroundColor = bgColor
		NetworkInfo.BorderStyle.Bg = bgColor
		NetworkInfo.TitleStyle.Bg = bgColor
		NetworkInfo.TextStyle.Bg = bgColor
	}
	if modelText != nil {
		modelText.BackgroundColor = bgColor
		modelText.BorderStyle.Bg = bgColor
		modelText.TitleStyle.Bg = bgColor
		modelText.TextStyle.Bg = bgColor
	}
	if helpText != nil {
		helpText.BackgroundColor = bgColor
		helpText.BorderStyle.Bg = bgColor
		helpText.TitleStyle.Bg = bgColor
		helpText.TextStyle.Bg = bgColor
	}
	if tbInfoParagraph != nil {
		tbInfoParagraph.BackgroundColor = bgColor
		tbInfoParagraph.BorderStyle.Bg = bgColor
		tbInfoParagraph.TitleStyle.Bg = bgColor
		tbInfoParagraph.TextStyle.Bg = bgColor
	}
	if infoParagraph != nil {
		infoParagraph.BackgroundColor = bgColor
		infoParagraph.BorderStyle.Bg = bgColor
		infoParagraph.TitleStyle.Bg = bgColor
		infoParagraph.TextStyle.Bg = bgColor
	}

	// Update sparkline groups and individual sparklines
	if sparkline != nil {
		sparkline.BackgroundColor = bgColor
		sparkline.TitleStyle.Bg = bgColor
	}
	if sparklineGroup != nil {
		sparklineGroup.BackgroundColor = bgColor
		sparklineGroup.BorderStyle.Bg = bgColor
		sparklineGroup.TitleStyle.Bg = bgColor
	}
	if gpuSparkline != nil {
		gpuSparkline.BackgroundColor = bgColor
		gpuSparkline.TitleStyle.Bg = bgColor
	}
	if gpuSparklineGroup != nil {
		gpuSparklineGroup.BackgroundColor = bgColor
		gpuSparklineGroup.BorderStyle.Bg = bgColor
		gpuSparklineGroup.TitleStyle.Bg = bgColor
	}
	if tbNetSparklineIn != nil {
		tbNetSparklineIn.BackgroundColor = bgColor
		tbNetSparklineIn.TitleStyle.Bg = bgColor
	}
	if tbNetSparklineOut != nil {
		tbNetSparklineOut.BackgroundColor = bgColor
		tbNetSparklineOut.TitleStyle.Bg = bgColor
	}
	if tbNetSparklineGroup != nil {
		tbNetSparklineGroup.BackgroundColor = bgColor
		tbNetSparklineGroup.BorderStyle.Bg = bgColor
		tbNetSparklineGroup.TitleStyle.Bg = bgColor
	}

	// Update CPU core widget
	if cpuCoreWidget != nil {
		cpuCoreWidget.BackgroundColor = bgColor
		cpuCoreWidget.BorderStyle.Bg = bgColor
		cpuCoreWidget.TitleStyle.Bg = bgColor
	}
}

// GetCurrentBgName returns the current background color name
func GetCurrentBgName() string {
	if currentBgIndex < len(bgColorNames) {
		return bgColorNames[currentBgIndex]
	}
	return "clear"
}
