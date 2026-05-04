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

	ProcessList         string `json:"processList,omitempty"`         // Process list color
	ProcessListDim      string `json:"processListDim,omitempty"`      // Non-current-user process text color (default: grey)
	ProcessListSelected string `json:"processListSelected,omitempty"` // Selected row foreground color (default: auto contrast)
	SystemInfo          string `json:"systemInfo,omitempty"`          // Apple Silicon system info box color
}

// MenuBarConfig controls the appearance of the --menubar status item
type MenuBarConfig struct {
	StatusBarWidth  int    `json:"status_bar_width,omitempty"`  // Width of each bar in status bar (px, default: 24)
	StatusBarHeight int    `json:"status_bar_height,omitempty"` // Height/thickness of status bar (px, default: 18)
	SparklineWidth  int    `json:"sparkline_width,omitempty"`   // Width of sparkline graphs in dropdown (px, default: 300)
	SparklineHeight int    `json:"sparkline_height,omitempty"`  // Height of sparkline graphs in dropdown (px, default: 40)
	ShowCPU         *bool  `json:"show_cpu,omitempty"`          // Show CPU bar in status bar (default: true)
	ShowGPU         *bool  `json:"show_gpu,omitempty"`          // Show GPU bar in status bar (default: true)
	ShowANE         *bool  `json:"show_ane,omitempty"`          // Show ANE bar in status bar (default: true)
	ShowMemory      *bool  `json:"show_memory,omitempty"`       // Show Memory bar in status bar (default: true)
	ShowPower       *bool  `json:"show_power,omitempty"`        // Show power watts text (default: true)
	ShowPercent     *bool  `json:"show_percent,omitempty"`      // Show percentage text next to bars (default: false)
	FontSize        int    `json:"font_size,omitempty"`         // Font size for status bars (px, default: 10)
	PowerFontSize   int    `json:"power_font_size,omitempty"`   // Font size for power watts (px, default: 11)
	CPUColor        string `json:"cpu_color,omitempty"`         // Hex color for CPU bar (default: systemGreen)
	GPUColor        string `json:"gpu_color,omitempty"`         // Hex color for GPU bar (default: systemCyan)
	ANEColor        string `json:"ane_color,omitempty"`         // Hex color for ANE bar (default: systemPurple)
	MemColor        string `json:"mem_color,omitempty"`         // Hex color for Memory bar (default: systemOrange)
	LabelColor      string `json:"label_color,omitempty"`       // Hex color for letter labels, percent text, and wattage text (default: system labelColor)
	BarOrder        string `json:"bar_order,omitempty"`         // Comma-separated order of status bar items (default: cpu,gpu,ane,memory)
}

// OverlayConfig controls the appearance and behavior of the --overlay HUD
type OverlayConfig struct {
	// CollapsedSections defines which sections are visible in collapsed mode.
	// Valid values: fps, frame, cpu, gpu, ane, memory
	// Default: ["fps", "frame", "cpu", "gpu", "memory"]
	CollapsedSections []string `json:"collapsed_sections,omitempty"`

	// ExpandedOrder defines the section display order in expanded mode.
	// Valid values: fps, frame, cpu, gpu, ane, memory, swap, power,
	//              bandwidth, gpu_freq, temps, thermal, fans, network
	// Default: all sections in canonical order
	ExpandedOrder []string `json:"expanded_order,omitempty"`

	// Opacity overrides the overlay opacity (0.15-1.0)
	Opacity *float64 `json:"opacity,omitempty"`
}

type AppConfig struct {
	Language      string             `json:"language,omitempty"`
	DefaultLayout string             `json:"default_layout"`
	Theme         string             `json:"theme"`
	Background    string             `json:"background,omitempty"`
	Interval      int                `json:"interval,omitempty"`
	SortColumn    *int               `json:"sort_column,omitempty"`
	SortReverse   bool               `json:"sort_reverse"`
	CustomTheme   *CustomThemeConfig `json:"custom_theme,omitempty"`
	MenuBar       *MenuBarConfig     `json:"menubar,omitempty"`
	Overlay       *OverlayConfig     `json:"overlay,omitempty"`
}

