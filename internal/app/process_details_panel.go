package app

import (
	"context"
	"fmt"
	"image"
	"time"

	ui "github.com/metaspartan/gotui/v5"
)

const (
	processDetailsMinWidth  = 150
	processDetailsMinHeight = 24
)

func processDetailsDocked(width, height int) bool {
	return width >= processDetailsMinWidth && height >= processDetailsMinHeight
}

func processDetailsVisible() bool {
	return processDetails != nil && !showHelp && currentConfig.DefaultLayout != LayoutInfo && currentConfig.DefaultLayout != LayoutFan
}

func processDetailsRects(width, height int) (image.Rectangle, image.Rectangle) {
	split := width - min(60, width/3) - 1
	return image.Rect(1, 1, split, height-1), image.Rect(split+1, 1, width-1, height-1)
}

func processDetailsTableWidth(width int) int {
	_, height := GetCachedTerminalDimensions()
	if processDetailsVisible() && processDetailsDocked(width, height) {
		table, _ := processDetailsRects(width, height)
		return table.Dx() - 2
	}
	return width - 2
}

var processDetailsGridWidth int

func prepareProcessDetailsLayout(width, height int) {
	if grid == nil || width < 3 || height < 3 || showHelp {
		return
	}
	rect := image.Rect(1, 1, width-1, height-1)
	if processDetailsVisible() && processDetailsDocked(width, height) {
		rect, _ = processDetailsRects(width, height)
	}
	if grid.Rectangle != rect || processDetailsGridWidth != rect.Dx() {
		processDetailsGridWidth = rect.Dx()
		grid.SetRect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
		updateProcessList()
	}
}

func closeProcessDetails() {
	if processDetails != nil && processDetails.cancel != nil {
		processDetails.cancel()
	}
	processDetails = nil
}

func (view *processDetailsView) selectProcess(pid int) {
	if view.pid == pid {
		return
	}
	if view.cancel != nil {
		view.cancel()
	}
	view.pid, view.offset = pid, 0
	view.Title = fmt.Sprintf(" Process %d — Details ", pid)
	if pid <= 0 {
		view.Title = " Process Details "
		view.lines = []string{"Select a process with ↑/↓."}
		return
	}
	view.lines = []string{"Loading process details…"}
	ctx, cancel := context.WithCancel(context.Background())
	view.cancel = cancel
	go view.fetchProcess(ctx, pid)
}

func (view *processDetailsView) fetchProcess(ctx context.Context, pid int) {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	lines := loadProcessDetailsContext(ctx, pid)
	renderMutex.Lock()
	defer renderMutex.Unlock()
	if ctx.Err() == nil && processDetails == view && view.pid == pid {
		view.lines = lines
	}
}

func handleDockedDetailsKey(key string) bool {
	switch key {
	case "<Tab>":
		processDetails.focused = !processDetails.focused
		return true
	case "<Escape>":
		closeProcessDetails()
		return true
	}
	if !processDetails.focused {
		return false
	}
	switch key {
	case "<Down>", "j", "<Up>", "k", "<PageDown>", "<PageUp>", "<Home>", "g", "<End>", "G":
		return handleProcessDetailsKey(key)
	}
	return false
}

func handleProcessDetailsMouse(event ui.Event) bool {
	if !processDetailsVisible() || killPending {
		return false
	}
	if !processDetailsDocked(GetCachedTerminalDimensions()) {
		return handleProcessDetailsKey(event.ID)
	}
	mouse, ok := event.Payload.(ui.Mouse)
	if !ok {
		return false
	}
	inside := image.Pt(mouse.X, mouse.Y).In(processDetails.Rectangle)
	if event.ID == "<MouseLeft>" {
		processDetails.focused = inside
		return inside
	}
	if inside && (event.ID == "<MouseWheelUp>" || event.ID == "<MouseWheelDown>") {
		return handleProcessDetailsKey(event.ID)
	}
	return false
}

func processDetailsSelectionPID() int {
	if processDetailsVisible() && processDetailsDocked(GetCachedTerminalDimensions()) {
		return selectedDetailsPID()
	}
	return 0
}

func restoreProcessDetailsSelection(pid int) {
	if pid <= 0 {
		return
	}
	if isPortsLayoutActive() {
		for index, port := range currentViewPorts() {
			if port.PID == pid {
				processList.SelectedRow = index + 1
				return
			}
		}
	} else {
		processes := lastProcesses
		if searchText != "" {
			processes = filteredProcesses
		}
		for index, process := range processes {
			if process.PID == pid {
				processList.SelectedRow = index + 1
				return
			}
		}
	}
	processList.SelectedRow = min(processList.SelectedRow, max(0, len(processList.Rows)-1))
}
