package app

/*
#include <sys/sysctl.h>
#include <pwd.h>
#include <unistd.h>
#include <libproc.h>
#include <mach/mach_host.h>
#include <mach/processor_info.h>
#include <mach/mach_init.h>
#include <mach/mach_time.h>

extern kern_return_t vm_deallocate(vm_map_t target_task, vm_address_t address, vm_size_t size);
*/
import "C"
import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	ui "github.com/metaspartan/gotui/v5"
)

var uidCache = make(map[uint32]string)
var uidCacheMutex sync.RWMutex

func getUsername(uid uint32) string {
	uidCacheMutex.RLock()
	name, ok := uidCache[uid]
	uidCacheMutex.RUnlock()
	if ok {
		return name
	}

	uidCacheMutex.Lock()
	defer uidCacheMutex.Unlock()

	// Double check
	if name, ok := uidCache[uid]; ok {
		return name
	}

	// Use C.getpwuid
	pwd := C.getpwuid(C.uid_t(uid))
	if pwd != nil {
		name = C.GoString(pwd.pw_name)
	} else {
		name = fmt.Sprintf("%d", uid)
	}
	uidCache[uid] = name
	return name
}

type ProcessTimeState struct {
	Time      uint64
	Timestamp time.Time
}

var prevProcessTimes = make(map[int]ProcessTimeState)
var prevProcessTimesMutex sync.Mutex

var timebaseInfo C.mach_timebase_info_data_t
var timebaseOnce sync.Once

func getTimebase() {
	C.mach_timebase_info(&timebaseInfo)
}

func processOsProc(kp C.struct_kinfo_proc, now time.Time, prevProcessTimes map[int]ProcessTimeState, totalMem uint64, numer, denom uint64) (ProcessMetrics, int, ProcessTimeState, bool) {
	pid := int(kp.kp_proc.p_pid)
	if pid == 0 {
		return ProcessMetrics{}, 0, ProcessTimeState{}, false
	}

	comm := C.GoString(&kp.kp_proc.p_comm[0])
	var pathBuf [C.PROC_PIDPATHINFO_MAXSIZE]C.char
	if C.proc_pidpath(C.int(pid), unsafe.Pointer(&pathBuf), C.PROC_PIDPATHINFO_MAXSIZE) > 0 {
		fullPath := C.GoString(&pathBuf[0])
		comm = filepath.Base(fullPath)
	}

	rssBytes := int64(0)
	vszBytes := int64(0)
	totalTimeNs := uint64(0)

	var taskInfo C.struct_proc_taskinfo
	ret := C.proc_pidinfo(C.int(pid), C.PROC_PIDTASKINFO, 0, unsafe.Pointer(&taskInfo), C.int(C.sizeof_struct_proc_taskinfo))
	if ret == C.int(C.sizeof_struct_proc_taskinfo) {
		rssBytes = int64(taskInfo.pti_resident_size)
		vszBytes = int64(taskInfo.pti_virtual_size)
		rawTime := uint64(taskInfo.pti_total_user) + uint64(taskInfo.pti_total_system)
		totalTimeNs = (rawTime * numer) / denom
	}

	cpuPercent := 0.0
	if prevState, ok := prevProcessTimes[pid]; ok {
		timeDelta := totalTimeNs - prevState.Time
		wallDelta := now.Sub(prevState.Timestamp).Nanoseconds()
		if wallDelta > 0 && timeDelta > 0 {
			cpuPercent = (float64(timeDelta) / float64(wallDelta)) * 100.0
		}
	}

	newState := ProcessTimeState{
		Time:      totalTimeNs,
		Timestamp: now,
	}

	memPercent := 0.0
	if totalMem > 0 {
		memPercent = (float64(rssBytes) / float64(totalMem)) * 100.0
	}

	state := ""
	switch kp.kp_proc.p_stat {
	case C.SIDL:
		state = "I"
	case C.SRUN:
		state = "R"
	case C.SSLEEP:
		state = "S"
	case C.SSTOP:
		state = "T"
	case C.SZOMB:
		state = "Z"
	default:
		state = "?"
	}

	uid := uint32(kp.kp_eproc.e_ucred.cr_uid)
	user := getUsername(uid)

	totalSeconds := float64(totalTimeNs) / 1e9
	timeStr := formatTime(totalSeconds)

	pm := ProcessMetrics{
		PID:         pid,
		User:        user,
		CPU:         cpuPercent,
		Memory:      memPercent,
		VSZ:         vszBytes / 1024,
		RSS:         rssBytes / 1024,
		Command:     comm,
		State:       state,
		Started:     "",
		Time:        timeStr,
		LastUpdated: now,
	}
	return pm, pid, newState, true
}

