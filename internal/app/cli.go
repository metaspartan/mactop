package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/metaspartan/mactop/v2/internal/i18n"
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
		fmt.Printf(i18n.T("CLI_Version")+"\n", version)
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
	case "--pid":
		return handlePIDFlag(idx, args)
	case "--dump-ioreport", "-d":
		fmt.Println(i18n.T("CLI_DumpingIOReport"))
		DebugIOReport()
		os.Exit(0)
	}
	return emptyResult(idx).values()
}

func printHelpAndExit() {
	fmt.Print(i18n.T("CLI_HelpText"))
	os.Exit(0)
}

func handleTestFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		fmt.Printf(i18n.T("CLI_TestInputReceived")+"\n", args[idx+1])
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
	return errorResult(idx, i18n.T("CLI_ErrorForegroundRequiresValue")).values()
}

func handleBgFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		bgColor := args[idx+1]
		if !IsHexColor(bgColor) {
			bgColor = strings.ToLower(bgColor)
		}
		cliBgColor = bgColor
		return emptyResult(idx + 1).values()
	}
	return errorResult(idx, i18n.T("CLI_ErrorBackgroundRequiresValue")).values()
}

func handlePrometheusFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		prometheusPort = args[idx+1]
		return emptyResult(idx + 1).values()
	}
	return errorResult(idx, i18n.T("CLI_ErrorPrometheusRequiresValue")).values()
}

func handleIntervalFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		interval, err := strconv.Atoi(args[idx+1])
		if err != nil {
			return errorResult(idx, fmt.Sprintf(i18n.T("CLI_ErrorInvalidInterval"), err)).values()
		}
		return intervalResult(idx, interval).values()
	}
	return errorResult(idx, i18n.T("CLI_ErrorIntervalRequiresValue")).values()
}

func handlePIDFlag(idx int, args []string) (int, string, int, bool, bool, error) {
	if idx+1 < len(args) {
		pid, err := strconv.Atoi(args[idx+1])
		if err != nil {
			return errorResult(idx, fmt.Sprintf(i18n.T("CLI_ErrorInvalidPID"), err)).values()
		}
		filterPID = pid
		return emptyResult(idx + 1).values()
	}
	return errorResult(idx, i18n.T("CLI_ErrorPIDRequiresValue")).values()
}

func runTestApp() {
	fmt.Println(i18n.T("CLI_TestingIOReportPowerMetrics"))
	initSocMetrics()
	for i := range 3 {
		m := sampleSocMetrics(500)
		thermalStr, _ := getThermalStateString()
		fmt.Printf(i18n.T("CLI_TestSample")+"\n", i+1)
		fmt.Printf(i18n.T("CLI_TestSocTemp")+"\n", m.SocTemp)
		fmt.Printf(i18n.T("CLI_TestCPU")+"\n",
			m.CPUPower, m.GPUPower, m.GPUFreqMHz, m.GPUActive)
		fmt.Printf(i18n.T("CLI_TestANE")+"\n",
			m.ANEPower, m.DRAMPower, m.GPUSRAMPower, m.TotalPower, thermalStr)
		fmt.Println()
	}
	cleanupSocMetrics()
	os.Exit(0)
}

func handleLegacyFlags() (string, int, bool, bool) {
	// NOTE: i18n is initialized lazily on first T() call (system language)
	// and re-initialized with the fully resolved language in Run() after
	// config is loaded. This avoids a double-init with different languages.

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
