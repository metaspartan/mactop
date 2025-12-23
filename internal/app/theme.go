package app

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v4"
	w "github.com/metaspartan/gotui/v4/widgets"
)

// themeOrder defines the order themes cycle through with 'c' key
// To add a new theme: add to themeOrder and colorMap (if it has a color)
var themeOrder = []string{
	"green", "red", "blue", "skyblue", "magenta", "yellow",
	"gold", "silver", "white", "lime", "orange", "violet", "pink",
	"coffee", "mint", "coral", "babyblue",
	"indigo", "teal", "lavender", "rose",
	"1977", // Special theme without a single color
	"latte", "frappe", "macchiato", "mocha",
}

// colorMap maps theme names to their primary UI color
var colorMap = map[string]ui.Color{
	"green":     ui.ColorGreen,
	"red":       ui.ColorRed,
	"blue":      ui.ColorBlue,
	"skyblue":   ui.ColorSkyBlue,
	"magenta":   ui.ColorMagenta,
	"yellow":    ui.ColorYellow,
	"gold":      ui.ColorGold,
	"silver":    ui.ColorSilver,
	"white":     ui.ColorWhite,
	"lime":      ui.ColorLime,
	"orange":    ui.ColorOrange,
	"violet":    ui.ColorViolet,
	"pink":      ui.ColorPink,
	"coffee":    ui.NewRGBColor(193, 165, 137),
	"mint":      ui.NewRGBColor(152, 255, 152),
	"coral":     ui.NewRGBColor(255, 127, 80),
	"babyblue":  ui.NewRGBColor(137, 207, 240),
	"indigo":    ui.NewRGBColor(75, 0, 130),
	"teal":      ui.NewRGBColor(0, 128, 128),
	"lavender":  ui.NewRGBColor(186, 187, 241),
	"rose":      ui.NewRGBColor(255, 0, 127),
	"latte":     CatppuccinLatte.Lavender,
	"frappe":    CatppuccinFrappe.Mauve,
	"macchiato": CatppuccinMacchiato.Sapphire,
	"mocha":     CatppuccinMocha.Peach,
}

// bgColorOrder defines the order backgrounds cycle through with 'b' key
// To add a new background: add to bgColorOrder and bgColorMap
var bgColorOrder = []string{
	"clear",
	"mocha-base", "mocha-mantle", "mocha-crust",
	"macchiato-base", "frappe-base",
	"midnight", "dark-slate", "deep-space",
	"white", "grey", "black",
}

// bgColorMap maps background names to their UI color
var bgColorMap = map[string]ui.Color{
	"clear":          ui.ColorClear,
	"mocha-base":     CatppuccinMocha.Base,
	"mocha-mantle":   CatppuccinMocha.Mantle,
	"mocha-crust":    CatppuccinMocha.Crust,
	"macchiato-base": CatppuccinMacchiato.Base,
	"frappe-base":    CatppuccinFrappe.Base,
	"deep-space":     rgb(13, 13, 19),
	"white":          ui.ColorWhite,
	"grey":           rgb(54, 54, 54),
	"black":          rgb(1, 1, 1),
}

var (
	BracketColor       ui.Color = ui.ColorWhite
	SecondaryTextColor ui.Color = 245
	IsLightMode        bool     = false
	CurrentBgColor     ui.Color = ui.ColorClear
)

// Catppuccin theme names
var catppuccinThemes = []string{"latte", "frappe", "macchiato", "mocha"}

// IsCatppuccinTheme returns true if the theme is a Catppuccin theme
func IsCatppuccinTheme(theme string) bool {
	for _, t := range catppuccinThemes {
		if theme == t {
			return true
		}
	}
	return false
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
		cpuGauge.BorderStyle.Bg = CurrentBgColor
		cpuGauge.TitleStyle.Fg = cpuColor
		cpuGauge.TitleStyle.Bg = CurrentBgColor
		cpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)
	}

	if gpuGauge != nil {
		gpuColor := getGPUColor()
		gpuGauge.BarColor = gpuColor
		gpuGauge.BorderStyle.Fg = gpuColor
		gpuGauge.BorderStyle.Bg = CurrentBgColor
		gpuGauge.TitleStyle.Fg = gpuColor
		gpuGauge.TitleStyle.Bg = CurrentBgColor
		gpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)
	}

	if memoryGauge != nil {
		memColor := getMemoryColor()
		memoryGauge.BarColor = memColor
		memoryGauge.BorderStyle.Fg = memColor
		memoryGauge.BorderStyle.Bg = CurrentBgColor
		memoryGauge.TitleStyle.Fg = memColor
		memoryGauge.TitleStyle.Bg = CurrentBgColor
		memoryGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)
	}

	if aneGauge != nil {
		aneColor := getANEColor()
		aneGauge.BarColor = aneColor
		aneGauge.BorderStyle.Fg = aneColor
		aneGauge.BorderStyle.Bg = CurrentBgColor
		aneGauge.TitleStyle.Fg = aneColor
		aneGauge.TitleStyle.Bg = CurrentBgColor
		aneGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)
	}
}

