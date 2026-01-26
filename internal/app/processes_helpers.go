package app

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v5"
)

// isLightNamedTheme returns true for named themes that need dark text on header
// Only indigo is dark enough to need white text
func isLightNamedTheme(theme string) bool {
	return theme != "indigo"
}

func resolveProcessThemeColor() (string, string) {
	themeColor := processList.TextStyle.Fg
	var themeColorStr string

	// Check if custom ProcessList color is set
	if currentConfig.CustomTheme != nil && currentConfig.CustomTheme.ProcessList != "" {
		if IsHexColor(currentConfig.CustomTheme.ProcessList) {
			themeColorStr = currentConfig.CustomTheme.ProcessList
		}
	}

	if themeColorStr == "" {
		// Check if the theme is a hex color
		if IsHexColor(currentConfig.Theme) {
			themeColorStr = currentConfig.Theme
		} else if IsCatppuccinTheme(currentConfig.Theme) {
			themeColorStr = GetCatppuccinHex(currentConfig.Theme, "Primary")
		} else if IsLightMode && currentConfig.Theme == "white" {
			themeColorStr = "black"
		} else if currentConfig.Theme == "1977" {
			themeColorStr = "green"
		} else if color, ok := colorMap[currentConfig.Theme]; ok {
			hexStr := resolveThemeColorString(currentConfig.Theme)
			if hexStr != currentConfig.Theme {
				themeColorStr = hexStr
			} else {
				themeColorStr = getThemeColorName(color)
			}
		} else {
			themeColorStr = getThemeColorName(themeColor)
		}
	}

	// Determine selected header foreground color (text on colored background)
	selectedHeaderFg := "#ffffff" // Default to white for most themes
	if themeColorStr == "black" || themeColorStr == "#000000" || themeColorStr == "#020202" {
		selectedHeaderFg = "#ffffff"
	} else if IsLightMode {
		selectedHeaderFg = "#020202"
	} else if IsCatppuccinTheme(currentConfig.Theme) {
		selectedHeaderFg = GetCatppuccinHex(currentConfig.Theme, "Base")
	} else if IsHexColor(themeColorStr) && IsLightHexColor(themeColorStr) {
		// Bright hex colors need dark text for contrast
		selectedHeaderFg = "#020202"
	} else if isLightNamedTheme(currentConfig.Theme) {
		// Bright named colors need dark text for contrast
		selectedHeaderFg = "#020202"
	}
	// For dark hex colors, white text looks best
	return themeColorStr, selectedHeaderFg
}

func getProcessListTitle() (string, ui.Style) {
	// Resolve title color (use process list color if set)
	var titleColor ui.Color = ui.ColorClear
	if currentConfig.CustomTheme != nil && currentConfig.CustomTheme.ProcessList != "" {
		if color, err := ParseHexColor(currentConfig.CustomTheme.ProcessList); err == nil {
			titleColor = color
		}
	}
	if titleColor == ui.ColorClear {
		titleColor = GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode)
	}

	if killPending {
		return fmt.Sprintf(" Process List - KILL CONFIRMATION PENDING (PID %d) ", killPID), ui.NewStyle(ui.ColorRed, CurrentBgColor, ui.ModifierBold)
	} else if searchMode || searchText != "" {
		return fmt.Sprintf(" Search: %s_ (Esc to clear) ", searchText), ui.NewStyle(titleColor, CurrentBgColor, ui.ModifierBold)
	} else if isFrozen {
		return " Process List [FROZEN] (f to resume) ", ui.NewStyle(titleColor, CurrentBgColor, ui.ModifierBold)
	}
	return "Process List (↑/↓ scroll, / search, f freeze, F9 kill)", ui.NewStyle(titleColor, CurrentBgColor)
}

func attemptKillProcess() {
	var currentViewProcesses []ProcessMetrics

	// If search criteria exists, use that (even if nil/empty), otherwise use full list
	if searchText != "" {
		if filteredProcesses == nil {
			currentViewProcesses = []ProcessMetrics{}
		} else {
			currentViewProcesses = filteredProcesses
		}
	} else {
		currentViewProcesses = lastProcesses
	}

	if len(currentViewProcesses) > 0 && processList.SelectedRow < len(currentViewProcesses)+1 {
		if processList.SelectedRow > 0 {
			processIndex := processList.SelectedRow - 1
			if processIndex < len(currentViewProcesses) {
				pid := currentViewProcesses[processIndex].PID
				showKillModal(pid)
			}
		}
	}
}

func handleSearchToggle() {
	searchMode = true
	searchText = ""
	filteredProcesses = nil
	updateProcessList()
}

func handleSearchClear() {
	if searchText != "" {
		searchText = ""
		filteredProcesses = nil
		updateProcessList()
	}
}

func handleVerticalNavigation(e ui.Event) {
	switch e.ID {
	case "<Up>", "k", "<MouseWheelUp>":
		if processList.SelectedRow > 0 {
			processList.SelectedRow--
			updateProcessList()
		}
	case "<Down>", "j", "<MouseWheelDown>":
		if processList.SelectedRow < len(processList.Rows)-1 {
			processList.SelectedRow++
			updateProcessList()
		}
	case "g", "<Home>":
		if len(processList.Rows) > 1 {
			processList.SelectedRow = 1
			updateProcessList()
		} else {
			processList.SelectedRow = 0
		}
	case "G", "<End>":
		if len(processList.Rows) > 0 {
			processList.SelectedRow = len(processList.Rows) - 1
			updateProcessList()
		}
	}
}

func handleColumnNavigation(e ui.Event) {
	switch e.ID {
	case "<Left>":
		if selectedColumn > 0 {
			selectedColumn--
			currentConfig.SortColumn = &selectedColumn
			saveConfig()
			updateProcessList()
		}
	case "<Right>":
		if selectedColumn < len(columns)-1 {
			selectedColumn++
			currentConfig.SortColumn = &selectedColumn
			saveConfig()
			updateProcessList()
		}
	case "<Enter>", "<Space>":
		handleSortToggle()
	}
}

func handleSortToggle() {
	sortReverse = !sortReverse
	currentConfig.SortReverse = sortReverse
	saveConfig()
	updateProcessList()
}
