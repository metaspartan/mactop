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

	// Per-gauge colors (override foreground if specified)
	CPU    string `json:"cpu,omitempty"`    // CPU gauge color (also affects cores, history)
	GPU    string `json:"gpu,omitempty"`    // GPU gauge color (also affects sparkline, history)
	Memory string `json:"memory,omitempty"` // Memory gauge color (also affects history)
	ANE    string `json:"ane,omitempty"`    // ANE (Apple Neural Engine) gauge color

	Network     string `json:"network,omitempty"`     // Network & Disk box color (also affects network sparklines)
	Power       string `json:"power,omitempty"`       // Power chart box color (also affects power sparkline, history)
	Thunderbolt string `json:"thunderbolt,omitempty"` // Thunderbolt/RDMA box color

	ProcessList string `json:"processList,omitempty"` // Process list color
	SystemInfo  string `json:"systemInfo,omitempty"`  // Apple Silicon system info box color
}

// MenuBarConfig controls the appearance of the --menubar status item
type MenuBarConfig struct {
	StatusBarWidth  int    `json:"status_bar_width,omitempty"` // Width of each bar in status bar (px, default: 24)
	SparklineWidth  int    `json:"sparkline_width,omitempty"`  // Width of sparkline graphs in dropdown (px, default: 300)
	SparklineHeight int    `json:"sparkline_height,omitempty"` // Height of sparkline graphs in dropdown (px, default: 40)
	ShowCPU         *bool  `json:"show_cpu,omitempty"`         // Show CPU bar in status bar (default: true)
	ShowGPU         *bool  `json:"show_gpu,omitempty"`         // Show GPU bar in status bar (default: true)
	ShowANE         *bool  `json:"show_ane,omitempty"`         // Show ANE bar in status bar (default: true)
	ShowMemory      *bool  `json:"show_memory,omitempty"`      // Show Memory bar in status bar (default: true)
	ShowPower       *bool  `json:"show_power,omitempty"`       // Show power watts text (default: true)
	ShowPercent     *bool  `json:"show_percent,omitempty"`     // Show percentage text next to bars (default: false)
	FontSize        int    `json:"font_size,omitempty"`        // Font size for status bars (px, default: 10)
	PowerFontSize   int    `json:"power_font_size,omitempty"`  // Font size for power watts (px, default: 11)
	CPUColor        string `json:"cpu_color,omitempty"`        // Hex color for CPU bar (default: systemGreen)
	GPUColor        string `json:"gpu_color,omitempty"`        // Hex color for GPU bar (default: systemCyan)
	ANEColor        string `json:"ane_color,omitempty"`        // Hex color for ANE bar (default: systemPurple)
	MemColor        string `json:"mem_color,omitempty"`        // Hex color for Memory bar (default: systemOrange)
}

type AppConfig struct {
	DefaultLayout string             `json:"default_layout"`
	Theme         string             `json:"theme"`
	Background    string             `json:"background,omitempty"`
	SortColumn    *int               `json:"sort_column,omitempty"`
	SortReverse   bool               `json:"sort_reverse"`
	CustomTheme   *CustomThemeConfig `json:"custom_theme,omitempty"`
	MenuBar       *MenuBarConfig     `json:"menubar,omitempty"`
}

// loadMenuBarConfig returns the menu bar config with defaults applied
func loadMenuBarConfig() MenuBarConfig {
	cfg := MenuBarConfig{
		StatusBarWidth:  24,
		SparklineWidth:  300,
		SparklineHeight: 40,
		FontSize:        10,
		PowerFontSize:   11,
	}
	if currentConfig.MenuBar != nil {
		m := currentConfig.MenuBar
		if m.StatusBarWidth > 0 {
			cfg.StatusBarWidth = m.StatusBarWidth
		}
		if m.SparklineWidth > 0 {
			cfg.SparklineWidth = m.SparklineWidth
		}
		if m.SparklineHeight > 0 {
			cfg.SparklineHeight = m.SparklineHeight
		}
		if m.FontSize > 0 {
			cfg.FontSize = m.FontSize
		}
		if m.PowerFontSize > 0 {
			cfg.PowerFontSize = m.PowerFontSize
		}
		if m.ShowCPU != nil {
			cfg.ShowCPU = m.ShowCPU
		}
		if m.ShowGPU != nil {
			cfg.ShowGPU = m.ShowGPU
		}
		if m.ShowANE != nil {
			cfg.ShowANE = m.ShowANE
		}
		if m.ShowMemory != nil {
			cfg.ShowMemory = m.ShowMemory
		}
		if m.ShowPower != nil {
			cfg.ShowPower = m.ShowPower
		}
		if m.ShowPercent != nil {
			cfg.ShowPercent = m.ShowPercent
		}
	}
	return cfg
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
//	  "foreground": "#9580FF",
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
