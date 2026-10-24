package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	"golang.org/x/sys/unix"
)

type processDetailsView struct {
	*ui.Block
	lines     []string
	offset    int
	pageSize  int
	textStyle ui.Style
	pid       int
	focused   bool
	cancel    context.CancelFunc
}

var processDetails *processDetailsView

func parseProcessArguments(data []byte) (string, []string, error) {
	if len(data) < 4 {
		return "", nil, fmt.Errorf("process arguments are truncated")
	}
	count := int(int32(binary.NativeEndian.Uint32(data[:4])))
	data = data[4:]
	end := bytes.IndexByte(data, 0)
	if count < 1 || end < 0 {
		return "", nil, fmt.Errorf("invalid process arguments")
	}
	path := string(data[:end])
	data = bytes.TrimLeft(data[end+1:], "\x00")
	arguments := make([]string, 0)
	for range count {
		end = bytes.IndexByte(data, 0)
		if end < 0 {
			return "", nil, fmt.Errorf("process arguments are truncated")
		}
		arguments = append(arguments, string(data[:end]))
		data = data[end+1:]
	}
	return path, arguments, nil
}

func quoteProcessArgument(argument string) string {
	if argument != "" && !strings.ContainsFunc(argument, func(char rune) bool {
		return !unicode.IsLetter(char) && !unicode.IsDigit(char) && !strings.ContainsRune("_@%+=:,./-", char)
	}) {
		return argument
	}
	return "'" + strings.ReplaceAll(argument, "'", "'\"'\"'") + "'"
}

func processField(parent context.Context, command string, arguments ...string) string {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, command, arguments...).Output()
	if err != nil {
		return "Unavailable (process exited or access denied)"
	}
	return strings.TrimSpace(string(output))
}

func loadProcessDetails(pid int) []string {
	return loadProcessDetailsContext(context.Background(), pid)
}

func loadProcessDetailsContext(ctx context.Context, pid int) []string {
	pidText := strconv.Itoa(pid)
	lines := []string{"PID: " + pidText}
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	path, arguments, parseErr := parseProcessArguments(raw)
	if err != nil || parseErr != nil {
		lines = append(lines, "", "Command and executable: unavailable (process exited or access denied)")
	} else {
		quoted := make([]string, len(arguments))
		for index, argument := range arguments {
			quoted[index] = quoteProcessArgument(argument)
		}
		lines = append(lines, "", "Full command:", strings.Join(quoted, " "), "", "Executable:", path)
	}
	cwd := processField(ctx, "/usr/sbin/lsof", "-a", "-p", pidText, "-d", "cwd", "-Fn")
	directory := "Unavailable (process exited or access denied)"
	for _, line := range strings.Split(cwd, "\n") {
		if strings.HasPrefix(line, "n") {
			directory = strings.TrimPrefix(line, "n")
			break
		}
	}
	parent := processField(ctx, "/bin/ps", "-p", pidText, "-o", "ppid=")
	return append(lines, "", "Working directory:", directory, "", "Parent PID: "+parent)
}

func selectedDetailsPID() int {
	if processList == nil || processList.SelectedRow < 1 {
		return 0
	}
	index := processList.SelectedRow - 1
	if isPortsLayoutActive() {
		ports := currentViewPorts()
		if index < len(ports) {
			return ports[index].PID
		}
		return 0
	}
	processes := lastProcesses
	if searchText != "" {
		processes = filteredProcesses
	}
	if index < len(processes) {
		return processes[index].PID
	}
	return 0
}

func openProcessDetails() {
	pid := selectedDetailsPID()
	if pid <= 0 {
		return
	}
	processDetails = &processDetailsView{Block: ui.NewBlock()}
	processDetails.selectProcess(pid)
	processDetails.BorderType = ui.BorderDouble
}

func wrapProcessDetails(lines []string, width int) []string {
	width = max(width, 1)
	wrapped := []string{}
	for _, line := range lines {
		var current strings.Builder
		columns := 0
		for _, char := range line {
			if unicode.IsControl(char) {
				char = ' '
			}
			size := runewidth.RuneWidth(char)
			if size > width {
				char = '�'
				size = 1
			}
			if columns+size > width {
				wrapped = append(wrapped, current.String())
				current.Reset()
				columns = 0
			}
			current.WriteRune(char)
			columns += size
		}
		wrapped = append(wrapped, current.String())
	}
	return wrapped
}

