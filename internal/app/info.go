package app

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/mactop/v2/internal/i18n"
)

func buildInfoLines(themeColor string) []string {
	uptimeSeconds, _ := GetNativeUptime()
	uptimeStr := formatTime(float64(uptimeSeconds))

	appleSiliconModel := cachedSystemInfo

	memMetrics := getMemoryMetrics()
	usedMem := float64(memMetrics.Used) / 1024 / 1024 / 1024
	totalMem := float64(memMetrics.Total) / 1024 / 1024 / 1024
	swapUsed := float64(memMetrics.SwapUsed) / 1024 / 1024 / 1024
	swapTotal := float64(memMetrics.SwapTotal) / 1024 / 1024 / 1024

	thermalStr, _ := getThermalStateString()
	if lastCPUMetrics.CPUTemp > 0 {
		thermalStr = fmt.Sprintf("%s (%s)", thermalStr, formatTemp(lastCPUMetrics.CPUTemp))
	}

	formatLine := func(label, value string) string {
		w := runewidth.StringWidth(label)
		paddedLabel := label
		if w < 15 {
			paddedLabel += strings.Repeat(" ", 15-w)
		}
		return fmt.Sprintf("[%s](fg:%s,mod:bold): [%s](fg:%s)", paddedLabel, themeColor, value, themeColor)
	}

	var sumWatts float64
	countWatts := 0
	for _, v := range powerValues {
		if v > 0 {
			actualWatts := (v / 8.0) * maxPowerSeen
			sumWatts += actualWatts
			countWatts++
		}
	}
	avgWatts := 0.0
	if countWatts > 0 {
		avgWatts = sumWatts / float64(countWatts)
	}

	// Get RDMA status
	rdmaStatus := CheckRDMAAvailable()
	rdmaLabel := i18n.T("Info_Disabled")
	if rdmaStatus.Available {
		rdmaLabel = i18n.T("Info_Enabled")
	}

	infoLines := []string{
		fmt.Sprintf("[%s@%s](fg:%s,mod:bold)", cachedCurrentUser, cachedHostname, themeColor),
		"-------------------------",
		formatLine(i18n.T("Info_OS"), fmt.Sprintf(i18n.T("Info_OSValue"), cachedOSVersion)),
		formatLine(i18n.T("Info_Host"), cachedModelName),
		formatLine(i18n.T("Info_Kernel"), cachedKernelVersion),
		formatLine(i18n.T("Info_Uptime"), uptimeStr),
		formatLine(i18n.T("Info_Shell"), cachedShell),
		formatLine(i18n.T("Info_CPU"), cachedModelName),
		formatLine(i18n.T("Info_GPU"), func() string {
			if appleSiliconModel.GPUCoreCount <= 0 {
				return i18n.T("Info_NotAvailable")
			}
			return fmt.Sprintf(i18n.T("TUI_GPUCores"), strconv.Itoa(appleSiliconModel.GPUCoreCount))
		}()),
		formatLine(i18n.T("Info_TFLOPs"), func() string {
			gpuCores := appleSiliconModel.GPUCoreCount
			maxFreq := GetMaxGPUFrequency()
			if maxFreq > 0 && gpuCores > 0 {
				fp32TFLOPs := float64(gpuCores) * float64(maxFreq) * 0.000256
				fp16TFLOPs := fp32TFLOPs * 2
				return fmt.Sprintf(i18n.T("Info_TFLOPsValue"), fp32TFLOPs, fp16TFLOPs)
			}
			return i18n.T("Info_NotAvailable")
		}()),
		formatLine(i18n.T("Info_Memory"), fmt.Sprintf("%.2f GB / %.2f GB", usedMem, totalMem)),
		formatLine(i18n.T("Info_Swap"), fmt.Sprintf("%.2f GB / %.2f GB", swapUsed, swapTotal)),
		"",
		formatLine(i18n.T("Info_CPUUsage"), fmt.Sprintf("%.2f%%", float64(cpuGauge.Percent))),
		formatLine(i18n.T("Info_GPUUsage"), fmt.Sprintf("%d%%", int(lastGPUMetrics.ActivePercent))),
		formatLine(i18n.T("Info_ANEUsage"), fmt.Sprintf("%d%%", int(lastCPUMetrics.ANEW/8.0*100))),
		formatLine(i18n.T("Info_Power"), fmt.Sprintf(i18n.T("Info_PowerValue"), lastCPUMetrics.PackageW, avgWatts)),
		formatLine(i18n.T("Info_Thermals"), thermalStr),
		formatLine(i18n.T("Info_Network"), fmt.Sprintf(i18n.T("Info_NetworkValue"), formatBytes(lastNetDiskMetrics.OutBytesPerSec, networkUnit), formatBytes(lastNetDiskMetrics.InBytesPerSec, networkUnit))),
		formatLine(i18n.T("Info_Disk"), fmt.Sprintf(i18n.T("Info_DiskValue"), formatBytes(lastNetDiskMetrics.ReadKBytesPerSec*1024, diskUnit), formatBytes(lastNetDiskMetrics.WriteKBytesPerSec*1024, diskUnit))),
		formatLine(i18n.T("Info_DRAMBW"), fmt.Sprintf(i18n.T("Info_DRAMBWValue"), lastCPUMetrics.DRAMReadBW, lastCPUMetrics.DRAMWriteBW, lastCPUMetrics.DRAMBWCombined)),
	}

	// Fan section
	if len(lastCPUMetrics.Fans) > 0 {
		infoLines = append(infoLines, "")
		for _, fan := range lastCPUMetrics.Fans {
			modeStr := i18n.T("Info_Auto")
			if fan.Mode == 1 {
				modeStr = i18n.T("Info_Manual")
			}
			infoLines = append(infoLines, formatLine(fan.Name, fmt.Sprintf(i18n.T("Info_FanValue"), fan.ActualRPM, modeStr, fan.MinRPM, fan.MaxRPM)))
		}
	}

	infoLines = append(infoLines, buildNetworkLinkLines(formatLine)...)
	infoLines = append(infoLines, buildVolumeLines(formatLine)...)

	infoLines = append(infoLines, "-------------------------")

	tbIn := formatBytes(lastTBInBytes, networkUnit)
	tbOut := formatBytes(lastTBOutBytes, networkUnit)
	infoLines = append(infoLines, formatLine(i18n.T("Info_TBNet"), fmt.Sprintf(i18n.T("Info_NetworkValue"), tbOut, tbIn)))

	infoLines = append(infoLines, formatLine(i18n.T("Info_RDMA"), rdmaLabel))

	infoLines = append(infoLines, buildThunderboltInfoLines(themeColor)...)

	return infoLines
}