func applyThemeToGauges(color ui.Color) {
	if cpuGauge != nil {
		cpuGauge.BarColor = color
		cpuGauge.BorderStyle.Fg = color
		cpuGauge.BorderStyle.Bg = CurrentBgColor
		cpuGauge.TitleStyle.Fg = color
		cpuGauge.TitleStyle.Bg = CurrentBgColor
		cpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)

		gpuGauge.BarColor = color
		gpuGauge.BorderStyle.Fg = color
		gpuGauge.BorderStyle.Bg = CurrentBgColor
		gpuGauge.TitleStyle.Fg = color
		gpuGauge.TitleStyle.Bg = CurrentBgColor
		gpuGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)

		memoryGauge.BarColor = color
		memoryGauge.BorderStyle.Fg = color
		memoryGauge.BorderStyle.Bg = CurrentBgColor
		memoryGauge.TitleStyle.Fg = color
		memoryGauge.TitleStyle.Bg = CurrentBgColor
		memoryGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)

		aneGauge.BarColor = color
		aneGauge.BorderStyle.Fg = color
		aneGauge.BorderStyle.Bg = CurrentBgColor
		aneGauge.TitleStyle.Fg = color
		aneGauge.TitleStyle.Bg = CurrentBgColor
		aneGauge.LabelStyle = ui.NewStyle(SecondaryTextColor, CurrentBgColor)
	}
}

