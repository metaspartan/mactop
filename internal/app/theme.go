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

// 1977 theme uses fixed per-component gauge colors regardless of cycle position

// --- Style helpers: centralize the repeated 3-5 line styling patterns ---

func styleGauge(g *w.Gauge, color, labelColor ui.Color) {
	if g == nil {
		return
	}
	g.BarColor = color
	g.BorderStyle.Fg = color
	g.BorderStyle.Bg = CurrentBgColor
	g.TitleStyle.Fg = color
	g.TitleStyle.Bg = CurrentBgColor
	g.LabelStyle = ui.NewStyle(labelColor, CurrentBgColor)
}

func styleParagraph(p *w.Paragraph, color ui.Color) {
	if p == nil {
		return
	}
	p.BorderStyle.Fg = color
	p.BorderStyle.Bg = CurrentBgColor
	p.TitleStyle.Fg = color
	p.TitleStyle.Bg = CurrentBgColor
	p.TextStyle = ui.NewStyle(color, CurrentBgColor)
}

func styleSparkline(s *w.Sparkline, color ui.Color) {
	if s == nil {
		return
	}
	s.LineColor = color
	s.TitleStyle = ui.NewStyle(color, CurrentBgColor)
}

func styleSparklineGroup(g *w.SparklineGroup, color ui.Color) {
	if g == nil {
		return
	}
	g.BorderStyle.Fg = color
	g.BorderStyle.Bg = CurrentBgColor
	g.TitleStyle.Fg = color
	g.TitleStyle.Bg = CurrentBgColor
}

func styleStepChart(sc *w.StepChart, color ui.Color) {
	if sc == nil {
		return
	}
	sc.BorderStyle.Fg = color
	sc.BorderStyle.Bg = CurrentBgColor
	sc.TitleStyle.Fg = color
	sc.TitleStyle.Bg = CurrentBgColor
	sc.LineColors = []ui.Color{color}
}

func update1977GaugeColors() {
	styleGauge(cpuGauge, ui.ColorGreen, SecondaryTextColor)
	styleGauge(gpuGauge, ui.ColorMagenta, SecondaryTextColor)
	styleGauge(memoryGauge, ui.ColorBlue, SecondaryTextColor)
	styleGauge(aneGauge, ui.ColorRed, SecondaryTextColor)
}

func applyThemeToGauges(color ui.Color) {
	styleGauge(cpuGauge, color, SecondaryTextColor)
	styleGauge(gpuGauge, color, SecondaryTextColor)
	styleGauge(memoryGauge, color, SecondaryTextColor)
	styleGauge(aneGauge, color, SecondaryTextColor)
}

func applyCatppuccinThemeToGauges(palette *CatppuccinPalette) {
	styleGauge(cpuGauge, palette.Green, palette.Subtext0)     // CPU = Green (success/performance)
	styleGauge(gpuGauge, palette.Blue, palette.Subtext0)      // GPU = Blue (info/secondary compute)
	styleGauge(memoryGauge, palette.Yellow, palette.Subtext0) // Memory = Yellow (resource usage)
	styleGauge(aneGauge, palette.Lavender, palette.Subtext0)  // ANE = Lavender (AI/neural)
}

// resolveCustomColor resolves a per-component hex color, falling back to foregroundColor.
func resolveCustomColor(specificKey string, foregroundColor ui.Color) ui.Color {
	if specificKey != "" && IsHexColor(specificKey) {
		if color, err := ParseHexColor(specificKey); err == nil {
			return color
		}
	}
	return foregroundColor
}

// applyCustomGaugeColors applies per-component gauge colors from custom theme.
func applyCustomGaugeColors(theme *CustomThemeConfig, fgColor ui.Color) {
	styleGauge(cpuGauge, resolveCustomColor(theme.CPU, fgColor), SecondaryTextColor)
	styleGauge(gpuGauge, resolveCustomColor(theme.GPU, fgColor), SecondaryTextColor)
	styleGauge(memoryGauge, resolveCustomColor(theme.Memory, fgColor), SecondaryTextColor)
	styleGauge(aneGauge, resolveCustomColor(theme.ANE, fgColor), SecondaryTextColor)
}

// applyCustomWidgetColors applies per-component widget colors from custom theme.
func applyCustomWidgetColors(theme *CustomThemeConfig, fgColor ui.Color) {
	// Sparklines
	powerColor := resolveCustomColor(theme.Power, fgColor)
	styleSparkline(sparkline, powerColor)
	styleSparklineGroup(sparklineGroup, powerColor)

	gpuColor := resolveCustomColor(theme.GPU, fgColor)
	styleSparkline(gpuSparkline, gpuColor)
	styleSparklineGroup(gpuSparklineGroup, gpuColor)

	netColor := resolveCustomColor(theme.Network, fgColor)
	styleSparkline(tbNetSparklineIn, netColor)
	styleSparkline(tbNetSparklineOut, netColor)
	styleSparklineGroup(tbNetSparklineGroup, netColor)

	// Step charts
	styleStepChart(gpuHistoryChart, gpuColor)
	styleStepChart(powerHistoryChart, powerColor)
	styleStepChart(memoryHistoryChart, resolveCustomColor(theme.Memory, fgColor))
	styleStepChart(cpuHistoryChart, resolveCustomColor(theme.CPU, fgColor))

	// Paragraphs
	styleParagraph(PowerChart, powerColor)
	styleParagraph(NetworkInfo, netColor)
	styleParagraph(tbInfoParagraph, resolveCustomColor(theme.Thunderbolt, fgColor))
	styleParagraph(infoParagraph, fgColor) // info box uses foreground directly
	styleParagraph(helpText, fgColor)
	styleParagraph(modelText, resolveCustomColor(theme.SystemInfo, fgColor))

	// Process list (needs special selected-style contrast logic)
	if processList != nil {
		color := resolveCustomColor(theme.ProcessList, fgColor)
		processList.BorderStyle.Fg = color
		processList.TitleStyle.Fg = color
		processList.TextStyle = ui.NewStyle(color, CurrentBgColor)

		selectedFg := ui.NewRGBColor(2, 2, 2)
		if theme.ProcessListSelected != "" && IsHexColor(theme.ProcessListSelected) {
			if parsed, err := ParseHexColor(theme.ProcessListSelected); err == nil {
				selectedFg = parsed
			}
		} else {
			colorForContrast := theme.ProcessList
			if colorForContrast == "" || !IsHexColor(colorForContrast) {
				colorForContrast = theme.Foreground
			}
			if !IsLightHexColor(colorForContrast) {
				selectedFg = ui.ColorWhite
			}
		}
		processList.SelectedStyle = ui.NewStyle(selectedFg, color)
	}

	// CPU Cores widget
	if cpuCoreWidget != nil {
		color := resolveCustomColor(theme.CPU, fgColor)
		cpuCoreWidget.BorderStyle.Fg = color
		cpuCoreWidget.TitleStyle.Fg = color
	}
}