func buildNetworkLinkLines(formatLine func(string, string) string) []string {
	var lines []string

	linkInfoMutex.RLock()
	ethInfo := cachedEthernetLinkInfo
	wifiInfo := cachedWiFiLinkInfo
	linkInfoMutex.RUnlock()

	for _, eth := range ethInfo {
		if eth.LinkUp {
			lines = append(lines, formatLine(i18n.T("Info_Ethernet"), fmt.Sprintf("%s (%s)", FormatLinkSpeed(eth.LinkSpeedMbps), eth.Name)))
		} else {
			lines = append(lines, formatLine(i18n.T("Info_Ethernet"), fmt.Sprintf(i18n.T("Info_Disconnected"), eth.Name)))
		}
	}
	if wifiInfo != nil {
		if wifiInfo.IsConnected {
			gen := wifiInfo.WiFiGeneration
			if gen == "" {
				gen = wifiInfo.PHYMode
			}
			lines = append(lines, formatLine(i18n.T("Info_WiFi"), fmt.Sprintf(i18n.T("Info_WiFiValue"), gen, wifiInfo.TxRateMbps, wifiInfo.InterfaceName)))
		} else {
			lines = append(lines, formatLine(i18n.T("Info_WiFi"), fmt.Sprintf(i18n.T("Info_Disconnected"), wifiInfo.InterfaceName)))
		}
	}
	return lines
}