func applyCatppuccinThemeToGauges(palette *CatppuccinPalette) {
	if cpuGauge != nil {
		// CPU = Green (success/performance - per Catppuccin style guide)
		cpuGauge.BarColor = palette.Green
		cpuGauge.BorderStyle.Fg = palette.Green
		cpuGauge.BorderStyle.Bg = CurrentBgColor
		cpuGauge.TitleStyle.Fg = palette.Green
		cpuGauge.TitleStyle.Bg = CurrentBgColor
		cpuGauge.LabelStyle = ui.NewStyle(palette.Subtext0, CurrentBgColor)

		// GPU = Blue (info/secondary compute - per Catppuccin style guide)
		gpuGauge.BarColor = palette.Blue
		gpuGauge.BorderStyle.Fg = palette.Blue
		gpuGauge.BorderStyle.Bg = CurrentBgColor
		gpuGauge.TitleStyle.Fg = palette.Blue
		gpuGauge.TitleStyle.Bg = CurrentBgColor
		gpuGauge.LabelStyle = ui.NewStyle(palette.Subtext0, CurrentBgColor)

		// Memory = Yellow (warning/resource usage - per Catppuccin style guide)
		memoryGauge.BarColor = palette.Yellow
		memoryGauge.BorderStyle.Fg = palette.Yellow
		memoryGauge.BorderStyle.Bg = CurrentBgColor
		memoryGauge.TitleStyle.Fg = palette.Yellow
		memoryGauge.TitleStyle.Bg = CurrentBgColor
		memoryGauge.LabelStyle = ui.NewStyle(palette.Subtext0, CurrentBgColor)

		// ANE = Lavender (AI/neural - distinctive accent)
		aneGauge.BarColor = palette.Lavender
		aneGauge.BorderStyle.Fg = palette.Lavender
		aneGauge.BorderStyle.Bg = CurrentBgColor
		aneGauge.TitleStyle.Fg = palette.Lavender
		aneGauge.TitleStyle.Bg = CurrentBgColor
		aneGauge.LabelStyle = ui.NewStyle(palette.Subtext0, CurrentBgColor)
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
		processList.TextStyle = ui.NewStyle(color, CurrentBgColor)
		selectedFg := ui.ColorBlack
		if lightMode && color == ui.ColorBlack {
			selectedFg = ui.ColorWhite
		}
		processList.SelectedStyle = ui.NewStyle(selectedFg, color)
		processList.BorderStyle.Fg = color
		processList.BorderStyle.Bg = CurrentBgColor
		processList.TitleStyle.Fg = color
		processList.TitleStyle.Bg = CurrentBgColor
	}
	if NetworkInfo != nil {
		NetworkInfo.TextStyle = ui.NewStyle(color, CurrentBgColor)
		NetworkInfo.BorderStyle.Fg = color
		NetworkInfo.BorderStyle.Bg = CurrentBgColor
		NetworkInfo.TitleStyle.Fg = color
		NetworkInfo.TitleStyle.Bg = CurrentBgColor
	}
	if PowerChart != nil {
		PowerChart.TextStyle = ui.NewStyle(color, CurrentBgColor)
		PowerChart.BorderStyle.Fg = color
		PowerChart.BorderStyle.Bg = CurrentBgColor
		PowerChart.TitleStyle.Fg = color
		PowerChart.TitleStyle.Bg = CurrentBgColor
	}
	if cpuCoreWidget != nil {
		cpuCoreWidget.BorderStyle.Fg = color
		cpuCoreWidget.BorderStyle.Bg = CurrentBgColor
		cpuCoreWidget.TitleStyle.Fg = color
		cpuCoreWidget.TitleStyle.Bg = CurrentBgColor
	}
	if modelText != nil {
		modelText.BorderStyle.Fg = color
		modelText.BorderStyle.Bg = CurrentBgColor
		modelText.TitleStyle.Fg = color
		modelText.TitleStyle.Bg = CurrentBgColor
		modelText.TextStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if helpText != nil {
		helpText.BorderStyle.Fg = color
		helpText.BorderStyle.Bg = CurrentBgColor
		helpText.TitleStyle.Fg = color
		helpText.TitleStyle.Bg = CurrentBgColor
		helpText.TextStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if mainBlock != nil {
		mainBlock.BorderStyle.Fg = color
		mainBlock.BorderStyle.Bg = CurrentBgColor
		mainBlock.TitleStyle.Fg = color
		mainBlock.TitleStyle.Bg = CurrentBgColor
		mainBlock.TitleBottomStyle.Fg = color
		mainBlock.TitleBottomStyle.Bg = CurrentBgColor
	}
	if tbInfoParagraph != nil {
		tbInfoParagraph.BorderStyle.Fg = color
		tbInfoParagraph.BorderStyle.Bg = CurrentBgColor
		tbInfoParagraph.TitleStyle.Fg = color
		tbInfoParagraph.TitleStyle.Bg = CurrentBgColor
		tbInfoParagraph.TextStyle = ui.NewStyle(color, CurrentBgColor)
	}
	if infoParagraph != nil {
		infoParagraph.BorderStyle.Fg = color
		infoParagraph.BorderStyle.Bg = CurrentBgColor
		infoParagraph.TitleStyle.Fg = color
		infoParagraph.TitleStyle.Bg = CurrentBgColor
		infoParagraph.TextStyle = ui.NewStyle(color, CurrentBgColor)
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
		case "latte":
			primaryColor = catppuccinPalette.Lavender // Purple-blue
		case "frappe":
			primaryColor = catppuccinPalette.Mauve // Purple
		case "macchiato":
			primaryColor = catppuccinPalette.Sapphire // Blue
		case "mocha":
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
			processList.TextStyle = ui.NewStyle(primaryColor, CurrentBgColor)
			selectedFg := catppuccinPalette.Base
			processList.SelectedStyle = ui.NewStyle(selectedFg, primaryColor)
			processList.BorderStyle.Fg = primaryColor
			processList.BorderStyle.Bg = CurrentBgColor
			processList.TitleStyle.Fg = primaryColor
			processList.TitleStyle.Bg = CurrentBgColor
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

func resolveThemeColorString(theme string) string {
	switch theme {
	case "coffee":
		return "#C1A589"
	case "mint":
		return "#98FF98"
	case "babyblue":
		return "#89CFF0"
	case "indigo":
		return "#4B0082"
	case "teal":
		return "#008080"
	case "coral":
		return "#FF7F50"
	case "lavender":
		return "#BABBF1" // Matches RGB(186, 187, 241)
	case "rose":
		return "#FF007F"
	case "lime", "orange", "violet", "pink":
		return theme
	case "1977":
		return "green"
	default:
		return theme
	}
}

func GetProcessTextColor(isCurrentUser bool) string {
	if IsLightMode {
		if isCurrentUser {
			color := GetThemeColorWithLightMode(currentConfig.Theme, true)
			if color == ui.ColorBlack {
				return "black"
			}
			if IsCatppuccinTheme(currentConfig.Theme) {
				return GetCatppuccinHex(currentConfig.Theme, "Text")
			}
			return resolveThemeColorString(currentConfig.Theme)
		}
		return "240"
	}

	if isCurrentUser {
		if IsCatppuccinTheme(currentConfig.Theme) {
			return GetCatppuccinHex(currentConfig.Theme, "Primary")
		}
		return resolveThemeColorString(currentConfig.Theme)
	}
	return "white"
}

func cycleTheme() {
	currentIndex := 0
	for i, name := range themeOrder {
		if name == currentConfig.Theme {
			currentIndex = i
			break
		}
	}
	nextIndex := (currentIndex + 1) % len(themeOrder)
	currentColorName = themeOrder[nextIndex]
	applyTheme(themeOrder[nextIndex], IsLightMode)

	currentConfig.Theme = currentColorName
	saveConfig()

	updateInfoUI()

	if mainBlock != nil {
		displayColorName := currentColorName
		if IsLightMode && currentColorName == "white" {
			displayColorName = "black"
		}
		mainBlock.TitleBottomLeft = fmt.Sprintf(" %d/%d layout (%s) ", currentLayoutNum+1, totalLayouts, displayColorName)
	}
}

// applyInitialBackground applies the saved background from config on startup
func applyInitialBackground() {
	bgName := currentConfig.Background
	if bgName == "" {
		bgName = "clear"
	}
	// Set currentBgIndex to match saved background
	for i, name := range bgColorOrder {
		if name == bgName {
			currentBgIndex = i
			break
		}
	}
	applyBackground(bgName)
}

// cycleBackground cycles through background colors
func cycleBackground() {
	currentBgIndex = (currentBgIndex + 1) % len(bgColorOrder)
	bgName := bgColorOrder[currentBgIndex]
	applyBackground(bgName)
	currentConfig.Background = bgName
	saveConfig()
}

// applyBackground sets the terminal background color
func applyBackground(bgName string) {
	bgColor, ok := bgColorMap[bgName]
	if !ok {
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

	applyBackgroundToBlocks(bgColor)
	applyBackgroundToGauges(bgColor)
	applyBackgroundToParagraphs(bgColor)
	applyBackgroundToSparklines(bgColor)
}

func applyBackgroundToBlocks(bgColor ui.Color) {
	if mainBlock != nil {
		mainBlock.BackgroundColor = bgColor
		mainBlock.BorderStyle.Bg = bgColor
		mainBlock.TitleStyle.Bg = bgColor
		mainBlock.TitleBottomStyle.Bg = bgColor
	}
	if processList != nil {
		processList.BackgroundColor = bgColor
		processList.BorderStyle.Bg = bgColor
		processList.TitleStyle.Bg = bgColor
		processList.TextStyle.Bg = bgColor
	}
	if cpuCoreWidget != nil {
		cpuCoreWidget.BackgroundColor = bgColor
		cpuCoreWidget.BorderStyle.Bg = bgColor
		cpuCoreWidget.TitleStyle.Bg = bgColor
	}
}

func applyBackgroundToGauges(bgColor ui.Color) {
	gauges := []*w.Gauge{cpuGauge, gpuGauge, memoryGauge, aneGauge}
	for _, g := range gauges {
		if g != nil {
			g.BackgroundColor = bgColor
			g.BorderStyle.Bg = bgColor
			g.TitleStyle.Bg = bgColor
			g.LabelStyle.Bg = bgColor
		}
	}
}

func applyBackgroundToParagraphs(bgColor ui.Color) {
	paragraphs := []*w.Paragraph{PowerChart, NetworkInfo, modelText, helpText, tbInfoParagraph, infoParagraph}
	for _, p := range paragraphs {
		if p != nil {
			p.BackgroundColor = bgColor
			p.BorderStyle.Bg = bgColor
			p.TitleStyle.Bg = bgColor
			p.TextStyle.Bg = bgColor
		}
	}
}

func applyBackgroundToSparklines(bgColor ui.Color) {
	// Individual sparklines
	sparklines := []*w.Sparkline{sparkline, gpuSparkline, tbNetSparklineIn, tbNetSparklineOut}
	for _, s := range sparklines {
		if s != nil {
			s.BackgroundColor = bgColor
			s.TitleStyle.Bg = bgColor
		}
	}
	// Sparkline groups
	groups := []*w.SparklineGroup{sparklineGroup, gpuSparklineGroup, tbNetSparklineGroup}
	for _, g := range groups {
		if g != nil {
			g.BackgroundColor = bgColor
			g.BorderStyle.Bg = bgColor
			g.TitleStyle.Bg = bgColor
		}
	}
}

// GetCurrentBgName returns the current background color name
func GetCurrentBgName() string {
	if currentBgIndex < len(bgColorOrder) {
		return bgColorOrder[currentBgIndex]
	}
	return "clear"
}
