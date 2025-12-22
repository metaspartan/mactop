package app

import (
	"fmt"
	"strings"

	ui "github.com/metaspartan/gotui/v4"
	"github.com/shirou/gopsutil/v4/host"
)

func buildInfoLines(themeColor string) []string {
	uptimeSeconds, _ := host.Uptime()
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
		formatLine("RDMA", rdmaLabel),
	}

	// Add Thunderbolt device info (use cached version)
	if cachedTBDeviceInfo != "" {
		tbLines := strings.Split(cachedTBDeviceInfo, "\n")
		for _, line := range tbLines {
			if line != "" {
				infoLines = append(infoLines, fmt.Sprintf("[%s](fg:%s)", line, themeColor))
			}
		}
	}

	infoLines = append(infoLines,
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
	)

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
	themeColor := "green"
	if currentConfig.Theme != "" {
		themeColor = currentConfig.Theme
	}
	if IsLightMode && themeColor == "white" {
		themeColor = "black"
	}

	infoLines := buildInfoLines(themeColor)
	asciiArt := getASCIIArt()

	termWidth, termHeight := ui.TerminalDimensions()
	showAscii := termWidth >= 82

	contentWidth := 80
	if !showAscii {
		contentWidth = 45
	}

	maxHeight := len(infoLines)
	if showAscii {
		if len(asciiArt) > maxHeight {
			maxHeight = len(asciiArt)
		}
	}
	contentHeight := maxHeight

	paddingTop := (termHeight - contentHeight) / 2
	if paddingTop > 5 {
		paddingTop = paddingTop - 5
	}
	if paddingTop < 0 {
		paddingTop = 0
	}

	paddingLeft := (termWidth - contentWidth) / 2
	if paddingLeft < 0 {
		paddingLeft = 0
	}
	paddingStr := strings.Repeat(" ", paddingLeft)

	var combinedText strings.Builder
	combinedText.WriteString(strings.Repeat("\n", paddingTop))

	rainbowColors := []string{"red", "magenta", "blue", "skyblue", "green", "yellow"}

	for i := 0; i < maxHeight; i++ {
		asciiLine := ""
		if showAscii {
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

		if showAscii {
			combinedText.WriteString(fmt.Sprintf("%s%s   %s\n", paddingStr, asciiLine, infoLine))
		} else {
			combinedText.WriteString(fmt.Sprintf("%s%s\n", paddingStr, infoLine))
		}
	}

	return combinedText.String()
}