func getProcessList(systemGpuPercent float64) ([]ProcessMetrics, error) {
	mib := []C.int{C.CTL_KERN, C.KERN_PROC, C.KERN_PROC_ALL}
	var size C.size_t

	if _, err := C.sysctl(&mib[0], 3, nil, &size, nil, 0); err != nil {
		return nil, fmt.Errorf("sysctl size check failed: %v", err)
	}

	buf := make([]byte, size)
	if _, err := C.sysctl(&mib[0], 3, unsafe.Pointer(&buf[0]), &size, nil, 0); err != nil {
		return nil, fmt.Errorf("sysctl fetch failed: %v", err)
	}

	count := int(size) / int(C.sizeof_struct_kinfo_proc)
	kprocs := (*[1 << 30]C.struct_kinfo_proc)(unsafe.Pointer(&buf[0]))[:count:count]

	var processes []ProcessMetrics
	now := time.Now()

	prevProcessTimesMutex.Lock()
	defer prevProcessTimesMutex.Unlock()

	nextProcessTimes := make(map[int]ProcessTimeState)

	mibMem := []C.int{6, 24}
	var memSize C.uint64_t
	memLen := C.size_t(unsafe.Sizeof(memSize))
	totalMem := uint64(0)
	if _, err := C.sysctl(&mibMem[0], 2, unsafe.Pointer(&memSize), &memLen, nil, 0); err == nil {
		totalMem = uint64(memSize)
	}

	timebaseOnce.Do(getTimebase)
	numer := uint64(timebaseInfo.numer)
	denom := uint64(timebaseInfo.denom)
	if denom == 0 {
		denom = 1
	}

	for _, kp := range kprocs {
		pm, pid, ns, ok := processOsProc(kp, now, prevProcessTimes, totalMem, numer, denom)
		if ok {
			processes = append(processes, pm)
			nextProcessTimes[pid] = ns
		}
	}

	prevProcessTimes = nextProcessTimes

	updateProcessGPUMetrics(processes, now, systemGpuPercent)

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPU > processes[j].CPU
	})

	if len(processes) > 500 {
		processes = processes[:500]
	}

	return processes, nil
}

// updateProcessGPUMetrics calculates per-process GPU usage and updates process metrics
func updateProcessGPUMetrics(processes []ProcessMetrics, now time.Time, systemGpuPercent float64) {
	gpuProcessStatsMutex.Lock()
	defer gpuProcessStatsMutex.Unlock()

	currentGPUStats := GetGPUProcessStats()
	gpuElapsed := now.Sub(lastGPUProcessStatsTime).Seconds()
	if gpuElapsed <= 0 {
		gpuElapsed = 1
	}

	gpuMsPerSec := make(map[int]float64)
	var totalRawGpuMs float64
	if currentGPUStats != nil && lastGPUProcessStats != nil {
		for pid, currentTime := range currentGPUStats {
			if prevTime, ok := lastGPUProcessStats[pid]; ok && currentTime >= prevTime {
				deltaNs := currentTime - prevTime
				gpuMs := float64(deltaNs) / gpuElapsed / 1_000_000
				gpuMsPerSec[pid] = gpuMs
				totalRawGpuMs += gpuMs
			}
		}
	}

	rawTotalPercent := totalRawGpuMs / 10.0

	scaleFactor := 1.0
	if rawTotalPercent > 0.01 && systemGpuPercent > 0.01 {
		scaleFactor = systemGpuPercent / rawTotalPercent
	}

	for i := range processes {
		if gpuMs, ok := gpuMsPerSec[processes[i].PID]; ok {
			processes[i].GPU = gpuMs * scaleFactor
		}
	}

	lastGPUProcessStats = currentGPUStats
	lastGPUProcessStatsTime = now
}

