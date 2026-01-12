package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CustomThemeConfig holds custom hex color values for theming
type CustomThemeConfig struct {
	Foreground string `json:"foreground,omitempty"` // Primary UI color (borders, titles, gauges)
	Background string `json:"background,omitempty"` // Background color
}

type AppConfig struct {
	DefaultLayout string             `json:"default_layout"`
	Theme         string             `json:"theme"`
	Background    string             `json:"background,omitempty"`
	SortColumn    *int               `json:"sort_column,omitempty"`
	SortReverse   bool               `json:"sort_reverse"`
	CustomTheme   *CustomThemeConfig `json:"custom_theme,omitempty"`
}

var currentConfig AppConfig

// migrateThemeName converts old 'catppuccin-*' theme names to short form
func migrateThemeName(theme string) string {
	oldToNew := map[string]string{
		"catppuccin-latte":     "coffee",
		"catppuccin-frappe":    "frappe",
		"catppuccin-macchiato": "macchiato",
		"catppuccin-mocha":     "mocha",
	}
	if newName, ok := oldToNew[theme]; ok {
		return newName
	}
	// Also handle any "catppuccin-" prefix generically
	if after, ok := strings.CutPrefix(theme, "catppuccin-"); ok {
		return after
	}
	return theme
}

func loadConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		currentConfig = AppConfig{DefaultLayout: "default"}
		return
	}
	configPath := filepath.Join(homeDir, ".mactop", "config.json")

	file, err := os.ReadFile(configPath)
	if err != nil {
		currentConfig = AppConfig{DefaultLayout: "default"}
		return
	}

	err = json.Unmarshal(file, &currentConfig)
	if err != nil {
		currentConfig = AppConfig{DefaultLayout: "default"}
	}

	// Migrate old theme names
	if currentConfig.Theme != "" {
		newTheme := migrateThemeName(currentConfig.Theme)
		if newTheme != currentConfig.Theme {
			currentConfig.Theme = newTheme
			// Save the migrated config
			saveConfig()
		}
	}
}

func saveConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configDir := filepath.Join(homeDir, ".mactop")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return
	}
	configPath := filepath.Join(configDir, "config.json")

	data, err := json.MarshalIndent(currentConfig, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(configPath, data, 0644)
}

// loadThemeFile loads custom theme from ~/.mactop/theme.json if it exists
// Theme file format:
//
//	{
//	  "accent": "#9580FF",
//	  "foreground": "#F8F8F2",
//	  "background": "#22212C"
//	}
func loadThemeFile() *CustomThemeConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	themePath := filepath.Join(homeDir, ".mactop", "theme.json")
	file, err := os.ReadFile(themePath)
	if err != nil {
		return nil
	}

	var theme CustomThemeConfig
	if err := json.Unmarshal(file, &theme); err != nil {
		return nil
	}

	// Validate at least one color is set
	if theme.Foreground == "" && theme.Background == "" {
		return nil
	}

	return &theme
}
