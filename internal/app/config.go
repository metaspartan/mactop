package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type AppConfig struct {
	DefaultLayout string `json:"default_layout"`
	Theme         string `json:"theme"`
	SortColumn    *int   `json:"sort_column,omitempty"`
	SortReverse   bool   `json:"sort_reverse"`
}

var currentConfig AppConfig

// migrateThemeName converts old 'catppuccin-*' theme names to short form
func migrateThemeName(theme string) string {
	oldToNew := map[string]string{
		"catppuccin-latte":     "latte",
		"catppuccin-frappe":    "frappe",
		"catppuccin-macchiato": "macchiato",
		"catppuccin-mocha":     "mocha",
	}
	if newName, ok := oldToNew[theme]; ok {
		return newName
	}
	// Also handle any "catppuccin-" prefix generically
	if strings.HasPrefix(theme, "catppuccin-") {
		return strings.TrimPrefix(theme, "catppuccin-")
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