func GetCPUUsage() ([]CPUUsage, error) {
	var numCPUs C.natural_t
	var cpuLoad *C.processor_cpu_load_info_data_t
	var cpuMsgCount C.mach_msg_type_number_t
	host := C.mach_host_self()
	kernReturn := C.host_processor_info(
		host,
		C.PROCESSOR_CPU_LOAD_INFO,
		&numCPUs,
		(*C.processor_info_array_t)(unsafe.Pointer(&cpuLoad)),
		&cpuMsgCount,
	)
	if kernReturn != C.KERN_SUCCESS {
		return nil, fmt.Errorf("error getting CPU info: %d", kernReturn)
	}
	defer C.vm_deallocate(
		C.mach_task_self_,
		(C.vm_address_t)(uintptr(unsafe.Pointer(cpuLoad))),
		C.vm_size_t(cpuMsgCount)*C.sizeof_processor_cpu_load_info_data_t,
	)
	cpuLoadInfo := (*[1 << 30]C.processor_cpu_load_info_data_t)(unsafe.Pointer(cpuLoad))[:numCPUs:numCPUs]
	cpuUsage := make([]CPUUsage, numCPUs)
	for i := 0; i < int(numCPUs); i++ {
		cpuUsage[i] = CPUUsage{
			User:   float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_USER]),
			System: float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_SYSTEM]),
			Idle:   float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_IDLE]),
			Nice:   float64(cpuLoadInfo[i].cpu_ticks[C.CPU_STATE_NICE]),
		}
	}
	return cpuUsage, nil
}

func getThemeColorName(themeColor ui.Color) string {
	switch themeColor {
	case ui.ColorBlack:
		return "black"
	case ui.ColorRed:
		return "red"
	case ui.ColorGreen:
		return "green"
	case ui.ColorYellow:
		return "yellow"
	case ui.ColorBlue:
		return "blue"
	case ui.ColorMagenta:
		return "magenta"
	case ui.ColorSkyBlue:
		return "skyblue"
	case ui.ColorGold:
		return "gold"
	case ui.ColorSilver:
		return "silver"
	case ui.ColorWhite:
		return "white"
	case ui.ColorLime:
		return "lime"
	case ui.ColorOrange:
		return "orange"
	case ui.ColorViolet:
		return "violet"
	case ui.ColorPink:
		return "pink"
	default:
		return "white"
	}
}

