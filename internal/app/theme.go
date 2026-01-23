package app

import (
	"fmt"
	"slices"

	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
)

// themeOrder defines the order themes cycle through with 'c' key
// To add a new theme: add to themeOrder and colorMap (if it has a color)
var themeOrder = []string{
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
	"coffee",
	"mint",
	"coral",
	"babyblue",
	"indigo",
	"teal",
	"lavender",
	"rose",
	"cyan",
	"amber",
	"crimson",
	"aqua",
	"peach",
	"caramel",
	"mosse",
	"sand",
	"copper",
	"1977", // Special theme without a single color
	"frappe",
	"macchiato",
	"mocha",
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
	"cyan":      ui.NewRGBColor(0, 255, 255),   // Bright cyan - electric/neon
	"amber":     ui.NewRGBColor(255, 191, 0),   // Warm amber - golden yellow
	"crimson":   ui.NewRGBColor(220, 20, 60),   // Deep crimson red
	"aqua":      ui.NewRGBColor(0, 255, 200),   // Bright aqua/turquoise
	"peach":     ui.NewRGBColor(255, 180, 128), // Soft peach
	"caramel":   ui.NewRGBColor(255, 195, 128), // Warm caramel brown
	"mosse":     ui.NewRGBColor(173, 153, 113), // Olive mosse brown
	"sand":      ui.NewRGBColor(237, 201, 175), // Warm sandy beige
	"copper":    ui.NewRGBColor(184, 115, 51),  // Rich copper bronze
	"frappe":    CatppuccinFrappe.Mauve,
	"macchiato": CatppuccinMacchiato.Sapphire,
	"mocha":     CatppuccinMocha.Peach,
}

