package app

import (
	ui "github.com/metaspartan/gotui/v5"
)

type CatppuccinPalette struct {
	Rosewater ui.Color
	Flamingo  ui.Color
	Pink      ui.Color
	Mauve     ui.Color
	Red       ui.Color
	Maroon    ui.Color
	Peach     ui.Color
	Yellow    ui.Color
	Green     ui.Color
	Teal      ui.Color
	Sky       ui.Color
	Sapphire  ui.Color
	Blue      ui.Color
	Lavender  ui.Color
	Text      ui.Color
	Subtext1  ui.Color
	Subtext0  ui.Color
	Overlay2  ui.Color
	Overlay1  ui.Color
	Overlay0  ui.Color
	Surface2  ui.Color
	Surface1  ui.Color
	Surface0  ui.Color
	Base      ui.Color
	Mantle    ui.Color
	Crust     ui.Color
}

func rgb(r, g, b int32) ui.Color {
	return ui.NewRGBColor(r, g, b)
}

var (
	CatppuccinLatte = CatppuccinPalette{
		Rosewater: rgb(220, 138, 120),
		Flamingo:  rgb(221, 120, 120),
		Pink:      rgb(234, 118, 203),
		Mauve:     rgb(136, 57, 239),
		Red:       rgb(210, 15, 57),
		Maroon:    rgb(230, 69, 83),
		Peach:     rgb(254, 100, 11),
		Yellow:    rgb(223, 142, 29),
		Green:     rgb(64, 160, 43),
		Teal:      rgb(23, 146, 153),
		Sky:       rgb(4, 165, 229),
		Sapphire:  rgb(32, 159, 181),
		Blue:      rgb(30, 102, 245),
		Lavender:  rgb(114, 135, 253),
		Text:      rgb(76, 79, 105),
		Subtext1:  rgb(92, 95, 119),
		Subtext0:  rgb(108, 111, 133),
		Overlay2:  rgb(124, 127, 147),
		Overlay1:  rgb(140, 143, 161),
		Overlay0:  rgb(156, 160, 176),
		Surface2:  rgb(172, 176, 190),
		Surface1:  rgb(188, 192, 204),
		Surface0:  rgb(204, 208, 218),
		Base:      rgb(239, 241, 245),
		Mantle:    rgb(230, 233, 239),
		Crust:     rgb(220, 224, 232),
	}

	CatppuccinFrappe = CatppuccinPalette{
		Rosewater: rgb(242, 213, 207),
		Flamingo:  rgb(238, 190, 190),
		Pink:      rgb(244, 184, 228),
		Mauve:     rgb(202, 158, 230),
		Red:       rgb(231, 130, 132),
		Maroon:    rgb(234, 153, 156),
		Peach:     rgb(239, 159, 118),
		Yellow:    rgb(229, 200, 144),
		Green:     rgb(166, 209, 137),
		Teal:      rgb(129, 200, 190),
		Sky:       rgb(153, 209, 219),
		Sapphire:  rgb(133, 193, 220),
		Blue:      rgb(140, 170, 238),
		Lavender:  rgb(186, 187, 241),
		Text:      rgb(198, 208, 245),
		Subtext1:  rgb(181, 191, 226),
		Subtext0:  rgb(165, 173, 203),
		Overlay2:  rgb(148, 156, 187),
		Overlay1:  rgb(131, 139, 167),
		Overlay0:  rgb(115, 121, 148),
		Surface2:  rgb(98, 104, 128),
		Surface1:  rgb(81, 87, 109),
		Surface0:  rgb(65, 69, 89),
		Base:      rgb(48, 52, 70),
		Mantle:    rgb(41, 44, 60),
		Crust:     rgb(35, 38, 52),
	}

	CatppuccinMacchiato = CatppuccinPalette{
		Rosewater: rgb(244, 219, 214),
		Flamingo:  rgb(240, 198, 198),
		Pink:      rgb(245, 189, 230),
		Mauve:     rgb(198, 160, 246),
		Red:       rgb(237, 135, 150),
		Maroon:    rgb(238, 153, 160),
		Peach:     rgb(245, 169, 127),
		Yellow:    rgb(238, 212, 159),
		Green:     rgb(166, 218, 149),
		Teal:      rgb(139, 213, 202),
		Sky:       rgb(145, 215, 227),
		Sapphire:  rgb(125, 196, 228),
		Blue:      rgb(138, 173, 244),
		Lavender:  rgb(183, 189, 248),
		Text:      rgb(202, 211, 245),
		Subtext1:  rgb(184, 192, 224),
		Subtext0:  rgb(165, 173, 203),
		Overlay2:  rgb(147, 154, 183),
		Overlay1:  rgb(128, 135, 162),
		Overlay0:  rgb(110, 115, 141),
		Surface2:  rgb(91, 96, 120),
		Surface1:  rgb(73, 77, 100),
		Surface0:  rgb(54, 58, 79),
		Base:      rgb(36, 39, 58),
		Mantle:    rgb(30, 32, 48),
		Crust:     rgb(24, 25, 38),
	}

	CatppuccinMocha = CatppuccinPalette{
		Rosewater: rgb(245, 224, 220),
		Flamingo:  rgb(242, 205, 205),
		Pink:      rgb(245, 194, 231),
		Mauve:     rgb(203, 166, 247),
		Red:       rgb(243, 139, 168),
		Maroon:    rgb(235, 160, 172),
		Peach:     rgb(250, 179, 135),
		Yellow:    rgb(249, 226, 175),
		Green:     rgb(166, 227, 161),
		Teal:      rgb(148, 226, 213),
		Sky:       rgb(137, 220, 235),
		Sapphire:  rgb(116, 199, 236),
		Blue:      rgb(137, 180, 250),
		Lavender:  rgb(180, 190, 254),
		Text:      rgb(205, 214, 244),
		Subtext1:  rgb(186, 194, 222),
		Subtext0:  rgb(166, 173, 200),
		Overlay2:  rgb(147, 153, 178),
		Overlay1:  rgb(127, 132, 156),
		Overlay0:  rgb(108, 112, 134),
		Surface2:  rgb(88, 91, 112),
		Surface1:  rgb(69, 71, 90),
		Surface0:  rgb(49, 50, 68),
		Base:      rgb(30, 30, 46),
		Mantle:    rgb(24, 24, 37),
		Crust:     rgb(17, 17, 27),
	}
)

