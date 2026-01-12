package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
)

// hexColorRegex matches valid 3 or 6 character hex colors with optional # prefix
var hexColorRegex = regexp.MustCompile(`^#?([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)

// IsHexColor checks if the given string is a valid hex color
func IsHexColor(s string) bool {
	return hexColorRegex.MatchString(s)
}

// ParseHexColor converts a hex color string to a ui.Color
// Accepts formats: #RRGGBB, RRGGBB, #RGB, RGB
func ParseHexColor(hex string) (ui.Color, error) {
	hex = strings.TrimPrefix(hex, "#")
	hex = strings.ToUpper(hex)

	if !hexColorRegex.MatchString(hex) && !hexColorRegex.MatchString("#"+hex) {
		return ui.ColorClear, fmt.Errorf("invalid hex color: %s", hex)
	}

	// Expand 3-char hex to 6-char (e.g., "ABC" -> "AABBCC")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return ui.ColorClear, fmt.Errorf("invalid red component: %v", err)
	}

	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return ui.ColorClear, fmt.Errorf("invalid green component: %v", err)
	}

	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return ui.ColorClear, fmt.Errorf("invalid blue component: %v", err)
	}

	return ui.NewRGBColor(int32(r), int32(g), int32(b)), nil
}

// MustParseHexColor parses a hex color or returns the fallback color on error
func MustParseHexColor(hex string, fallback ui.Color) ui.Color {
	color, err := ParseHexColor(hex)
	if err != nil {
		return fallback
	}
	return color
}
