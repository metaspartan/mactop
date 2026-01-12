package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// flagResult is a helper to construct common flag return values
type flagResult struct {
	idx, interval         int
	colorName             string
	setColor, setInterval bool
	err                   error
}

func (r flagResult) values() (int, string, int, bool, bool, error) {
	return r.idx, r.colorName, r.interval, r.setColor, r.setInterval, r.err
}

func emptyResult(idx int) flagResult {
	return flagResult{idx: idx}
}

func colorResult(idx int, name string) flagResult {
	return flagResult{idx: idx + 1, colorName: name, setColor: true}
}

func intervalResult(idx, val int) flagResult {
	return flagResult{idx: idx + 1, interval: val, setInterval: true}
}

func errorResult(idx int, msg string) flagResult {
	return flagResult{idx: idx, err: fmt.Errorf("%s", msg)}
}

func handleFlag(arg string, idx int, args []string) (int, string, int, bool, bool, error) {
	switch arg {
	case "--help", "-h":
		printHelpAndExit()
	case "--version", "-v":
		fmt.Println("mactop version:", version)
		os.Exit(0)
	case "--test", "-t":
		return handleTestFlag(idx, args)
	case "--testapp", "-a":
		runTestApp()
	case "--foreground":
		return handleForegroundFlag(idx, args)
	case "--bg", "--background":
		return handleBgFlag(idx, args)
	case "--prometheus", "-p":
		return handlePrometheusFlag(idx, args)
	case "--interval", "-i":
		return handleIntervalFlag(idx, args)
	case "--dump-ioreport", "-d":
		fmt.Println("Dumping IOReport channels...")
		DebugIOReport()
		os.Exit(0)
	}
	return emptyResult(idx).values()
}

func printHelpAndExit() {
	fmt.Print(`Usage: mactop [options]

Options:
  -h, --help              Show this help message
  -v, --version           Show the version of mactop
  -i, --interval <ms>     Set the update interval in milliseconds (default: 1000)
  --foreground <color>    Set the UI foreground color (named or hex, e.g., green, #9580FF)
  --bg <hex>              Set the UI background color (hex, e.g., #22212C)
  -p, --prometheus <port> Run Prometheus metrics server on specified port (e.g. :9090)
      --headless          Run in headless mode (no TUI, output JSON to stdout)
      --format <format>   Set the output format (json, toon, etc.)
      --pretty            Pretty print JSON output in headless mode
      --count <n>         Number of samples to collect in headless mode (0 = infinite)
      --dump-ioreport, -d Dump all available IOReport channels and exit
      --unit-network <unit> Network unit: auto, byte, kb, mb, gb (default: auto)
      --unit-disk <unit>    Disk unit: auto, byte, kb, mb, gb (default: auto)
      --unit-temp <unit>    Temperature unit: celsius, fahrenheit (default: celsius)

Theme File:
  Create ~/.mactop/theme.json with custom hex colors:
  {"foreground": "#9580FF", "background": "#22212C"}


For more information, see https://github.com/metaspartan/mactop written by Carsen Klock.
`)
	os.Exit(0)
}

func handleTestFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		fmt.Printf("Test input received: %s\n", args[idx+1])
		os.Exit(0)
	}
	return emptyResult(idx).values()
}

func handleForegroundFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		colorName := args[idx+1]
		if !IsHexColor(colorName) {
			colorName = strings.ToLower(colorName)
		}
		return colorResult(idx, colorName).values()
	}
	return errorResult(idx, "Error: --foreground flag requires a color value").values()
}

func handleBgFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		cliBgColor = args[idx+1]
		return emptyResult(idx + 1).values()
	}
	return errorResult(idx, "Error: --bg flag requires a hex color value").values()
}

func handlePrometheusFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		prometheusPort = args[idx+1]
		return emptyResult(idx + 1).values()
	}
	return errorResult(idx, "Error: --prometheus flag requires a port number").values()
}

func handleIntervalFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		interval, err := strconv.Atoi(args[idx+1])
		if err != nil {
			return errorResult(idx, fmt.Sprintf("Invalid interval: %v", err)).values()
		}
		return intervalResult(idx, interval).values()
	}
	return errorResult(idx, "Error: --interval flag requires an interval value").values()
}

func runTestApp() {
	fmt.Println("Testing IOReport power metrics...")
	initSocMetrics()
	for i := 0; i < 3; i++ {
		m := sampleSocMetrics(500)
		thermalStr, _ := getThermalStateString()
		fmt.Printf("Sample %d:\n", i+1)
		fmt.Printf("  SoC Temp: %.1f°C\n", m.SocTemp)
		fmt.Printf("  CPU: %.2fW | GPU: %.2fW (%d MHz, %.0f%% active)\n",
			m.CPUPower, m.GPUPower, m.GPUFreqMHz, m.GPUActive)
		fmt.Printf("  ANE: %.2fW | DRAM: %.2fW | GPU SRAM: %.2fW | Total: %.2fW | %s\n",
			m.ANEPower, m.DRAMPower, m.GPUSRAMPower, m.TotalPower, thermalStr)
		fmt.Println()
	}
	cleanupSocMetrics()
	os.Exit(0)
}

func handleLegacyFlags() (string, int, bool, bool) {
	var (
		colorName             string
		interval              int
		setColor, setInterval bool
	)
	for i := 1; i < len(os.Args); i++ {
		newI, cName, intVal, isColor, isInt, err := handleFlag(os.Args[i], i, os.Args)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if isColor {
			colorName = cName
			setColor = true
		}
		if isInt {
			interval = intVal
			setInterval = true
		}
		i = newI
	}
	return colorName, interval, setColor, setInterval
}