var (
	LatteHex = map[string]string{
		"Primary": "#7287fd", // Lavender
		"Text":    "#4c4f69",
		"Base":    "#eff1f5",
	}
	FrappeHex = map[string]string{
		"Primary": "#ca9ee6", // Mauve (purple)
		"Text":    "#c6d0f5",
		"Base":    "#303446",
	}
	MacchiatoHex = map[string]string{
		"Primary": "#7dc4e4", // Sapphire (blue)
		"Text":    "#cad3f5",
		"Base":    "#24273a",
	}
	MochaHex = map[string]string{
		"Primary": "#fab387", // Peach (orange)
		"Text":    "#cdd6f4",
		"Base":    "#1e1e2e",
	}
)

func GetCatppuccinHex(theme, colorName string) string {
	var hexMap map[string]string
	switch theme {
	case "latte":
		hexMap = LatteHex
	case "frappe":
		hexMap = FrappeHex
	case "macchiato":
		hexMap = MacchiatoHex
	case "mocha":
		hexMap = MochaHex
	default:
		return "#ffffff"
	}
	if val, ok := hexMap[colorName]; ok {
		return val
	}
	return "#ffffff"
}

func GetCatppuccinPalette(name string) *CatppuccinPalette {
	switch name {
	case "latte":
		return &CatppuccinLatte
	case "frappe":
		return &CatppuccinFrappe
	case "macchiato":
		return &CatppuccinMacchiato
	case "mocha":
		return &CatppuccinMocha
	default:
		return nil
	}
}
