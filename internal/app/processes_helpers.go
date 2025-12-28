package app

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v5"
)

func resolveProcessThemeColor() (string, string) {
	themeColor := processList.TextStyle.Fg
	var themeColorStr string
	if IsCatppuccinTheme(currentConfig.Theme) {
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

	selectedHeaderFg := "black"
	if themeColorStr == "black" {
		selectedHeaderFg = "white"
	} else if IsCatppuccinTheme(currentConfig.Theme) {
		selectedHeaderFg = GetCatppuccinHex(currentConfig.Theme, "Base")
	}
	return themeColorStr, selectedHeaderFg
}

func getProcessListTitle() (string, ui.Style) {
	if killPending {
		return fmt.Sprintf(" Process List - KILL CONFIRMATION PENDING (PID %d) ", killPID), ui.NewStyle(ui.ColorRed, CurrentBgColor, ui.ModifierBold)
	} else if searchMode || searchText != "" {
		return fmt.Sprintf(" Search: %s_ (Esc to clear) ", searchText), ui.NewStyle(GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode), CurrentBgColor, ui.ModifierBold)
	} else if isFrozen {
		return " Process List [FROZEN] (f to resume) ", ui.NewStyle(GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode), CurrentBgColor, ui.ModifierBold)
	}
	return "Process List (↑/↓ scroll, / search, f freeze, F9 kill)", ui.NewStyle(GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode), CurrentBgColor)
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
