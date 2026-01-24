package app

import (
	"fmt"
	"regexp"
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
	}

	volumes := getVolumes()
	if len(volumes) > 0 {
		infoLines = append(infoLines, "-------------------------")
		for _, v := range volumes {
			used := formatBytes(v.Used*1e9, diskUnit)
			total := formatBytes(v.Total*1e9, diskUnit)
			avail := formatBytes(v.Available*1e9, diskUnit)
			infoLines = append(infoLines, formatLine(v.Name, fmt.Sprintf("%s / %s (%s free)", used, total, avail)))
		}
	}

	infoLines = append(infoLines, "-------------------------")

	tbIn := formatBytes(lastTBInBytes, networkUnit)
	tbOut := formatBytes(lastTBOutBytes, networkUnit)
	infoLines = append(infoLines, formatLine("TB Net", fmt.Sprintf("↑ %s/s ↓ %s/s", tbOut, tbIn)))

	infoLines = append(infoLines, formatLine("RDMA", rdmaLabel))

	tbInfoMutex.Lock()
	tbInfo := tbDeviceInfo
	tbInfoMutex.Unlock()

	for line := range strings.Lines(tbInfo) {
		if line != "" {
			infoLines = append(infoLines, fmt.Sprintf("[%s](fg:%s)", line, themeColor))
		}
	}

	return infoLines
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