func sortProcesses(processes []ProcessMetrics) {
	sort.Slice(processes, func(i, j int) bool {
		var less bool
		var equal bool

		switch columns[selectedColumn] {
		case "PID":
			less = processes[i].PID < processes[j].PID
			equal = processes[i].PID == processes[j].PID
		case "USER":
			u1, u2 := strings.ToLower(processes[i].User), strings.ToLower(processes[j].User)
			less = u1 < u2
			equal = u1 == u2
		case "VIRT":
			less = processes[i].VSZ > processes[j].VSZ // Descending default
			equal = processes[i].VSZ == processes[j].VSZ
		case "RES":
			less = processes[i].RSS > processes[j].RSS // Descending default
			equal = processes[i].RSS == processes[j].RSS
		case "CPU":
			less = processes[i].CPU > processes[j].CPU // Descending default
			equal = processes[i].CPU == processes[j].CPU
		case "GPU":
			less = processes[i].GPU > processes[j].GPU // Descending default
			equal = processes[i].GPU == processes[j].GPU
		case "MEM":
			less = processes[i].Memory > processes[j].Memory // Descending default
			equal = processes[i].Memory == processes[j].Memory
		case "TIME":
			iTime := parseTimeString(processes[i].Time)
			jTime := parseTimeString(processes[j].Time)
			less = iTime > jTime // Descending default
			equal = iTime == jTime
		case "CMD":
			c1, c2 := strings.ToLower(processes[i].Command), strings.ToLower(processes[j].Command)
			less = c1 < c2
			equal = c1 == c2
		default:
			less = processes[i].CPU > processes[j].CPU
			equal = processes[i].CPU == processes[j].CPU
		}

		if equal {
			// Secondary sort by PID (always ascending) to ensure stability
			return processes[i].PID < processes[j].PID
		}

		if sortReverse {
			return !less
		}
		return less
	})
}

func calculateMaxWidths(availableWidth int) map[string]int {
	maxWidths := map[string]int{
		"PID":  5,
		"USER": 8,
		"VIRT": 6,
		"RES":  6,
		"CPU":  6,
		"GPU":  6,
		"MEM":  5,
		"TIME": 8,
		"CMD":  15,
	}
	usedWidth := 0
	for col, width := range maxWidths {
		if col != "CMD" {
			usedWidth += width + 1
		}
	}

	cmdWidth := availableWidth - usedWidth
	if cmdWidth < 5 {
		cmdWidth = 5
	}
	maxWidths["CMD"] = cmdWidth
	return maxWidths
}

func buildHeader(maxWidths map[string]int, themeColorStr, selectedHeaderFg string) string {
	header := ""
	for i, col := range columns {
		width := maxWidths[col]

		// Determine arrow for selected column
		arrow := ""
		if i == selectedColumn {
			arrow = "↓"
			if sortReverse {
				arrow = "↑"
			}
		}

		// Build column text with arrow included in width
		colWithArrow := col + arrow

		format := ""
		switch col {
		case "PID":
			format = fmt.Sprintf("%%%ds", width) // Right-align
		case "USER":
			format = fmt.Sprintf("%%-%ds", width) // Left-align
		case "VIRT", "RES":
			format = fmt.Sprintf("%%%ds", width) // Right-align
		case "CPU", "GPU", "MEM":
			format = fmt.Sprintf("%%%ds", width) // Right-align
		case "TIME":
			format = fmt.Sprintf("%%%ds", width) // Right-align
		case "CMD":
			format = fmt.Sprintf("%%-%ds", width) // Left-align
		}

		colText := fmt.Sprintf(format, colWithArrow)
		header += fmt.Sprintf("[%s](fg:%s,bg:%s)", colText, selectedHeaderFg, themeColorStr)

		if i < len(columns)-1 {
			header += fmt.Sprintf("[%s](fg:%s,bg:%s)", "|", selectedHeaderFg, themeColorStr)
		}
	}
	return header
}

