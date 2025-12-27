package app

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/toon-format/toon-go"
	"gopkg.in/yaml.v3"
)

type HeadlessOutput struct {
	Timestamp             string             `json:"timestamp" yaml:"timestamp" xml:"Timestamp" toon:"timestamp"`
	SocMetrics            SocMetrics         `json:"soc_metrics" yaml:"soc_metrics" xml:"SocMetrics" toon:"soc_metrics"`
	Memory                MemoryMetrics      `json:"memory" yaml:"memory" xml:"Memory" toon:"memory"`
	NetDisk               NetDiskMetrics     `json:"net_disk" yaml:"net_disk" xml:"NetDisk" toon:"net_disk"`
	CPUUsage              float64            `json:"cpu_usage" yaml:"cpu_usage" xml:"CPUUsage" toon:"cpu_usage"`
	GPUUsage              float64            `json:"gpu_usage" yaml:"gpu_usage" xml:"GPUUsage" toon:"gpu_usage"`
	CoreUsages            []float64          `json:"core_usages" yaml:"core_usages" xml:"CoreUsages" toon:"core_usages"`
	SystemInfo            SystemInfo         `json:"system_info" yaml:"system_info" xml:"SystemInfo" toon:"system_info"`
	ThermalState          string             `json:"thermal_state" yaml:"thermal_state" xml:"ThermalState" toon:"thermal_state"`
	ThunderboltInfo       *ThunderboltOutput `json:"thunderbolt_info" yaml:"thunderbolt_info" xml:"ThunderboltInfo" toon:"thunderbolt_info"`
	TBNetTotalBytesInSec  float64            `json:"tb_net_total_bytes_in_per_sec" yaml:"tb_net_total_bytes_in_per_sec" xml:"TBNetTotalBytesInSec" toon:"tb_net_total_bytes_in_per_sec"`
	TBNetTotalBytesOutSec float64            `json:"tb_net_total_bytes_out_per_sec" yaml:"tb_net_total_bytes_out_per_sec" xml:"TBNetTotalBytesOutSec" toon:"tb_net_total_bytes_out_per_sec"`
	RDMAStatus            RDMAStatus         `json:"rdma_status" yaml:"rdma_status" xml:"RDMAStatus" toon:"rdma_status"`
	CPUTemp               float32            `json:"cpu_temp" yaml:"cpu_temp" xml:"CPUTemp" toon:"cpu_temp"`
	GPUTemp               float32            `json:"gpu_temp" yaml:"gpu_temp" xml:"GPUTemp" toon:"gpu_temp"`
}

func runHeadless(count int) {
	if err := initSocMetrics(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize metrics: %v\n", err)
		os.Exit(1)
	}
	defer cleanupSocMetrics()

	startHeadlessPrometheus()

	// Validate format
	format := strings.ToLower(headlessFormat)
	switch format {
	case "json", "yaml", "xml", "toon":
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s. Defaulting to json.\n", format)
		format = "json"
	}

	tbInfo := performHeadlessWarmup()

	printHeadlessStart(format, count)

	samplesCollected := 0

	// First manual collection
	if err := processHeadlessSample(format, tbInfo); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
	}
	samplesCollected++

	if count > 0 && samplesCollected >= count {
		printHeadlessEnd(format, count)
		return
	}

	ticker := time.NewTicker(time.Duration(updateInterval) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		printHeadlessSeparator(format, count, samplesCollected)

		if err := processHeadlessSample(format, tbInfo); err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		}

		samplesCollected++
		if count > 0 && samplesCollected >= count {
			printHeadlessEnd(format, count)
			return
		}
	}
}

func printHeadlessStart(format string, count int) {
	if count > 0 {
		switch format {
		case "json":
			fmt.Print("[")
		case "xml":
			fmt.Print("<MactopOutputList>")
		}
	}
}

func printHeadlessEnd(format string, count int) {
	if count > 0 {
		switch format {
		case "json":
			fmt.Println("]")
		case "xml":
			fmt.Println("</MactopOutputList>")
		}
	}
}

func printHeadlessSeparator(format string, count int, samplesCollected int) {
	if samplesCollected > 0 && count > 0 {
		switch format {
		case "json":
			fmt.Print(",")
		case "yaml":
			fmt.Println("---")
		}
	} else if format == "yaml" {
		fmt.Println("---")
	}
}

func startHeadlessPrometheus() {
	if prometheusPort != "" {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(prometheusPort, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Prometheus server error: %v\n", err)
			}
		}()
	}
}

func performHeadlessWarmup() *ThunderboltOutput {
	GetCPUPercentages()
	getNetDiskMetrics()
	GetThunderboltNetStats()

	startInit := time.Now()
	tbInfo, _ := GetFormattedThunderboltInfo()
	initDuration := time.Since(startInit)

	initialDelay := time.Duration(updateInterval)*time.Millisecond - initDuration
	if initialDelay > 0 {
		time.Sleep(initialDelay)
	}
	return tbInfo
}