func buildVolumeLines(formatLine func(string, string) string) []string {
	var lines []string
	volumes := getVolumes()
	if len(volumes) > 0 {
		lines = append(lines, "-------------------------")
		for _, v := range volumes {
			used := formatBytes(v.Used*1e9, diskUnit)
			total := formatBytes(v.Total*1e9, diskUnit)
			avail := formatBytes(v.Available*1e9, diskUnit)
			lines = append(lines, formatLine(v.Name, fmt.Sprintf(i18n.T("Info_VolumeValue"), used, total, avail)))
		}
	}
	return lines
}

func buildThunderboltInfoLines(themeColor string) []string {
	var lines []string
	tbInfoMutex.Lock()
	tbInfo := tbDeviceInfo
	tbInfoMutex.Unlock()

	if tbInfo != "" {
		for line := range strings.Lines(tbInfo) {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, fmt.Sprintf("[%s](fg:%s)", line, themeColor))
			}
		}
	}
	return lines
}

func getASCIIArt() []string {
	return []string{
		"                    'c.       ",
		"                 ,xNMM.       ",
		"               .OMMMMo        ",
		"               OMMM0,         ",
		"     .;loddo:' MACTOPbyCK;.   ",
		"   cKMMMMMMMMMMNWMMMMMMMMMM0: ",
		" .KMMMMMMMMMMMMMMMMMMMMMMMWd. ",
		" XMMMMMMMMMMMMMMMMMMMMMMMX.   ",
		";MMMMMMMMMMMMMMMMMMMMMMMM:    ",
		":MMMMMMMMMMMMMMMMMMMMMMMM:    ",
		".MMMMMMMMMMMMMMMMMMMMMMMMX.   ",
		" kMMMMMMMMMMMMMMMMMMMMMMMMWd. ",
		" .XMMMMMMMMMMMMMMMMMMMMMMMMMMk",
		"  .XMMMMMMMMMMMMMMMMMMMMMMMMK.",
		"    kMMMMMMMMMMMMMMMMMMMMMMd  ",
		"     ;KMMMMMMMWXXWMMMMMMMk.   ",
		"       .cooc,.    .,coo:.     ",
	}
}

func buildInfoText() string {
	themeColor := getThemeColor()
	infoLines := buildInfoLines(themeColor)
	asciiArt := getASCIIArt()

	layout := calculateInfoLayout(len(infoLines), len(asciiArt))

	return renderInfoText(infoLines, asciiArt, layout, themeColor)
}

func getThemeColor() string {
	themeColor := "green"
	if currentConfig.Theme != "" {
		if currentConfig.Theme == "1977" {
			themeColor = "green"
		} else if IsCatppuccinTheme(currentConfig.Theme) {
			themeColor = GetCatppuccinHex(currentConfig.Theme, "Primary")
		} else {
			// Use resolveThemeColorString for all colors (handles hex codes)
			themeColor = resolveThemeColorString(currentConfig.Theme)
		}
	}
	if IsLightMode && themeColor == "white" {
		themeColor = "black"
	}
	return themeColor
}

type infoLayout struct {
	startLine    int
	endLine      int
	paddingLeft  int
	paddingTop   int
	showAscii    bool
	totalLines   int
	contentWidth int
}

func calculateInfoLayout(infoLinesCount, asciiLinesCount int) infoLayout {
	termWidth, termHeight := ui.TerminalDimensions()
	showAscii := termWidth >= 82

	contentWidth := 80
	if !showAscii {
		contentWidth = 45
	}

	// Calculate available height for content (leave room for borders and scroll indicators)
	// We reserve 2 extra lines for top/bottom scroll indicators
	availableHeight := termHeight - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Determine total content height
	totalLines := infoLinesCount
	if showAscii && asciiLinesCount > totalLines {
		totalLines = asciiLinesCount
	}

	// Clamp scroll offset
	maxScroll := totalLines - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if infoScrollOffset > maxScroll {
		infoScrollOffset = maxScroll
	}
	if infoScrollOffset < 0 {
		infoScrollOffset = 0
	}

	// Calculate visible range
	startLine := infoScrollOffset
	endLine := min(startLine+availableHeight, totalLines)

	// Determine padding based on whether content needs scrolling
	paddingTop := 0
	if totalLines <= availableHeight {
		// Content fits, minimal padding
		paddingTop = 1 // Just a little spacing
	}

	paddingLeft := (termWidth - contentWidth) / 2
	if paddingLeft < 0 {
		paddingLeft = 0
	}

	return infoLayout{
		startLine:    startLine,
		endLine:      endLine,
		paddingLeft:  paddingLeft,
		paddingTop:   paddingTop,
		showAscii:    showAscii,
		totalLines:   totalLines,
		contentWidth: contentWidth,
	}
}