func buildProcessRows(processes []ProcessMetrics, maxWidths map[string]int) []string {
	items := make([]string, len(processes))
	for i, p := range processes {
		seconds := parseTimeString(p.Time)
		timeStr := formatTime(seconds)
		virtStr := formatMemorySize(p.VSZ)
		resStr := formatResMemorySize(p.RSS)
		username := truncateWithEllipsis(p.User, maxWidths["USER"])

		cmdName := p.Command // Already simplified by ps -c

		// Convert GPU from ms/s to percentage (ms/s / 10 = %)
		// 1000 ms/s = 100% GPU utilization
		gpuPercent := p.GPU / 10.0

		line := fmt.Sprintf("%*d %-*s %*s %*s %*.1f%% %*.1f%% %*.1f%% %*s %-s",
			maxWidths["PID"], p.PID,
			maxWidths["USER"], username,
			maxWidths["VIRT"], virtStr,
			maxWidths["RES"], resStr,
			maxWidths["CPU"]-1, p.CPU,
			maxWidths["GPU"]-1, gpuPercent,
			maxWidths["MEM"]-1, p.Memory,
			maxWidths["TIME"], timeStr,
			truncateWithEllipsis(cmdName, maxWidths["CMD"]),
		)

		if i == processList.SelectedRow-1 {
			items[i] = line
		} else if currentUser != "" && currentUser != "root" && p.User != currentUser {
			color := GetProcessTextColor(false)
			items[i] = fmt.Sprintf("[%s](fg:%s)", line, color)
		} else {
			color := GetProcessTextColor(true)
			items[i] = fmt.Sprintf("[%s](fg:%s)", line, color)
		}
	}
	return items
}

func updateProcessList() {
	processes := lastProcesses
	if searchText != "" {
		if filteredProcesses == nil {
			processes = []ProcessMetrics{}
		} else {
			processes = filteredProcesses
		}
	}

	if processes == nil {
		return
	}

	themeColorStr, selectedHeaderFg := resolveProcessThemeColor()

	termWidth, _ := GetCachedTerminalDimensions()
	availableWidth := termWidth - 2
	if availableWidth < 1 {
		availableWidth = 1
	}

	maxWidths := calculateMaxWidths(availableWidth)

	header := buildHeader(maxWidths, themeColorStr, selectedHeaderFg)
	sortProcesses(processes)
	rows := buildProcessRows(processes, maxWidths)

	items := make([]string, len(processes)+1)
	items[0] = header
	copy(items[1:], rows)

	processList.Title, processList.TitleStyle = getProcessListTitle()
	processList.Rows = items
}

func handleSearchInput(e ui.Event) {
	switch e.ID {
	case "<Escape>":
		searchMode = false
		searchText = ""
		filteredProcesses = nil
		updateProcessList()
	case "<Enter>":
		searchMode = false
		updateProcessList()
	case "<F9>":
		attemptKillProcess()
	case "<Backspace>":
		if len(searchText) > 0 {
			runes := []rune(searchText)
			searchText = string(runes[:len(runes)-1])
		}
		updateFilteredProcesses()
		updateProcessList()
	case "<Space>":
		searchText += " "
		updateFilteredProcesses()
		updateProcessList()
	default:
		// Only append printable characters (simple check)
		if len(e.ID) == 1 {
			searchText += e.ID
			updateFilteredProcesses()
			updateProcessList()
		}
	}
}

func refreshFilteredProcesses() {
	if searchText == "" {
		filteredProcesses = nil
		return
	}
	filteredProcesses = nil
	lowerText := strings.ToLower(searchText)
	for _, p := range lastProcesses {
		if strings.Contains(strings.ToLower(p.Command), lowerText) {
			filteredProcesses = append(filteredProcesses, p)
		}
	}
}

func updateFilteredProcesses() {
	refreshFilteredProcesses()
	if len(filteredProcesses) > 0 {
		processList.SelectedRow = 1
	} else {
		processList.SelectedRow = 0
	}
}

