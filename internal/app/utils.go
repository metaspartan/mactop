package app

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	ui "github.com/metaspartan/gotui/v5"
)

func UpdateCachedTerminalDimensions(w, h int) {
	cachedTermMutex.Lock()
	defer cachedTermMutex.Unlock()
	cachedTermWidth = w
	cachedTermHeight = h
}

func GetCachedTerminalDimensions() (int, int) {
	cachedTermMutex.RLock()
	if cachedTermWidth != 0 && cachedTermHeight != 0 {
		width, height := cachedTermWidth, cachedTermHeight
		cachedTermMutex.RUnlock()
		return width, height
	}
	cachedTermMutex.RUnlock()

	// Fallback or lazy init
	w, h := ui.TerminalDimensions()
	cachedTermMutex.Lock()
	cachedTermWidth = w
	cachedTermHeight = h
	cachedTermMutex.Unlock()
	return w, h
}

func StderrToLogfile(logfile *os.File) {
	syscall.Dup2(int(logfile.Fd()), 2)
}

func parseTimeString(timeStr string) float64 {
	var hours, minutes int
	var seconds float64
	if strings.Contains(timeStr, "h") {
		parts := strings.Split(timeStr, "h")
		fmt.Sscanf(parts[0], "%d", &hours)
		fmt.Sscanf(parts[1], "%d:%f", &minutes, &seconds)
	} else {
		fmt.Sscanf(timeStr, "%d:%f", &minutes, &seconds)
	}
	return float64(hours*3600) + float64(minutes*60) + seconds
}

func formatTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) / 60) % 60
	secs := int(seconds) % 60
	centisecs := int((seconds - float64(int(seconds))) * 100)

	if hours > 0 {
		return fmt.Sprintf("%dh%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d.%02d", minutes, secs, centisecs)
}

func formatMemorySize(kb int64) string {
	const (
		MB = 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case kb >= TB:
		return fmt.Sprintf("%.1fT", float64(kb)/float64(TB))
	case kb >= GB:
		return fmt.Sprintf("%.1fG", float64(kb)/float64(GB))
	case kb >= MB:
		return fmt.Sprintf("%dM", kb/MB)
	default:
		return fmt.Sprintf("%dK", kb)
	}
}

func formatResMemorySize(kb int64) string {
	const (
		MB = 1024
		GB = MB * 1024
	)
	switch {
	case kb >= GB:
		return fmt.Sprintf("%.1fG", float64(kb)/float64(GB))
	case kb >= MB:
		return fmt.Sprintf("%dM", kb/MB)
	default:
		return fmt.Sprintf("%dK", kb)
	}
}

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 3 {
		return "..."
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func max(nums ...int) int {
	maxVal := nums[0]
	for _, num := range nums[1:] {
		if num > maxVal {
			maxVal = num
		}
	}
	return maxVal
}

func formatBytes(val float64, unitType string) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}

	targetUnit := strings.ToLower(unitType)
	if targetUnit == "" {
		targetUnit = "auto"
	}

	value := val
	suffix := ""

	switch targetUnit {
	case "byte":
		suffix = "B"
	case "kb":
		value /= 1024
		suffix = "KB"
	case "mb":
		value /= 1024 * 1024
		suffix = "MB"
	case "gb":
		value /= 1024 * 1024 * 1024
		suffix = "GB"
	case "auto":
		i := 0
		for value >= 1000 && i < len(units)-1 {
			value /= 1024
			i++
		}
		suffix = units[i]
	default:
		i := 0
		for value >= 1000 && i < len(units)-1 {
			value /= 1024
			i++
		}
		suffix = units[i]
	}

	return fmt.Sprintf("%.1f%s", value, suffix)
}

func formatTemp(celsius float64) string {
	if strings.ToLower(tempUnit) == "fahrenheit" {
		f := (celsius * 9 / 5) + 32
		return fmt.Sprintf("%d°F", int(f))
	}
	return fmt.Sprintf("%d°C", int(celsius))
}

// isCompactLayout returns true if the current layout is one of the compact layouts (tiny, micro, nano, pico)
func isCompactLayout() bool {
	layout := currentConfig.DefaultLayout
	return layout == LayoutTiny || layout == LayoutMicro || layout == LayoutNano || layout == LayoutPico
}