func renderInfoText(infoLines, asciiArt []string, layout infoLayout, themeColor string) string {
	paddingStr := strings.Repeat(" ", layout.paddingLeft)

	var combinedText strings.Builder
	combinedText.WriteString(strings.Repeat("\n", layout.paddingTop))

	rainbowColors := []string{"red", "magenta", "blue", "skyblue", "green", "yellow"}

	// Show scroll indicator if needed
	if infoScrollOffset > 0 {
		fmt.Fprintf(&combinedText, "%s[%s (k/↑)](fg:%s)\n", paddingStr, i18n.T("Info_ScrollUp"), themeColor)
	}

	// Helper for stripping tags to calculate visible length
	stripTags := func(s string) string {
		re := regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
		return re.ReplaceAllString(s, "$1")
	}

	// Calculate the maximum visible length dynamically
	maxVisibleLen := 48
	for _, l := range infoLines {
		vLen := runewidth.StringWidth(stripTags(l))
		if vLen > maxVisibleLen {
			maxVisibleLen = vLen
		}
	}
	textColWidth := maxVisibleLen + 2

	for i := layout.startLine; i < layout.endLine; i++ {
		asciiLine := ""
		if layout.showAscii {
			if i < len(asciiArt) {
				color := rainbowColors[i%len(rainbowColors)]
				asciiLine = fmt.Sprintf("[%s](fg:%s)", asciiArt[i], color)
			} else {
				asciiLine = fmt.Sprintf("%30s", " ")
			}
		}

		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}

		if layout.showAscii {
			visibleLen := runewidth.StringWidth(stripTags(infoLine))
			paddingSpaces := textColWidth - visibleLen
			if paddingSpaces < 2 {
				paddingSpaces = 2
			}

			fmt.Fprintf(&combinedText, "%s%s%s%s\n", paddingStr, infoLine, strings.Repeat(" ", paddingSpaces), asciiLine)
		} else {
			infoLine := ""
			if i < len(infoLines) {
				infoLine = infoLines[i]
			}
			fmt.Fprintf(&combinedText, "%s%s\n", paddingStr, infoLine)
		}
	}

	// Show scroll indicator if there's more below
	if layout.endLine < layout.totalLines {
		fmt.Fprintf(&combinedText, "%s[%s (j/↓)](fg:%s)\n", paddingStr, i18n.T("Info_ScrollDown"), themeColor)
	}

	return combinedText.String()
}

func fanRPMBar(fan FanInfo, themeColor string) []string {
	modeStr := "Auto"
	modeColor := "green"
	if fan.Mode == 1 {
		modeStr = "Manual"
		modeColor = "yellow"
	}

	pct := 0.0
	if fan.MaxRPM > 0 {
		pct = float64(fan.ActualRPM) / float64(fan.MaxRPM) * 100.0
	}

	barWidth := 20
	filled := min(int(pct/100.0*float64(barWidth)), barWidth)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	rpmColor := themeColor
	if pct > 80 {
		rpmColor = "red"
	} else if pct > 50 {
		rpmColor = "yellow"
	}

	return []string{
		fmt.Sprintf("[%s](fg:%s,mod:bold)  [%s](fg:%s) [%4d](fg:%s,mod:bold) / %d RPM  [%s](fg:%s)",
			fan.Name, themeColor, bar, rpmColor, fan.ActualRPM, rpmColor, fan.MaxRPM, modeStr, modeColor),
		fmt.Sprintf("    Target: %d RPM  |  Range: %d – %d RPM",
			fan.TargetRPM, fan.MinRPM, fan.MaxRPM),
	}
}