func processHeadlessSample(format string, tbInfo *ThunderboltOutput) error {
	output := collectHeadlessData(tbInfo)
	var data []byte
	var err error

	switch format {
	case "json":
		if headlessPretty {
			data, err = json.MarshalIndent(output, "", "  ")
		} else {
			data, err = json.Marshal(output)
		}
	case "yaml":
		data, err = yaml.Marshal(output)
	case "xml":
		if headlessPretty {
			data, err = xml.MarshalIndent(output, "", "  ")
		} else {
			data, err = xml.Marshal(output)
		}
	case "toon":
		data, err = toon.Marshal(output)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func collectHeadlessData(tbInfo *ThunderboltOutput) HeadlessOutput {
	m := sampleSocMetrics(updateInterval)
	mem := getMemoryMetrics()
	netDisk := getNetDiskMetrics()

	var cpuUsage float64
	percentages, err := GetCPUPercentages()
	if err == nil && len(percentages) > 0 {
		var total float64
		for _, p := range percentages {
			total += p
		}
		cpuUsage = total / float64(len(percentages))
	}

	thermalStr, _ := getThermalStateString()

	componentSum := m.TotalPower
	totalPower := m.SystemPower

	if totalPower < componentSum {
		totalPower = componentSum
	}

	residualSystem := totalPower - componentSum

	m.SystemPower = residualSystem
	m.TotalPower = totalPower

	tbNetStats := GetThunderboltNetStats()
	var tbNetTotalIn, tbNetTotalOut float64
	for _, stat := range tbNetStats {
		tbNetTotalIn += stat.BytesInPerSec
		tbNetTotalOut += stat.BytesOutPerSec
	}

	mapTBNetStatsToBuses(tbNetStats, tbInfo)

	return HeadlessOutput{
		Timestamp:             time.Now().Format(time.RFC3339),
		SocMetrics:            m,
		Memory:                mem,
		NetDisk:               netDisk,
		CPUUsage:              cpuUsage,
		GPUUsage:              m.GPUActive,
		CoreUsages:            percentages,
		SystemInfo:            getSOCInfo(),
		ThunderboltInfo:       tbInfo,
		TBNetTotalBytesInSec:  tbNetTotalIn,
		TBNetTotalBytesOutSec: tbNetTotalOut,
		RDMAStatus:            CheckRDMAAvailable(),
		ThermalState:          thermalStr,
		CPUTemp:               m.CPUTemp,
		GPUTemp:               m.GPUTemp,
	}
}

func mapTBNetStatsToBuses(tbNetStats []ThunderboltNetStats, tbInfo *ThunderboltOutput) {
	// Sort and assign TB Net Stats to Buses
	var enStats []ThunderboltNetStats
	for _, stat := range tbNetStats {
		if strings.HasPrefix(stat.InterfaceName, "en") {
			enStats = append(enStats, stat)
		}
	}

	// Sort en stats by interface number (en2, en3, ...)
	sort.Slice(enStats, func(i, j int) bool {
		// Extract number from enX
		getNu := func(s string) int {
			numStr := strings.TrimPrefix(s, "en")
			n, _ := strconv.Atoi(numStr)
			return n
		}
		return getNu(enStats[i].InterfaceName) < getNu(enStats[j].InterfaceName)
	})

	// Assign to buses based on sorted order (Ordinal Mapping)
	// We sort buses by ID (0, 1, 2...) and map them to sorted interfaces (en2, en3, en4...)
	if tbInfo != nil && len(tbInfo.Buses) > 0 {
		type busIndex struct {
			originalIndex int
			id            int
		}
		var sortedBuses []busIndex

		for i, bus := range tbInfo.Buses {
			// Format is typically "TB4 Bus 5" or "TB4 @ TB3 Bus 3"
			// The bus number is always the last element
			parts := strings.Fields(bus.Name)
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				if busID, err := strconv.Atoi(lastPart); err == nil {
					sortedBuses = append(sortedBuses, busIndex{i, busID})
				}
			}
		}

		// Sort buses by ID
		sort.Slice(sortedBuses, func(i, j int) bool {
			return sortedBuses[i].id < sortedBuses[j].id
		})

		// Assign stats ordinally
		for i := 0; i < len(enStats) && i < len(sortedBuses); i++ {
			// Get the target bus using the original index from our sorted list
			busIdx := sortedBuses[i].originalIndex
			if busIdx >= 0 && busIdx < len(tbInfo.Buses) {
				stat := enStats[i] // Copy for safe pointer reference
				tbInfo.Buses[busIdx].NetworkStats = &stat
			}
		}
	}
}