// intOrDefault returns v if > 0, otherwise def.
func intOrDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// loadMenuBarConfig returns the menu bar config with defaults applied
func loadMenuBarConfig() MenuBarConfig {
	cfg := MenuBarConfig{
		StatusBarWidth:  24,
		StatusBarHeight: 18,
		SparklineWidth:  300,
		SparklineHeight: 80,
		FontSize:        10,
		PowerFontSize:   11,
	}
	if currentConfig.MenuBar == nil {
		return cfg
	}
	m := currentConfig.MenuBar
	cfg.StatusBarWidth = intOrDefault(m.StatusBarWidth, cfg.StatusBarWidth)
	cfg.StatusBarHeight = intOrDefault(m.StatusBarHeight, cfg.StatusBarHeight)
	cfg.SparklineWidth = intOrDefault(m.SparklineWidth, cfg.SparklineWidth)
	cfg.SparklineHeight = intOrDefault(m.SparklineHeight, cfg.SparklineHeight)
	cfg.FontSize = intOrDefault(m.FontSize, cfg.FontSize)
	cfg.PowerFontSize = intOrDefault(m.PowerFontSize, cfg.PowerFontSize)
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
	if m.CPUColor != "" {
		cfg.CPUColor = m.CPUColor
	}
	if m.GPUColor != "" {
		cfg.GPUColor = m.GPUColor
	}
	if m.ANEColor != "" {
		cfg.ANEColor = m.ANEColor
	}
	if m.MemColor != "" {
		cfg.MemColor = m.MemColor
	}
	if m.LabelColor != "" {
		cfg.LabelColor = m.LabelColor
	}
	if m.BarOrder != "" {
		cfg.BarOrder = m.BarOrder
	}
	return cfg
}

// Canonical overlay section names
var (
	overlayDefaultCollapsed = []string{"fps", "frame", "cpu", "gpu", "memory"}
	overlayDefaultExpanded  = []string{
		"fps", "frame", "cpu", "gpu", "ane", "memory", "swap",
		"power", "bandwidth", "gpu_freq", "temps", "thermal", "fans", "network",
	}
	overlayValidSections = map[string]bool{
		"fps": true, "frame": true, "cpu": true, "gpu": true, "ane": true,
		"memory": true, "swap": true, "power": true, "bandwidth": true,
		"gpu_freq": true, "temps": true, "thermal": true, "fans": true, "network": true,
	}
)

// filterValidSections returns only recognized section names from the input
func filterValidSections(sections []string) []string {
	var result []string
	for _, s := range sections {
		if overlayValidSections[strings.ToLower(strings.TrimSpace(s))] {
			result = append(result, strings.ToLower(strings.TrimSpace(s)))
		}
	}
	return result
}

// loadOverlayConfig returns the overlay config with defaults applied
func loadOverlayConfig() OverlayConfig {
	cfg := OverlayConfig{
		CollapsedSections: append([]string(nil), overlayDefaultCollapsed...),
		ExpandedOrder:     append([]string(nil), overlayDefaultExpanded...),
	}
	if currentConfig.Overlay == nil {
		return cfg
	}
	o := currentConfig.Overlay
	if len(o.CollapsedSections) > 0 {
		filtered := filterValidSections(o.CollapsedSections)
		if len(filtered) > 0 {
			cfg.CollapsedSections = filtered
		}
	}
	if len(o.ExpandedOrder) > 0 {
		filtered := filterValidSections(o.ExpandedOrder)
		if len(filtered) > 0 {
			cfg.ExpandedOrder = filtered
		}
	}
	if o.Opacity != nil {
		cfg.Opacity = o.Opacity
	}
	return cfg
}

var currentConfig AppConfig

const mactopAppDirName = "mactop"

func mactopLegacyDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), mactopAppDirName)
	}
	return filepath.Join(homeDir, ".mactop")
}

func xdgAppDir(envName string) (string, bool) {
	baseDir := os.Getenv(envName)
	if baseDir == "" || !filepath.IsAbs(baseDir) {
		return "", false
	}
	return filepath.Join(baseDir, mactopAppDirName), true
}

func mactopConfigDir() string {
	if dir, ok := xdgAppDir("XDG_CONFIG_HOME"); ok {
		return dir
	}
	return mactopLegacyDir()
}

func mactopStateDir() string {
	if dir, ok := xdgAppDir("XDG_STATE_HOME"); ok {
		return dir
	}
	return mactopLegacyDir()
}

func mactopConfigPath(name string) string {
	return filepath.Join(mactopConfigDir(), name)
}

func mactopStatePath(name string) string {
	return filepath.Join(mactopStateDir(), name)
}

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
	configPath := mactopConfigPath("config.json")

	file, err := os.ReadFile(configPath)
	if err != nil {
		currentConfig = AppConfig{DefaultLayout: "default"}
		return
	}

	err = json.Unmarshal(file, &currentConfig)
	if err != nil {
		currentConfig = AppConfig{DefaultLayout: "default"}
	}

	if currentConfig.Theme != "" {
		newTheme := migrateThemeName(currentConfig.Theme)
		if newTheme != currentConfig.Theme {
			currentConfig.Theme = newTheme
			saveConfig()
		}
	}
}

func saveConfig() {
	configDir := mactopConfigDir()
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

// loadThemeFile loads custom theme from the mactop config directory if it exists
// Theme file format:
//
//	{
//	  "foreground": "#9580FF",
//	  "background": "#22212C"
//	}
func loadThemeFile() *CustomThemeConfig {
	themePath := mactopConfigPath("theme.json")
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