// sensorGroupMap maps SMC key second character to group category.
// Note: 's' is handled conditionally in sensorGroupName (SSD on M1-M4, S-Core on M5+).
var sensorGroupMap = map[byte]string{
	'p': "CPU P-Core", 'e': "CPU E-Core", 'f': "CPU P-Core",
	'g': "GPU", 'C': "CPU Core", 'c': "CPU Core",
	'm': "Memory", 'M': "Memory", 'S': "SSD",
	'H': "NAND", 'N': "NAND",
	'a': "Ambient", 'A': "Ambient", 'F': "Ambient",
	'B': "Board", 'b': "Board",
	'V': "VRM", 'P': "SoC Package", 'R': "GPU",
	'T': "Thunderbolt", 'I': "Thunderbolt",
	'w': "Wireless", 'W': "Wireless",
	'D': "Display", 'd': "Display", 'L': "Display",
}

// getNonSMCSensorGroup handles synthetic sensor keys that don't start with 'T'.
// Returns group name or empty string if not a known non-SMC key.
func getNonSMCSensorGroup(key string) string {
	switch key[0] {
	case 'H': // HID synthetic keys from IOHIDEventSystemClient
		switch key[1] {
		case 'e':
			return "CPU E-Core"
		case 'p':
			return "CPU P-Core"
		case 's':
			return "CPU S-Core"
		case 'g':
			return "GPU"
		}
	case 'N': // NVMe SMART sensors use 'Nv' prefix
		if key[1] == 'v' {
			return "NVMe"
		}
	}
	return ""
}

// sensorGroupName returns the group category for a sensor based on its SMC key.
func sensorGroupName(key string) string {
	if len(key) < 2 {
		return "Other"
	}
	if key[0] != 'T' {
		if group := getNonSMCSensorGroup(key); group != "" {
			return group
		}
		return "Other"
	}
	// Multi-char prefix matching for Apple Silicon specifics
	if len(key) >= 3 {
		if group := getAppleSiliconSensorGroup(key); group != "" {
			return group
		}
	}
	// Conditional: 's' = CPU S-Core on M5+ (has S-cores), SSD on M1-M4
	if key[1] == 's' {
		if cachedSystemInfo.SCoreCount > 0 {
			return "CPU S-Core"
		}
		return "SSD"
	}
	if group, ok := sensorGroupMap[key[1]]; ok {
		return group
	}
	return "Other"
}

// getAppleSiliconSensorGroup is a helper to reduce cyclomatic complexity
func getAppleSiliconSensorGroup(key string) string {
	if (key[1] == 'P' && (key[2] == 'D' || key[2] == 'M' || key[2] == 'S')) || key[1] == 'R' && key[2] == 'D' {
		if key[1] == 'R' {
			return "GPU"
		}
		return "SoC Package"
	}
	if key[1] == 'C' && (key[2] == 'M' || key[2] == 'D') {
		return "CPU Die"
	}
	return ""
}