func updateKillModal() {
	termWidth, termHeight := GetCachedTerminalDimensions()
	modalWidth := 50
	modalHeight := 10 // Slightly taller for the buttons provided by widget

	x := (termWidth - modalWidth) / 2
	y := (termHeight - modalHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	confirmModal.SetRect(x, y, x+modalWidth, y+modalHeight)

	// Theme colors
	var primaryColor ui.Color
	var bg ui.Color = CurrentBgColor
	// Ensure opacity
	if GetCurrentBgName() == "clear" {
		bg = ui.ColorBlack
	}

	if IsCatppuccinTheme(currentConfig.Theme) {
		primaryColor = processList.TitleStyle.Fg
	} else if IsLightMode && currentConfig.Theme == "white" {
		primaryColor = ui.ColorBlack
	} else if color, ok := colorMap[currentConfig.Theme]; ok {
		primaryColor = color
	} else {
		primaryColor = ui.ColorGreen
	}

	confirmModal.BackgroundColor = bg
	confirmModal.TextStyle = ui.NewStyle(ui.ColorWhite, bg)
	if IsLightMode {
		confirmModal.TextStyle = ui.NewStyle(ui.ColorBlack, bg)
	}
	confirmModal.BorderStyle = ui.NewStyle(primaryColor, bg)
	confirmModal.TitleStyle = ui.NewStyle(primaryColor, bg, ui.ModifierBold)
	// Style buttons
	for _, btn := range confirmModal.Buttons {
		btn.TextStyle = ui.NewStyle(primaryColor, bg)
		btn.ActiveStyle = ui.NewStyle(bg, primaryColor)
		btn.BorderStyle = ui.NewStyle(primaryColor, bg)
	}
}

func showKillModal(pid int) {
	killPending = true
	killPID = pid
	confirmModal.ActiveButtonIndex = 1

	if len(confirmModal.Buttons) >= 2 {
		confirmModal.Buttons[0].OnClick = func() {
			executeKill()
		}
		confirmModal.Buttons[1].OnClick = func() {
			hideKillModal()
			updateProcessList()
		}
	}

	confirmModal.Title = fmt.Sprintf(" CONFIRM KILL PID %d ", pid)
	updateKillModal()
}

func hideKillModal() {
	killPending = false
}

func handleKillPending(e ui.Event) {
	switch e.ID {
	case "y", "Y": // Quick confirm
		executeKill()
	case "n", "N", "<Escape>": // Quick cancel
		hideKillModal()
		updateProcessList()
	case "<Left>", "h":
		confirmModal.ActiveButtonIndex = 0
		updateKillModal()
	case "<Right>", "l":
		confirmModal.ActiveButtonIndex = 1
		updateKillModal()
	case "<Enter>", "<Space>":
		if confirmModal.ActiveButtonIndex >= 0 && confirmModal.ActiveButtonIndex < len(confirmModal.Buttons) {
			if confirmModal.Buttons[confirmModal.ActiveButtonIndex].OnClick != nil {
				confirmModal.Buttons[confirmModal.ActiveButtonIndex].OnClick()
			}
		}
	}
}

func executeKill() {
	if err := syscall.Kill(killPID, syscall.SIGTERM); err == nil {
		stderrLogger.Printf("Sent SIGTERM to PID %d\n", killPID)

		if procs, err := getProcessList(lastGPUMetrics.ActivePercent); err == nil {
			lastProcesses = procs
			if searchMode || searchText != "" {
				updateFilteredProcesses()
			}
		}
	} else {
		stderrLogger.Printf("Failed to kill PID %d: %v\n", killPID, err)
	}
	hideKillModal()
	updateProcessList()
}

func handleNavigation(e ui.Event) {
	if searchMode {
		return
	}

	switch e.ID {
	case "/":
		handleSearchToggle()
	case "<Escape>":
		handleSearchClear()
	case "<Up>", "k", "<MouseWheelUp>", "<Down>", "j", "<MouseWheelDown>", "g", "<Home>", "G", "<End>":
		handleVerticalNavigation(e)
	case "<Left>", "<Right>":
		handleColumnNavigation(e)
	case "<Enter>", "<Space>":
		handleSortToggle()
	case "<F9>":
		attemptKillProcess()
	}
}

func handleProcessListEvents(e ui.Event) {
	// Don't handle process list navigation when in Info layout (allow Info scrolling)
	if currentConfig.DefaultLayout == LayoutInfo {
		return
	}
	if killPending {
		handleKillPending(e)
		return
	}
	if searchMode {
		handleSearchInput(e)
		return
	}
	handleNavigation(e)
}
