package app

import (
	"os"
	"time"

	ui "github.com/metaspartan/gotui/v4"
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
					lastCPUMetrics = cpuMetrics
					renderMutex.Lock()
					updateCPUUI(cpuMetrics)
					updateTotalPowerChart(cpuMetrics.PackageW)
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case gpuMetrics := <-gpuMetricsChan:
					lastGPUMetrics = gpuMetrics
					renderMutex.Lock()
					updateGPUUI(gpuMetrics)
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case netdiskMetrics := <-netdiskMetricsChan:
					lastNetDiskMetrics = netdiskMetrics
					renderMutex.Lock()
					updateNetDiskUI(netdiskMetrics)
					updateInfoUI()
					renderMutex.Unlock()
				default:
				}
				select {
				case processes := <-processMetricsChan:
					if processList.SelectedRow == 0 {
						lastProcesses = processes
						renderMutex.Lock()
						updateProcessList()
						renderMutex.Unlock()
					}
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
		mainBlock.TitleBottom = " Help: h | Info: i | Layout: l | Color: c | Exit: q "
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
		ui.Render(mainBlock, grid)
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
		updateInfoUI()
		renderMutex.Unlock()
		saveConfig()
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
		if currentConfig.DefaultLayout == LayoutInfo {
			currentConfig.DefaultLayout = LayoutDefault
			currentLayoutNum = 0
		} else {
			currentConfig.DefaultLayout = LayoutInfo
			for i, layout := range layoutOrder {
				if layout == LayoutInfo {
					currentLayoutNum = i
					break
				}
			}
		}
		applyLayout(currentConfig.DefaultLayout)
		renderMutex.Lock()
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
	fakeEvent := ui.Event{Type: ui.KeyboardEvent, ID: key}
	renderMutex.Lock()
	handleProcessListEvents(fakeEvent)
	ui.Clear()
	w, h := ui.TerminalDimensions()
	if w > 2 && h > 2 {
		ui.Render(mainBlock, grid)
	} else {
		ui.Render(mainBlock)
	}
	renderMutex.Unlock()

	switch key {
	case "q", "<C-c>", "r", "p", "c", "l", "h", "?", "i":
		handleModeKeys(key, done)
	case "-", "_", "+", "=":
		handleIntervalKeys(key)
	}
}

func handleGenericMouseEvent(e ui.Event) {
	renderMutex.Lock()
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