// classifyCPUCoreSensors splits generic "CPU Core" sensors into E-Core, P-Core,
// and S-Core categories using known core counts.
func classifyCPUCoreSensors(sensors []TempSensor, sysInfo SystemInfo) []TempSensor {
	eCount := sysInfo.ECoreCount
	pCount := sysInfo.PCoreCount
	sCount := sysInfo.SCoreCount
	totalCores := eCount + pCount + sCount
	if totalCores == 0 {
		return sensors
	}

	// Collect indices of generic "CPU Core" sensors (by key-based group)
	var cpuIndices []int
	for i, s := range sensors {
		group := sensorGroupName(s.Key)
		if group == "CPU Core" {
			cpuIndices = append(cpuIndices, i)
		}
	}
	if len(cpuIndices) == 0 {
		return sensors
	}

	// Sort indices by key to get cluster order (E-cores first per die)
	sort.Slice(cpuIndices, func(a, b int) bool {
		return sensors[cpuIndices[a]].Key < sensors[cpuIndices[b]].Key
	})

	// Split by core ratio
	n := len(cpuIndices)
	eSensors := int(math.Round(float64(n) * float64(eCount) / float64(totalCores)))
	pSensors := int(math.Round(float64(n) * float64(pCount) / float64(totalCores)))

	result := make([]TempSensor, len(sensors))
	copy(result, sensors)

	for i, idx := range cpuIndices {
		if i < eSensors {
			result[idx].Name = "CPU E-Core"
		} else if i < eSensors+pSensors {
			result[idx].Name = "CPU P-Core"
		} else {
			result[idx].Name = "CPU S-Core"
		}
	}
	return result
}

