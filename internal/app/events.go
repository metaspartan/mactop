package app

import (
	"os"
	"time"

	ui "github.com/metaspartan/gotui/v5"
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
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case gpuMetrics := <-gpuMetricsChan:
					renderMutex.Lock()
					lastGPUMetrics = gpuMetrics
					updateGPUUI(gpuMetrics)
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case tbNetStats := <-tbNetStatsChan:
					renderMutex.Lock()
					updateTBNetUI(tbNetStats)
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case netdiskMetrics := <-netdiskMetricsChan:
					renderMutex.Lock()
					lastNetDiskMetrics = netdiskMetrics
					updateNetDiskUI(netdiskMetrics)
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case processes := <-processMetricsChan:
					renderMutex.Lock()
					if processList.SelectedRow <= 1 {
						lastProcesses = processes
						if searchText != "" {
							refreshFilteredProcesses()
						}
						updateProcessList()
					}
					renderMutex.Unlock()
				default:
				}
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
		mainBlock.TitleBottom = " Info: i | Layout: l | Color: c | BG: b | Exit: q "
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
		renderMutex.Lock()
		updateLayout(w, h)
		drawScreen(w, h)
		renderMutex.Unlock()
	case "p":
		togglePartyMode()
	case "c":
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
	case "l":
		renderMutex.Lock()
		cycleLayout()
		renderMutex.Unlock()
		saveConfig()
		renderMutex.Lock()
		w, h := ui.TerminalDimensions()
		drawScreen(w, h)
		renderMutex.Unlock()
	case "h", "?":
		toggleHelpMenu()
	case "i":

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
	case "b":
		renderMutex.Lock()
		cycleBackground()
		w, h := ui.TerminalDimensions()
		drawScreen(w, h)
		renderMutex.Unlock()
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
		renderMutex.Unlock()
	}
}

func handleKeyboardEvent(e ui.Event, done chan struct{}) {
	key := e.ID

	// Delegate to process list events (handles search/modal/navigation)
	renderMutex.Lock()
	handleProcessListEvents(e)

	if killPending || searchMode {
		w, h := ui.TerminalDimensions()
		drawScreen(w, h)
		renderMutex.Unlock()
		return
	}

	ui.Clear()
	w, h := ui.TerminalDimensions()
	drawScreen(w, h)
	renderMutex.Unlock()

	switch key {
	case "q", "<C-c>", "r", "p", "c", "l", "h", "?", "i", "b":
		handleModeKeys(key, done)
	case "-", "_", "+", "=":
		handleIntervalKeys(key)
	case "j", "<Down>":
		// Scroll down in Info layout
		if currentConfig.DefaultLayout == LayoutInfo {
			renderMutex.Lock()
			infoScrollOffset++
			updateInfoUI()
			w, h := ui.TerminalDimensions()
			drawScreen(w, h)
			renderMutex.Unlock()
		}
	case "k", "<Up>":
		// Scroll up in Info layout
		if currentConfig.DefaultLayout == LayoutInfo {
			renderMutex.Lock()
			if infoScrollOffset > 0 {
				infoScrollOffset--
			}
			updateInfoUI()
			w, h := ui.TerminalDimensions()
			drawScreen(w, h)
			renderMutex.Unlock()
		}
	}
}

func handleGenericMouseEvent(e ui.Event) {
	renderMutex.Lock()

	// Handle mouse wheel scrolling in Info layout
	if currentConfig.DefaultLayout == LayoutInfo {
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
	w, h := ui.TerminalDimensions()
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
