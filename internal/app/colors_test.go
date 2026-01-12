package app

import (
	"testing"

	ui "github.com/metaspartan/gotui/v5"
)

func TestIsHexColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Valid 6-char with hash", "#FFFFFF", true},
		{"Valid 6-char without hash", "FFFFFF", true},
		{"Valid 3-char with hash", "#FFF", true},
		{"Valid 3-char without hash", "FFF", true},
		{"Valid lowercase", "#aabbcc", true},
		{"Valid mixed case", "#AaBbCc", true},
		{"Dracula purple", "#9580FF", true},
		{"Dracula background", "#22212C", true},
		{"Invalid too short", "#FF", false},
		{"Invalid too long", "#FFFFFFF", false},
		{"Invalid characters", "#GGGGGG", false},
		{"Invalid named color", "green", false},
		{"Empty string", "", false},
		{"Just hash", "#", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHexColor(tt.input); got != tt.want {
				t.Errorf("IsHexColor(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantR   int32
		wantG   int32
		wantB   int32
		wantErr bool
	}{
		{"White", "#FFFFFF", 255, 255, 255, false},
		{"Black", "#000000", 0, 0, 0, false},
		{"Red", "#FF0000", 255, 0, 0, false},
		{"Green", "#00FF00", 0, 255, 0, false},
		{"Blue", "#0000FF", 0, 0, 255, false},
		{"Without hash", "AABBCC", 170, 187, 204, false},
		{"Short form white", "#FFF", 255, 255, 255, false},
		{"Short form black", "000", 0, 0, 0, false},
		{"Short form ABC", "#ABC", 170, 187, 204, false},
		{"Dracula purple", "#9580FF", 149, 128, 255, false},
		{"Dracula background", "#22212C", 34, 33, 44, false},
		{"Lowercase", "#aabbcc", 170, 187, 204, false},
		{"Invalid", "notacolor", 0, 0, 0, true},
		{"Empty", "", 0, 0, 0, true},
		{"Invalid chars", "#GGHHII", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHexColor(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHexColor(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			expected := ui.NewRGBColor(tt.wantR, tt.wantG, tt.wantB)
			if got != expected {
				t.Errorf("ParseHexColor(%q) = %v, want RGB(%d,%d,%d)", tt.input, got, tt.wantR, tt.wantG, tt.wantB)
			}
		})
	}
}

func TestMustParseHexColor(t *testing.T) {
	fallback := ui.ColorGreen

	// Valid color should be parsed
	got := MustParseHexColor("#FF0000", fallback)
	expected := ui.NewRGBColor(255, 0, 0)
	if got != expected {
		t.Errorf("MustParseHexColor valid = %v, want %v", got, expected)
	}

	// Invalid color should return fallback
	got = MustParseHexColor("invalid", fallback)
	if got != fallback {
		t.Errorf("MustParseHexColor invalid = %v, want fallback %v", got, fallback)
	}
}