// applyCustomPerComponentColors applies per-component colors from custom theme.
// Falls back to foreground color if component color not specified.
func applyCustomPerComponentColors(theme *CustomThemeConfig, foregroundColor ui.Color) {
	applyCustomGaugeColors(theme, foregroundColor)
	applyCustomWidgetColors(theme, foregroundColor)
}

func applyThemeToSparklines(color ui.Color) {
	styleSparkline(sparkline, color)
	styleSparklineGroup(sparklineGroup, color)
	styleSparkline(gpuSparkline, color)
	styleSparklineGroup(gpuSparklineGroup, color)
	styleSparkline(tbNetSparklineIn, color)
	styleSparkline(tbNetSparklineOut, color)
	styleSparklineGroup(tbNetSparklineGroup, color)
}

func applyThemeToStepCharts(color ui.Color) {
	for _, sc := range []*w.StepChart{gpuHistoryChart, powerHistoryChart, memoryHistoryChart, cpuHistoryChart} {
		styleStepChart(sc, color)
	}
}

func applyThemeToWidgets(color ui.Color, lightMode bool) {
	// Process list needs special selected-style logic
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

	// Paragraphs
	styleParagraph(NetworkInfo, color)
	styleParagraph(PowerChart, color)
	styleParagraph(modelText, color)
	styleParagraph(helpText, color)
	styleParagraph(tbInfoParagraph, color)
	styleParagraph(infoParagraph, color)

	// CPU Cores widget
	if cpuCoreWidget != nil {
		cpuCoreWidget.BorderStyle.Fg = color
		cpuCoreWidget.BorderStyle.Bg = CurrentBgColor
		cpuCoreWidget.TitleStyle.Fg = color
		cpuCoreWidget.TitleStyle.Bg = CurrentBgColor
	}

	// Main block
	if mainBlock != nil {
		mainBlock.BorderStyle.Fg = color
		mainBlock.BorderStyle.Bg = CurrentBgColor
		mainBlock.TitleStyle.Fg = color
		mainBlock.TitleStyle.Bg = CurrentBgColor
		mainBlock.TitleBottomStyle.Fg = color
		mainBlock.TitleBottomStyle.Bg = CurrentBgColor
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
	if isCurrentUser {
		// Prioritize custom ProcessList color if valid
		if currentConfig.CustomTheme != nil && currentConfig.CustomTheme.ProcessList != "" {
			if IsHexColor(currentConfig.CustomTheme.ProcessList) {
				return currentConfig.CustomTheme.ProcessList
			}
		}

		if IsLightMode {
			color := GetThemeColorWithLightMode(currentConfig.Theme, true)
			if color == ui.NewRGBColor(2, 2, 2) {
				return "#020202"
			}
			if IsCatppuccinTheme(currentConfig.Theme) {
				return GetCatppuccinHex(currentConfig.Theme, "Text")
			}
			return resolveThemeColorString(currentConfig.Theme)
		}

		if IsCatppuccinTheme(currentConfig.Theme) {
			return GetCatppuccinHex(currentConfig.Theme, "Primary")
		}
		return resolveThemeColorString(currentConfig.Theme)
	}
	// Non-current user processes
	if currentConfig.CustomTheme != nil && currentConfig.CustomTheme.ProcessListDim != "" {
		if IsHexColor(currentConfig.CustomTheme.ProcessListDim) {
			return currentConfig.CustomTheme.ProcessListDim
		}
	}
	if IsLightMode {
		return "240"
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

	// When cycling themes, clear the custom theme configuration to prevent
	// lingering custom colors from overriding the selected preset theme.
	currentConfig.CustomTheme = nil

	applyTheme(themeOrder[nextIndex], IsLightMode)
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

// hasCustomComponentColors returns true if any per-component color is specified.
func hasCustomComponentColors(t *CustomThemeConfig) bool {
	return t.CPU != "" || t.GPU != "" || t.Memory != "" || t.ANE != "" ||
		t.Network != "" || t.Power != "" || t.Thunderbolt != "" ||
		t.ProcessList != "" || t.ProcessListDim != "" ||
		t.ProcessListSelected != "" || t.SystemInfo != ""
}

// applyCustomThemeFile loads and applies custom theme from the mactop config directory
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

		// Apply per-component colors if any are specified
		if hasCustomComponentColors(theme) {
			foregroundColor, _ := ParseHexColor(theme.Foreground)
			applyCustomPerComponentColors(theme, foregroundColor)
		}
	}

	return appliedFg, appliedBg
}