// bgColorOrder defines the order backgrounds cycle through with 'b' key
// To add a new background: add to bgColorOrder and bgColorMap
var bgColorOrder = []string{
	"clear",
	"mocha-base",
	"mocha-mantle",
	"mocha-crust",
	"macchiato-base",
	"frappe-base",
	"deep-space",
	"white",
	"grey",
	"black",
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
var catppuccinThemes = []string{"frappe", "macchiato", "mocha"}

// IsCatppuccinTheme returns true if the theme is a Catppuccin theme
func IsCatppuccinTheme(theme string) bool {
	return slices.Contains(catppuccinThemes, theme)
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

func applyThemeToStepCharts(color ui.Color) {
	stepCharts := []*w.StepChart{gpuHistoryChart, powerHistoryChart, memoryHistoryChart, cpuHistoryChart}
	for _, sc := range stepCharts {
		if sc != nil {
			sc.BorderStyle.Fg = color
			sc.BorderStyle.Bg = CurrentBgColor
			sc.TitleStyle.Fg = color
			sc.TitleStyle.Bg = CurrentBgColor
			sc.LineColors = []ui.Color{color}
		}
	}
}

func applyThemeToWidgets(color ui.Color, lightMode bool) {
	if processList != nil {
		processList.TextStyle = ui.NewStyle(color, CurrentBgColor)
		selectedFg := ui.NewRGBColor(2, 2, 2)
		if lightMode && color == ui.NewRGBColor(2, 2, 2) {
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

// resolveThemeColor resolves a color name (named, hex, or special) to a ui.Color
func resolveThemeColor(colorName string) (ui.Color, string) {
	is1977 := colorName == "1977"

	// Check if colorName is a hex color (e.g., "#9580FF")
	if IsHexColor(colorName) {
		if parsedColor, err := ParseHexColor(colorName); err == nil {
			return parsedColor, colorName
		}
	}

	// Try named color lookup
	if color, ok := colorMap[colorName]; ok {
		return color, colorName
	}

	// Special 1977 theme or default to green
	if is1977 {
		return ui.ColorGreen, colorName
	}
	return ui.ColorGreen, "green"
}

// setLightModeColors adjusts colors for light/dark mode
func setLightModeColors(lightMode bool, color ui.Color) ui.Color {
	if lightMode {
		BracketColor = ui.NewRGBColor(2, 2, 2)
		SecondaryTextColor = ui.NewRGBColor(2, 2, 2)
		if color == ui.ColorWhite {
			return ui.NewRGBColor(2, 2, 2)
		}
	} else {
		BracketColor = ui.ColorWhite
		SecondaryTextColor = 245
	}
	return color
}

// setGlobalTheme sets the global UI theme colors
func setGlobalTheme(color ui.Color) {
	ui.Theme.Block.Title.Fg = color
	ui.Theme.Block.Border.Fg = color
	ui.Theme.Paragraph.Text.Fg = color
	ui.Theme.Gauge.Label.Fg = color
	ui.Theme.Gauge.Bar = color
	ui.Theme.BarChart.Bars = []ui.Color{color}
}

// applyCatppuccinFullTheme applies Catppuccin theme with all widgets
func applyCatppuccinFullTheme(colorName string, palette *CatppuccinPalette, lightMode bool) {
	var primaryColor ui.Color
	switch colorName {
	case "frappe":
		primaryColor = palette.Mauve
	case "macchiato":
		primaryColor = palette.Sapphire
	case "mocha":
		primaryColor = palette.Peach
	default:
		primaryColor = palette.Lavender
	}

	ui.Theme.Block.Title.Fg = primaryColor
	ui.Theme.Block.Border.Fg = primaryColor
	ui.Theme.Paragraph.Text.Fg = palette.Text
	ui.Theme.Gauge.Label.Fg = palette.Subtext1
	ui.Theme.BarChart.Bars = []ui.Color{palette.Blue}

	applyCatppuccinThemeToGauges(palette)
	applyThemeToSparklines(primaryColor)
	applyThemeToStepCharts(primaryColor)
	applyThemeToWidgets(primaryColor, lightMode)

	if mainBlock != nil {
		mainBlock.BorderStyle.Fg = primaryColor
		mainBlock.TitleStyle.Fg = primaryColor
		mainBlock.TitleBottomStyle.Fg = primaryColor
	}
	if processList != nil {
		processList.TextStyle = ui.NewStyle(primaryColor, CurrentBgColor)
		processList.SelectedStyle = ui.NewStyle(palette.Base, primaryColor)
		processList.BorderStyle.Fg = primaryColor
		processList.BorderStyle.Bg = CurrentBgColor
		processList.TitleStyle.Fg = primaryColor
		processList.TitleStyle.Bg = CurrentBgColor
	}
}

func applyTheme(colorName string, lightMode bool) {
	color, resolvedName := resolveThemeColor(colorName)
	currentConfig.Theme = resolvedName
	color = setLightModeColors(lightMode, color)
	setGlobalTheme(color)

	if resolvedName == "1977" {
		update1977GaugeColors()
		applyThemeToSparklines(color)
		applyThemeToStepCharts(color)
		applyThemeToWidgets(color, lightMode)
		return
	}

	if palette := GetCatppuccinPalette(resolvedName); palette != nil {
		applyCatppuccinFullTheme(resolvedName, palette, lightMode)
		return
	}

	applyThemeToGauges(color)
	applyThemeToSparklines(color)
	applyThemeToStepCharts(color)
	applyThemeToWidgets(color, lightMode)
}

func GetThemeColor(colorName string) ui.Color {
	// Check if colorName is a hex color
	if IsHexColor(colorName) {
		if color, err := ParseHexColor(colorName); err == nil {
			return color
		}
	}
	color, ok := colorMap[colorName]
	if !ok {
		return ui.ColorGreen
	}
	return color
}

func GetThemeColorWithLightMode(colorName string, lightMode bool) ui.Color {
	color := GetThemeColor(colorName)
	if lightMode && color == ui.ColorWhite {
		return ui.NewRGBColor(2, 2, 2)
	}
	return color
}

// themeHexMap maps theme names to their hex color strings for text rendering
var themeHexMap = map[string]string{
	"coffee":   "#C1A589",
	"mint":     "#98FF98",
	"babyblue": "#89CFF0",
	"indigo":   "#4B0082",
	"teal":     "#008080",
	"coral":    "#FF7F50",
	"lavender": "#BABBF1",
	"rose":     "#FF007F",
	"cyan":     "#00FFFF",
	"amber":    "#FFBF00",
	"crimson":  "#DC143C",
	"aqua":     "#00FFC8",
	"peach":    "#FFB480",
	"caramel":  "#FFC380",
	"mosse":    "#AD9971",
	"sand":     "#EDC9AF",
	"copper":   "#B87333",
	"1977":     "green",
}

func resolveThemeColorString(theme string) string {
	// If it's already a hex color, return as-is
	if IsHexColor(theme) {
		return theme
	}
	if hex, ok := themeHexMap[theme]; ok {
		return hex
	}
	return theme
}

func GetProcessTextColor(isCurrentUser bool) string {
	if IsLightMode {
		if isCurrentUser {
			color := GetThemeColorWithLightMode(currentConfig.Theme, true)
			if color == ui.NewRGBColor(2, 2, 2) {
				return "#020202"
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
	return "#888888" // Grey for non-current-user (root/system) processes
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
// Accepts named backgrounds (from bgColorMap) or hex colors
func applyBackground(bgName string) {
	var bgColor ui.Color
	var ok bool

	// Check if bgName is a hex color
	if IsHexColor(bgName) {
		if parsed, err := ParseHexColor(bgName); err == nil {
			bgColor = parsed
			ok = true
		}
	}

	// Try named background lookup if not a hex color
	if !ok {
		bgColor, ok = bgColorMap[bgName]
	}

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
	applyBackgroundToStepCharts(bgColor)
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

func applyBackgroundToStepCharts(bgColor ui.Color) {
	stepCharts := []*w.StepChart{gpuHistoryChart, powerHistoryChart, memoryHistoryChart, cpuHistoryChart}
	for _, sc := range stepCharts {
		if sc != nil {
			sc.BackgroundColor = bgColor
			sc.BorderStyle.Bg = bgColor
			sc.TitleStyle.Bg = bgColor
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

// applyCustomThemeFile loads and applies custom theme from ~/.mactop/theme.json
// Returns (appliedForeground, appliedBackground) to indicate which colors were set
func applyCustomThemeFile() (bool, bool) {
	theme := loadThemeFile()
	if theme == nil {
		return false, false
	}

	appliedFg := false
	appliedBg := false

	// Apply custom background first (so foreground color applies on top)
	if theme.Background != "" && IsHexColor(theme.Background) {
		applyBackground(theme.Background)
		currentConfig.Background = theme.Background
		appliedBg = true
	}

	// Apply foreground color (primary UI color)
	if theme.Foreground != "" && IsHexColor(theme.Foreground) {
		applyTheme(theme.Foreground, IsLightMode)
		currentConfig.Theme = theme.Foreground
		currentConfig.CustomTheme = theme
		appliedFg = true
	}

	return appliedFg, appliedBg
}
