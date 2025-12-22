package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func handleFlag(arg string, idx int, args []string) (int, string, int, bool, bool, error) {
	colorName := ""
	interval := 0
	setColor := false
	setInterval := false

	switch arg {
	case "--help", "-h":
		fmt.Print(`Usage: mactop [options]

Options:
  -h, --help            Show this help message
  -v, --version         Show the version of mactop
  -i, --interval <ms>   Set the update interval in milliseconds (default: 1000)
  -c, --color <color>   Set the UI color (green, red, blue, skyblue, magenta, yellow, gold, silver, white)
  -p, --prometheus <port> Run Prometheus metrics server on specified port (e.g. :9090)
      --headless        Run in headless mode (no TUI, output JSON to stdout)
      --count <n>       Number of samples to collect in headless mode (0 = infinite)
      --dump-ioreport, -d Dump all available IOReport channels and exit
      --unit-network <unit> Network unit: auto, byte, kb, mb, gb (default: auto)
      --unit-disk <unit>    Disk unit: auto, byte, kb, mb, gb (default: auto)
      --unit-temp <unit>    Temperature unit: celsius, fahrenheit (default: celsius)


For more information, see https://github.com/metaspartan/mactop written by Carsen Klock.
`)
		os.Exit(0)
	case "--version", "-v":
		fmt.Println("mactop version:", version)
		os.Exit(0)
	case "--test", "-t":
		if idx+1 < len(args) {
			testInput := args[idx+1]
			fmt.Printf("Test input received: %s\n", testInput)
			os.Exit(0)
		}
	case "--testapp", "-a":
		runTestApp()
	case "--color", "-c":
		if idx+1 < len(args) {
			colorName = strings.ToLower(args[idx+1])
			setColor = true
			return idx + 1, colorName, interval, setColor, setInterval, nil
		}
		return idx, "", 0, false, false, fmt.Errorf("Error: --color flag requires a color value")
	case "--prometheus", "-p":
		if idx+1 < len(args) {
			prometheusPort = args[idx+1]
			return idx + 1, "", 0, false, false, nil
		}
		return idx, "", 0, false, false, fmt.Errorf("Error: --prometheus flag requires a port number")
	case "--interval", "-i":
		if idx+1 < len(args) {
			var err error
			interval, err = strconv.Atoi(args[idx+1])
			if err != nil {
				return idx, "", 0, false, false, fmt.Errorf("Invalid interval: %v", err)
			}
			setInterval = true
			return idx + 1, "", interval, false, setInterval, nil
		}
		return idx, "", 0, false, false, fmt.Errorf("Error: --interval flag requires an interval value")
	case "--dump-ioreport", "-d":
		fmt.Println("Dumping IOReport channels...")
		DebugIOReport()
		os.Exit(0)
	case "--monitor-ioreport", "-m":
		fmt.Println("Monitoring IOReport channels for changes (5s sample)...")
		MonitorIOReport()
		os.Exit(0)
	}
	return idx, "", 0, false, false, nil
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
