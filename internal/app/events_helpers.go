package app

import (
	ui "github.com/metaspartan/gotui/v5"
)

func toggleInfoLayout() {
	renderMutex.Lock()
	if currentConfig.DefaultLayout == LayoutInfo {
		if lastActiveLayout != "" {
			currentConfig.DefaultLayout = lastActiveLayout
		} else {
			currentConfig.DefaultLayout = LayoutDefault
		}
		for i, layout := range layoutOrder {
			if layout == currentConfig.DefaultLayout {
				currentLayoutNum = i
				break
			}
		}
	} else {
		lastActiveLayout = currentConfig.DefaultLayout
		currentConfig.DefaultLayout = LayoutInfo
		for i, layout := range layoutOrder {
			if layout == LayoutInfo {
				currentLayoutNum = i
				break
			}
		}
	}
	applyLayout(currentConfig.DefaultLayout)
	w, h := ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}

func handleThemeCycle() {
	renderMutex.Lock()
	w, h := ui.TerminalDimensions()
	updateLayout(w, h)
	cycleTheme()
	renderMutex.Unlock()
	renderMutex.Lock()
	updateProcessList()
	w, h = ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}

func handleLayoutCycle() {
	renderMutex.Lock()
	cycleLayout()
	renderMutex.Unlock()
	saveConfig()
	renderMutex.Lock()
	w, h := ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}

func handleBackgroundCycle() {
	renderMutex.Lock()
	cycleBackground()
	w, h := ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}

func toggleFreeze() {
	renderMutex.Lock()
	isFrozen = !isFrozen
	updateProcessList() // To redraw title with [FROZEN]
	w, h := ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}
