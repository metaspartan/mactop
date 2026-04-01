package app

import (
	"os"
	"time"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/mactop/v2/internal/i18n"
)

func startBackgroundUpdates(done chan struct{}) {
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				select {
				case cpuMetrics := <-cpuMetricsChan:
					renderMutex.Lock()
					lastCPUMetrics = cpuMetrics
					updateCPUUI(cpuMetrics)
					updateTotalPowerChart(cpuMetrics.PackageW)
					renderMutex.Unlock()
				default:
				}
				select {
				case gpuMetrics := <-gpuMetricsChan:
					renderMutex.Lock()
					lastGPUMetrics = gpuMetrics
					updateGPUUI(gpuMetrics)
					renderMutex.Unlock()
				default:
				}
				select {
				case tbNetStats := <-tbNetStatsChan:
					renderMutex.Lock()
					updateTBNetUI(tbNetStats)
					renderMutex.Unlock()
				default:
				}
				select {
				case netdiskMetrics := <-netdiskMetricsChan:
					renderMutex.Lock()
					lastNetDiskMetrics = netdiskMetrics
					updateNetDiskUI(netdiskMetrics)
					renderMutex.Unlock()
				default:
				}
				select {
				case processes := <-processMetricsChan:
					renderMutex.Lock()
					if !isFrozen && !killPending {
						lastProcesses = processes
						if searchText != "" {
							refreshFilteredProcesses()
						}
						updateProcessList()
					}
					renderMutex.Unlock()
				default:
				}
				// Update info UI once per cycle instead of multiple times
				renderMutex.Lock()
				updateInfoUI()
				renderMutex.Unlock()
				renderUI()

			}
		}
	}()
}

func updateLayout(w, h int) {
	mainBlock.SetRect(0, 0, w, h)
	if w < 93 {
		mainBlock.TitleBottom = ""
	} else {
		mainBlock.TitleBottom = i18n.T("TUI_InfoLayoutColorExit")
	}
	if w > 2 && h > 2 {
		grid.SetRect(1, 1, w-1, h-1)
	}
	if showHelp {
		grid.SetRect(0, 0, w, h)
	}
}

func drawScreen(w, h int) {
	ui.Clear()
	if w > 2 && h > 2 {
		if killPending {
			ui.Render(mainBlock, grid, confirmModal)
		} else {
			ui.Render(mainBlock, grid)
		}
	} else {
		ui.Render(mainBlock)
	}
}

func handleResizeEvent(e ui.Event) {
	payload := e.Payload.(ui.Resize)
	w, h := payload.Width, payload.Height
	UpdateCachedTerminalDimensions(w, h)
	renderMutex.Lock()
	updateLayout(w, h)
	drawScreen(w, h)
	renderMutex.Unlock()
}

func handleModeKeys(key string, done chan struct{}) {
	switch key {
	case "q", "<C-c>":
		close(done)
		ui.Close()
		os.Exit(0)
	case "r":
		w, h := ui.TerminalDimensions()
		UpdateCachedTerminalDimensions(w, h)
		renderMutex.Lock()
		updateLayout(w, h)
		drawScreen(w, h)
		renderMutex.Unlock()
	case "p":
		togglePartyMode()
	case "c":
		handleThemeCycle()
	case "l":
		handleLayoutCycle()
	case "h", "?":
		toggleHelpMenu()
	case "i":
		toggleInfoLayout()
	case "b":
		handleBackgroundCycle()
	case "f":
		toggleFreeze()
	case "F":
		toggleFanLayout()
	}
}

func handleIntervalKeys(key string) {
	delta := 0
	switch key {
	case "-", "_":
		delta = -100
	case "+", "=":
		delta = 100
	}

	if delta != 0 {
		updateInterval += delta
		if updateInterval < 100 {
			updateInterval = 100
		}
		if updateInterval > 5000 {
			updateInterval = 5000
		}
		ticker.Reset(time.Duration(updateInterval) * time.Millisecond)
		if partyMode && partyTicker != nil {
			partyTicker.Reset(time.Duration(updateInterval/2) * time.Millisecond)
		}

		renderMutex.Lock()
		updateHelpText()
		updateModelText()
		updateIntervalText()
		currentConfig.Interval = updateInterval
		renderMutex.Unlock()
		saveConfig()
	}
}

