package app

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
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
		paddedLabel := fmt.Sprintf("%-13s", label)
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
	rdmaLabel := "Disabled"
	if rdmaStatus.Available {
		rdmaLabel = "Enabled"
	}

	infoLines := []string{
		fmt.Sprintf("[%s@%s](fg:%s,mod:bold)", cachedCurrentUser, cachedHostname, themeColor),
		"-------------------------",
		formatLine("OS", fmt.Sprintf("macOS %s", cachedOSVersion)),
		formatLine("Host", cachedModelName),
		formatLine("Kernel", cachedKernelVersion),
		formatLine("Uptime", uptimeStr),
		formatLine("Shell", cachedShell),
		formatLine("CPU", cachedModelName),
		formatLine("GPU", fmt.Sprintf("%d Core GPU", appleSiliconModel.GPUCoreCount)),
		formatLine("TFLOPs", func() string {
			gpuCores := appleSiliconModel.GPUCoreCount
			maxFreq := GetMaxGPUFrequency()
			if maxFreq > 0 && gpuCores > 0 {
				fp32TFLOPs := float64(gpuCores) * float64(maxFreq) * 0.000256
				fp16TFLOPs := fp32TFLOPs * 2
				return fmt.Sprintf("%.1f FP32 / %.1f FP16", fp32TFLOPs, fp16TFLOPs)
			}
			return "N/A"
		}()),
		formatLine("Memory", fmt.Sprintf("%.2f GB / %.2f GB", usedMem, totalMem)),
		formatLine("Swap", fmt.Sprintf("%.2f GB / %.2f GB", swapUsed, swapTotal)),
		"",
		formatLine("CPU Usage", fmt.Sprintf("%.2f%%", float64(cpuGauge.Percent))),
		formatLine("GPU Usage", fmt.Sprintf("%d%%", int(lastGPUMetrics.ActivePercent))),
		formatLine("ANE Usage", fmt.Sprintf("%d%%", int(lastCPUMetrics.ANEW/8.0*100))),
		formatLine("Power", fmt.Sprintf("%.2f W (Avg %.0f W)", lastCPUMetrics.PackageW, avgWatts)),
		formatLine("Thermals", thermalStr),
		formatLine("Network", fmt.Sprintf("↑ %s/s ↓ %s/s", formatBytes(lastNetDiskMetrics.OutBytesPerSec, networkUnit), formatBytes(lastNetDiskMetrics.InBytesPerSec, networkUnit))),
		formatLine("Disk", fmt.Sprintf("R %s/s W %s/s", formatBytes(lastNetDiskMetrics.ReadKBytesPerSec*1024, diskUnit), formatBytes(lastNetDiskMetrics.WriteKBytesPerSec*1024, diskUnit))),
		formatLine("DRAM BW", fmt.Sprintf("R %.1f / W %.1f / %.1f GB/s", lastCPUMetrics.DRAMReadBW, lastCPUMetrics.DRAMWriteBW, lastCPUMetrics.DRAMBWCombined)),
	}

	// Fan section
	if len(lastCPUMetrics.Fans) > 0 {
		infoLines = append(infoLines, "")
		for _, fan := range lastCPUMetrics.Fans {
			modeStr := "Auto"
			if fan.Mode == 1 {
				modeStr = "Manual"
			}
			infoLines = append(infoLines, formatLine(fan.Name, fmt.Sprintf("%d RPM (%s, %d-%d)", fan.ActualRPM, modeStr, fan.MinRPM, fan.MaxRPM)))
		}
	}

	infoLines = append(infoLines, buildNetworkLinkLines(formatLine)...)
	infoLines = append(infoLines, buildVolumeLines(formatLine)...)

	infoLines = append(infoLines, "-------------------------")

	tbIn := formatBytes(lastTBInBytes, networkUnit)
	tbOut := formatBytes(lastTBOutBytes, networkUnit)
	infoLines = append(infoLines, formatLine("TB Net", fmt.Sprintf("↑ %s/s ↓ %s/s", tbOut, tbIn)))

	infoLines = append(infoLines, formatLine("RDMA", rdmaLabel))

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
			lines = append(lines, formatLine("Ethernet", fmt.Sprintf("%s (%s)", FormatLinkSpeed(eth.LinkSpeedMbps), eth.Name)))
		} else {
			lines = append(lines, formatLine("Ethernet", fmt.Sprintf("Disconnected (%s)", eth.Name)))
		}
	}
	if wifiInfo != nil {
		if wifiInfo.IsConnected {
			gen := wifiInfo.WiFiGeneration
			if gen == "" {
				gen = wifiInfo.PHYMode
			}
			lines = append(lines, formatLine("Wi-Fi", fmt.Sprintf("%s @ %dMbps (%s)", gen, wifiInfo.TxRateMbps, wifiInfo.InterfaceName)))
		} else {
			lines = append(lines, formatLine("Wi-Fi", fmt.Sprintf("Disconnected (%s)", wifiInfo.InterfaceName)))
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
			lines = append(lines, formatLine(v.Name, fmt.Sprintf("%s / %s (%s free)", used, total, avail)))
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
		fmt.Fprintf(&combinedText, "%s[↑ Scroll up (k/↑)](fg:%s)\n", paddingStr, themeColor)
	}

	// Helper for stripping tags to calculate visible length
	stripTags := func(s string) string {
		re := regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
		return re.ReplaceAllString(s, "$1")
	}

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
			visibleLen := len(stripTags(infoLine))

			textColWidth := 48
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
		fmt.Fprintf(&combinedText, "%s[↓ Scroll down (j/↓)](fg:%s)\n", paddingStr, themeColor)
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
	filled := int(pct / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
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
var sensorGroupMap = map[byte]string{
	'p': "CPU P-Core", 'e': "CPU E-Core", 'f': "CPU P-Core",
	'g': "GPU", 'C': "CPU Core", 'c': "CPU Core",
	'm': "Memory", 'M': "Memory", 's': "SSD", 'S': "SSD",
	'H': "NAND", 'N': "NAND",
	'a': "Ambient", 'A': "Ambient", 'F': "Ambient",
	'B': "Board", 'b': "Board",
	'V': "VRM", 'P': "SoC Package", 'R': "GPU",
	'T': "Thunderbolt", 'I': "Thunderbolt",
	'w': "Wireless", 'W': "Wireless",
	'D': "Display", 'd': "Display", 'L': "Display",
}

// sensorGroupName returns the group category for a sensor based on its SMC key.
func sensorGroupName(key string) string {
	if len(key) < 2 || key[0] != 'T' {
		return "Other"
	}
	// Multi-char prefix matching for Apple Silicon specifics
	if len(key) >= 3 {
		if (key[1] == 'P' && (key[2] == 'D' || key[2] == 'M' || key[2] == 'S')) || key[1] == 'R' && key[2] == 'D' {
			if key[1] == 'R' {
				return "GPU"
			}
			return "SoC Package"
		}
		if key[1] == 'C' && (key[2] == 'M' || key[2] == 'D') {
			return "CPU Die"
		}
	}
	if group, ok := sensorGroupMap[key[1]]; ok {
		return group
	}
	return "Other"
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

	// Group sensors by category (using key-based group name for proper merging)
	groups := make(map[string]*tempGroup)
	var groupOrder []string
	for _, s := range sensors {
		cat := sensorGroupName(s.Key)
		// For classified CPU sensors, use the reclassified Name
		if strings.HasPrefix(s.Name, "CPU E-Core") || strings.HasPrefix(s.Name, "CPU P-Core") || strings.HasPrefix(s.Name, "CPU S-Core") {
			cat = s.Name
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
		"GPU", "SoC Package", "Memory", "SSD", "NAND",
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
	if g.count == 1 {
		return fmt.Sprintf("  [%-16s](fg:%s)  [%s](fg:%s)",
			cat, themeColor, formatTemp(avg), tempColor)
	}
	return fmt.Sprintf("  [%-16s](fg:%s)  [%s](fg:%s)  [avg of %d, %s – %s](fg:%s)",
		cat, themeColor, formatTemp(avg), tempColor,
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
		fmt.Fprintf(&result, "%s[↑ Scroll up (k/↑)](fg:%s)\n", paddingStr, themeColor)
	}
	for i := startLine; i < endLine; i++ {
		fmt.Fprintf(&result, "%s%s\n", paddingStr, lines[i])
	}
	if endLine < totalLines {
		fmt.Fprintf(&result, "%s[↓ Scroll down (j/↓)](fg:%s)\n", paddingStr, themeColor)
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
	lines = append(lines, fmt.Sprintf("[Thermal State](fg:%s,mod:bold): [%s](fg:%s)", themeColor, thermalStr, themeColor))
	lines = append(lines, "")

	// CPU/GPU quick temps
	if lastCPUMetrics.CPUTemp > 0 {
		lines = append(lines, formatLine("CPU Temp", formatTemp(lastCPUMetrics.CPUTemp)))
	}
	if lastCPUMetrics.GPUTemp > 0 {
		lines = append(lines, formatLine("GPU Temp", formatTemp(lastCPUMetrics.GPUTemp)))
	}
	lines = append(lines, "")

	// Fan RPM bars
	if len(lastCPUMetrics.Fans) > 0 {
		for _, fan := range lastCPUMetrics.Fans {
			lines = append(lines, fanRPMBar(fan, themeColor)...)
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, fmt.Sprintf("[No fans detected](fg:%s)", themeColor))
	}

	return strings.Join(lines, "\n")
}

// buildFanTempText renders the grouped/averaged temperature sensor panel
func buildFanTempText(themeColor string) string {
	var lines []string

	if len(lastCPUMetrics.TempSensors) > 0 {
		lines = append(lines, buildGroupedTempLines(lastCPUMetrics.TempSensors, themeColor)...)
	} else {
		lines = append(lines, fmt.Sprintf("[No temperature sensors detected](fg:%s)", themeColor))
	}

	return renderScrollableLines(lines, themeColor)
}

// buildFanControlText renders a compact single-line status bar
func buildFanControlText(themeColor string) string {
	if fanControl {
		return fmt.Sprintf("[⚠ FAN CONTROL ACTIVE](fg:red,mod:bold)  [+/-](fg:%s,mod:bold) Speed  [a](fg:%s,mod:bold) Auto  [0/9](fg:%s,mod:bold) Min/Max  [R](fg:green,mod:bold) Reset  [l](fg:%s) Layout  [F](fg:%s) Exit",
			themeColor, themeColor, themeColor, themeColor, themeColor)
	}
	return fmt.Sprintf("[Read-only](fg:%s)  Use --fan-control to enable writes  |  [l](fg:%s,mod:bold) Layout  [F](fg:%s,mod:bold) Exit fan view",
		themeColor, themeColor, themeColor)
}