func buildGroupedTempLines(sensors []TempSensor, themeColor string) []string {
	// Classify generic CPU Core sensors into E/P/S before grouping
	sensors = classifyCPUCoreSensors(sensors, cachedSystemInfo)

	// Group sensors by BASE category — always show grouped averages,
	// never individual per-core lines. Consistent across all chips.
	groups := make(map[string]*tempGroup)
	var groupOrder []string
	for _, s := range sensors {
		cat := sensorGroupName(s.Key)
		// For classified/HID CPU core sensors, merge into base category
		// (e.g., "CPU E-Core" not "CPU E-Core 04") for consistent grouping
		if strings.HasPrefix(s.Name, "CPU E-Core") {
			cat = "CPU E-Core"
		} else if strings.HasPrefix(s.Name, "CPU P-Core") {
			cat = "CPU P-Core"
		} else if strings.HasPrefix(s.Name, "CPU S-Core") {
			cat = "CPU S-Core"
		}
		g, exists := groups[cat]
		if !exists {
			g = &tempGroup{min: s.Value, max: s.Value}
			groups[cat] = g
			groupOrder = append(groupOrder, cat)
		}
		g.sum += s.Value
		g.count++
		if s.Value < g.min {
			g.min = s.Value
		}
		if s.Value > g.max {
			g.max = s.Value
		}
	}

	// Preferred display order — most important first
	preferred := []string{
		"CPU E-Core", "CPU P-Core", "CPU S-Core", "CPU Core", "CPU Die",
		"GPU", "SoC Package", "Memory", "SSD", "NAND", "NVMe",
		"Ambient", "VRM", "Board", "Thunderbolt",
		"Wireless", "Display",
	}

	var ordered []string
	seen := make(map[string]bool)
	for _, name := range preferred {
		if _, exists := groups[name]; exists {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	// Append any remaining groups not in preferred order
	for _, name := range groupOrder {
		if !seen[name] {
			ordered = append(ordered, name)
		}
	}

	var lines []string
	for _, cat := range ordered {
		g := groups[cat]
		lines = append(lines, formatTempGroupLine(cat, g, themeColor))
	}
	return lines
}

type tempGroup struct {
	sum   float64
	count int
	min   float64
	max   float64
}

func formatTempGroupLine(cat string, g *tempGroup, themeColor string) string {
	avg := g.sum / float64(g.count)
	tempColor := themeColor
	if avg > 90 {
		tempColor = "red"
	} else if avg > 70 {
		tempColor = "yellow"
	}
	// Pluralize core category names for display
	displayName := cat
	if g.count > 1 {
		switch cat {
		case "CPU E-Core":
			displayName = "CPU E-Cores"
		case "CPU P-Core":
			displayName = "CPU P-Cores"
		case "CPU S-Core":
			displayName = "CPU S-Cores"
		}
	}
	if g.count == 1 {
		return fmt.Sprintf("  [%-16s](fg:%s)  [%s](fg:%s)",
			displayName, themeColor, formatTemp(avg), tempColor)
	}
	return fmt.Sprintf("  [%-16s](fg:%s)  [%s](fg:%s)  [avg of %d, %s – %s](fg:%s)",
		displayName, themeColor, formatTemp(avg), tempColor,
		g.count, formatTemp(g.min), formatTemp(g.max), themeColor)
}

func renderScrollableLines(lines []string, themeColor string) string {
	_, termHeight := ui.TerminalDimensions()
	availableHeight := termHeight - 6
	if availableHeight < 5 {
		availableHeight = 5
	}
	totalLines := len(lines)

	maxScroll := totalLines - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if infoScrollOffset > maxScroll {
		infoScrollOffset = maxScroll
	}
	if infoScrollOffset < 0 {
		infoScrollOffset = 0
	}

	startLine := infoScrollOffset
	endLine := min(startLine+availableHeight, totalLines)

	paddingStr := "  "

	var result strings.Builder
	result.WriteString("\n")

	if infoScrollOffset > 0 {
		fmt.Fprintf(&result, "%s[%s (k/↑)](fg:%s)\n", paddingStr, i18n.T("Info_ScrollUp"), themeColor)
	}
	for i := startLine; i < endLine; i++ {
		fmt.Fprintf(&result, "%s%s\n", paddingStr, lines[i])
	}
	if endLine < totalLines {
		fmt.Fprintf(&result, "%s[%s (j/↓)](fg:%s)\n", paddingStr, i18n.T("Info_ScrollDown"), themeColor)
	}

	return result.String()
}

// buildFanStatusText renders the fan RPM panel content
func buildFanStatusText(themeColor string) string {
	formatLine := func(label, value string) string {
		paddedLabel := fmt.Sprintf("%-16s", label)
		return fmt.Sprintf("[%s](fg:%s,mod:bold): [%s](fg:%s)", paddedLabel, themeColor, value, themeColor)
	}

	var lines []string

	// Thermal state
	thermalStr, _ := getThermalStateString()
	if lastCPUMetrics.CPUTemp > 0 {
		thermalStr = fmt.Sprintf("%s (%s)", thermalStr, formatTemp(lastCPUMetrics.CPUTemp))
	}
	lines = append(lines, fmt.Sprintf("[%s](fg:%s,mod:bold): [%s](fg:%s)", i18n.T("Overlay_Thermal"), themeColor, thermalStr, themeColor))
	lines = append(lines, "")

	// CPU/GPU quick temps
	if lastCPUMetrics.CPUTemp > 0 {
		lines = append(lines, formatLine(i18n.T("Fan_CPUTemp"), formatTemp(lastCPUMetrics.CPUTemp)))
	}
	if lastCPUMetrics.GPUTemp > 0 {
		lines = append(lines, formatLine(i18n.T("Fan_GPUTemp"), formatTemp(lastCPUMetrics.GPUTemp)))
	}
	lines = append(lines, "")

	// Fan RPM bars
	if len(lastCPUMetrics.Fans) > 0 {
		for _, fan := range lastCPUMetrics.Fans {
			lines = append(lines, fanRPMBar(fan, themeColor)...)
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, fmt.Sprintf("[%s](fg:%s)", i18n.T("Fan_NoFansDetected"), themeColor))
	}

	return strings.Join(lines, "\n")
}

// buildFanTempText renders the grouped/averaged temperature sensor panel
func buildFanTempText(themeColor string) string {
	var lines []string

	if len(lastCPUMetrics.TempSensors) > 0 {
		lines = append(lines, buildGroupedTempLines(lastCPUMetrics.TempSensors, themeColor)...)
	} else {
		lines = append(lines, fmt.Sprintf("[%s](fg:%s)", i18n.T("Fan_NoTemperatureSensorsDetected"), themeColor))
	}

	return renderScrollableLines(lines, themeColor)
}

// buildFanControlText renders a compact single-line status bar
func buildFanControlText(themeColor string) string {
	if fanControl {
		return fmt.Sprintf(i18n.T("Fan_ControlActive"),
			themeColor, themeColor, themeColor, themeColor, themeColor)
	}
	return fmt.Sprintf(i18n.T("Fan_ReadOnly"),
		themeColor, themeColor, themeColor)
}