func handleKeyboardEvent(e ui.Event, done chan struct{}) {
	key := e.ID

	// Delegate to process list events (handles search/modal/navigation)
	renderMutex.Lock()

	if showHelp {
		switch key {
		case "j", "<Down>":
			helpScrollOffset++
			updateHelpText()
			drawScreen(GetCachedTerminalDimensions())
			renderMutex.Unlock()
			return
		case "k", "<Up>":
			if helpScrollOffset > 0 {
				helpScrollOffset--
			}
			updateHelpText()
			drawScreen(GetCachedTerminalDimensions())
			renderMutex.Unlock()
			return
		}
	} else {
		handleProcessListEvents(e)
	}

	if killPending || searchMode {
		w, h := GetCachedTerminalDimensions()
		drawScreen(w, h)
		renderMutex.Unlock()
		return
	}

	ui.Clear()
	w, h := GetCachedTerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()

	switch key {
	case "q", "<C-c>", "r", "p", "c", "l", "h", "?", "i", "b", "f", "F":
		handleModeKeys(key, done)
	case "-", "_", "+", "=":
		if !handleFanControlKeys(key) {
			handleIntervalKeys(key)
		}
	case "a", "A", "0", "9", "R":
		handleFanControlKeys(key)
	case "j", "<Down>":
		handleInfoFanScroll(1)
	case "k", "<Up>":
		handleInfoFanScroll(-1)
	}
}

func handleFanControlKeys(key string) bool {
	if !fanControl || currentConfig.DefaultLayout != LayoutFan {
		return false
	}
	switch key {
	case "+", "=", "-", "_":
		handleFanSpeedAdjust(key)
	case "a", "A":
		handleFanAutoToggle()
	case "0":
		handleFanSetMin()
	case "9":
		handleFanSetMax()
	case "R":
		handleFanResetAuto()
	default:
		return false
	}
	return true
}

func handleInfoFanScroll(direction int) {
	if currentConfig.DefaultLayout != LayoutInfo && currentConfig.DefaultLayout != LayoutFan {
		return
	}
	renderMutex.Lock()
	infoScrollOffset += direction
	if infoScrollOffset < 0 {
		infoScrollOffset = 0
	}
	updateInfoUI()
	w, h := ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}

func handleGenericMouseEvent(e ui.Event) {
	renderMutex.Lock()

	if showHelp {
		switch e.ID {
		case "<MouseWheelUp>":
			if helpScrollOffset > 0 {
				helpScrollOffset--
			}
			updateHelpText()
		case "<MouseWheelDown>":
			helpScrollOffset++
			updateHelpText()
		}
		drawScreen(GetCachedTerminalDimensions())
		renderMutex.Unlock()
		return
	}

	// Handle mouse wheel scrolling in Info or Fan layout
	if currentConfig.DefaultLayout == LayoutInfo || currentConfig.DefaultLayout == LayoutFan {
		switch e.ID {
		case "<MouseWheelUp>":
			if infoScrollOffset > 0 {
				infoScrollOffset--
			}
			updateInfoUI()
		case "<MouseWheelDown>":
			infoScrollOffset++
			updateInfoUI()
		}
	}

	handleProcessListEvents(e)
	w, h := GetCachedTerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()
}

func handleEvents(done chan struct{}, uiEvents <-chan ui.Event) {
	for e := range uiEvents {
		switch e.Type {
		case ui.ResizeEvent:
			handleResizeEvent(e)
		case ui.KeyboardEvent:
			handleKeyboardEvent(e, done)
		case ui.MouseEvent:
			handleGenericMouseEvent(e)
		}
	}
}