func (view *processDetailsView) Draw(buffer *ui.Buffer) {
	view.applyTheme()
	view.Block.Draw(buffer)
	content := view.Inner
	if content.Dx() > 6 {
		content.Min.X += 2
		content.Max.X -= 2
	}
	if content.Dy() > 4 {
		content.Min.Y++
		content.Max.Y--
	}
	width, height := content.Dx(), content.Dy()
	if width <= 0 || height <= 0 {
		return
	}
	lines := wrapProcessDetails(view.lines, width)
	view.pageSize = height
	view.offset = max(0, min(view.offset, max(0, len(lines)-height)))
	for row, line := range lines[view.offset:min(view.offset+height, len(lines))] {
		column := content.Min.X
		style := view.textStyle
		if isProcessDetailsLabel(line) {
			style = ui.NewStyle(view.BorderStyle.Fg, view.BackgroundColor, ui.ModifierBold)
		}
		for _, char := range line {
			buffer.SetCell(ui.NewCell(char, style), image.Pt(column, content.Min.Y+row))
			column += runewidth.RuneWidth(char)
		}
	}
}

func renderProcessDetails(width, height int) bool {
	prepareProcessDetailsLayout(width, height)
	if !processDetailsVisible() || width < 3 || height < 3 {
		return false
	}
	if processDetailsDocked(width, height) {
		processDetails.selectProcess(selectedDetailsPID())
		_, rect := processDetailsRects(width, height)
		processDetails.SetRect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
		processDetails.TitleBottom = " Tab: Details · d: Close "
		if processDetails.focused {
			processDetails.TitleBottom = " ↑/↓: Scroll · Tab: List · d: Close "
		}
	} else {
		modalWidth, modalHeight := min(110, max(3, width-6)), min(30, max(3, height-4))
		left, top := (width-modalWidth)/2, (height-modalHeight)/2
		processDetails.SetRect(left, top, left+modalWidth, top+modalHeight)
		processDetails.TitleBottom = " ↑/↓ Scroll · PgUp/PgDn · Esc Close "
	}
	if killPending {
		ui.Render(mainBlock, grid, processDetails, confirmModal)
	} else {
		ui.Render(mainBlock, grid, processDetails)
	}
	return true
}

func handleProcessDetailsKey(key string) bool {
	if processDetails == nil {
		return false
	}
	switch key {
	case "<Escape>", "q":
		closeProcessDetails()
	case "<Down>", "j", "<MouseWheelDown>":
		processDetails.offset++
	case "<Up>", "k", "<MouseWheelUp>":
		processDetails.offset--
	case "<PageDown>":
		processDetails.offset += max(1, processDetails.pageSize)
	case "<PageUp>":
		processDetails.offset -= max(1, processDetails.pageSize)
	case "<Home>", "g":
		processDetails.offset = 0
	case "<End>", "G":
		processDetails.offset = int(^uint(0) >> 1)
	}
	return true
}

func handleProcessDetailsEvent(key string) bool {
	if key == "<C-c>" || showHelp || killPending || searchMode {
		return false
	}
	if currentConfig.DefaultLayout == LayoutInfo || currentConfig.DefaultLayout == LayoutFan {
		return false
	}
	if key == "d" {
		if processDetails == nil {
			openProcessDetails()
		} else {
			closeProcessDetails()
		}
		return true
	}
	if processDetails == nil {
		return false
	}
	if !processDetailsDocked(GetCachedTerminalDimensions()) {
		return handleProcessDetailsKey(key)
	}
	return handleDockedDetailsKey(key)
}

func handleProcessDetailsKeyboardEvent(key string) bool {
	renderMutex.Lock()
	defer renderMutex.Unlock()
	if !handleProcessDetailsEvent(key) {
		return false
	}
	drawScreen(GetCachedTerminalDimensions())
	return true
}

func (view *processDetailsView) applyTheme() {
	background := CurrentBgColor
	foreground := ui.ColorWhite
	if IsLightMode {
		foreground = ui.ColorBlack
	}
	if IsPaletteTheme(currentConfig.Theme) {
		foreground = GetThemeColor(GetPaletteHex(currentConfig.Theme, "Text"))
		if background == ui.ColorClear {
			background = GetThemeColor(GetPaletteHex(currentConfig.Theme, "Base"))
		}
	}
	if background == ui.ColorClear {
		background = ui.ColorBlack
		if IsLightMode {
			background = ui.ColorWhite
		}
	}
	if IsLightHexColor(fmt.Sprintf("#%06x", background.Hex())) {
		foreground = ui.ColorBlack
	}
	accent := GetThemeColorWithLightMode(currentConfig.Theme, IsLightMode)
	if processList != nil {
		accent = processList.BorderStyle.Fg
	}
	view.BackgroundColor = background
	view.BorderStyle = ui.NewStyle(accent, background)
	view.TitleStyle = ui.NewStyle(accent, background, ui.ModifierBold)
	view.TitleBottomStyle = ui.NewStyle(accent, background)
	view.textStyle = ui.NewStyle(foreground, background)
}

func isProcessDetailsLabel(line string) bool {
	switch line {
	case "Full command:", "Executable:", "Working directory:":
		return true
	}
	return strings.HasPrefix(line, "PID: ") || strings.HasPrefix(line, "Parent PID: ")
}
