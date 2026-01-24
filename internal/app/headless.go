package app

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/toon-format/toon-go"
	"gopkg.in/yaml.v3"
)

func safeFloat64At(slice []float64, index int) float64 {
	if index >= 0 && index < len(slice) {
		return slice[index]
	}
	return 0.0
}

type HeadlessOutput struct {
	Timestamp             string             `json:"timestamp" yaml:"timestamp" xml:"Timestamp" toon:"timestamp"`
	SocMetrics            SocMetrics         `json:"soc_metrics" yaml:"soc_metrics" xml:"SocMetrics" toon:"soc_metrics"`
	Memory                MemoryMetrics      `json:"memory" yaml:"memory" xml:"Memory" toon:"memory"`
	NetDisk               NetDiskMetrics     `json:"net_disk" yaml:"net_disk" xml:"NetDisk" toon:"net_disk"`
	CPUUsage              float64            `json:"cpu_usage" yaml:"cpu_usage" xml:"CPUUsage" toon:"cpu_usage"`
	ECPUUsage             []float64          `json:"ecpu_usage" yaml:"ecpu_usage" xml:"ECPUUsage" toon:"ecpu_usage"`
	PCPUUsage             []float64          `json:"pcpu_usage" yaml:"pcpu_usage" xml:"PCPUUsage" toon:"pcpu_usage"`
	GPUUsage              float64            `json:"gpu_usage" yaml:"gpu_usage" xml:"GPUUsage" toon:"gpu_usage"`
	TFLOPsFP32            float64            `json:"tflops_fp32" yaml:"tflops_fp32" xml:"TFLOPsFP32" toon:"tflops_fp32"`
	TFLOPsFP16            float64            `json:"tflops_fp16" yaml:"tflops_fp16" xml:"TFLOPsFP16" toon:"tflops_fp16"`
	CoreUsages            []float64          `json:"core_usages" yaml:"core_usages" xml:"CoreUsages" toon:"core_usages"`
	SystemInfo            SystemInfo         `json:"system_info" yaml:"system_info" xml:"SystemInfo" toon:"system_info"`
	ThermalState          string             `json:"thermal_state" yaml:"thermal_state" xml:"ThermalState" toon:"thermal_state"`
	ThunderboltInfo       *ThunderboltOutput `json:"thunderbolt_info" yaml:"thunderbolt_info" xml:"ThunderboltInfo" toon:"thunderbolt_info"`
	TBNetTotalBytesInSec  float64            `json:"tb_net_total_bytes_in_per_sec" yaml:"tb_net_total_bytes_in_per_sec" xml:"TBNetTotalBytesInSec" toon:"tb_net_total_bytes_in_per_sec"`
	TBNetTotalBytesOutSec float64            `json:"tb_net_total_bytes_out_per_sec" yaml:"tb_net_total_bytes_out_per_sec" xml:"TBNetTotalBytesOutSec" toon:"tb_net_total_bytes_out_per_sec"`
	RDMAStatus            RDMAStatus         `json:"rdma_status" yaml:"rdma_status" xml:"RDMAStatus" toon:"rdma_status"`
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
	case "json", "yaml", "xml", "toon", "csv":
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s. Defaulting to json.\n", format)
		format = "json"
	}

	tbInfo := performHeadlessWarmup()

	printHeadlessStart(format, count)

	// Setup signal handling for graceful shutdown (to close XML tags)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	samplesCollected := 0

	// Cache SystemInfo since it doesn't change
	cachedHeadlessSysInfo := getSOCInfo()

	// First manual collection
	if err := processHeadlessSample(format, tbInfo, cachedHeadlessSysInfo); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
	}
	samplesCollected++

	if count > 0 && samplesCollected >= count {
		printHeadlessEnd(format, count)
		return
	}

	ticker := time.NewTicker(time.Duration(updateInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			printHeadlessEnd(format, count)
			return
		case <-ticker.C:
			printHeadlessSeparator(format, count, samplesCollected)

			if err := processHeadlessSample(format, tbInfo, cachedHeadlessSysInfo); err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
			}

			samplesCollected++
			if count > 0 && samplesCollected >= count {
				printHeadlessEnd(format, count)
				return
			}
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
		case "csv":
			printCSVHeader()
		}
	} else {
		switch format {
		case "xml":
			// XML always needs a root element, even in infinite mode
			fmt.Print("<MactopOutputList>")
		case "csv":
			printCSVHeader()
		}
	}
}

func printCSVHeader() {
	headers := []string{
		"Timestamp",
		"System_Name", "Core_Count", "E_Core_Count", "P_Core_Count", "GPU_Core_Count",
		"CPU_Usage", "ECPU_Freq_MHz", "ECPU_Active", "PCPU_Freq_MHz", "PCPU_Active", "GPU_Usage",
		"Mem_Used", "Mem_Total", "Swap_Used",
		"Disk_Read_KB", "Disk_Write_KB",
		"Net_In_Bytes", "Net_Out_Bytes",
		"TB_Net_In_Bytes", "TB_Net_Out_Bytes",
		"Total_Power", "System_Power",
		"CPU_Temp", "GPU_Temp", "Thermal_State",
		"RDMA_Available", "RDMA_Status", "RDMA_Device_Count",
	}

	// Add dynamic core headers
	sysInfo := getSOCInfo()
	for i := 0; i < sysInfo.CoreCount; i++ {
		headers = append(headers, fmt.Sprintf("Core_%d", i))
	}

	// Add JSON blob header for complex nested data
	headers = append(headers, "Thunderbolt_Info_JSON")

	// Print CSV header line
	fmt.Println(strings.Join(headers, ","))
}

func printHeadlessEnd(format string, count int) {
	if count > 0 {
		switch format {
		case "json":
			fmt.Println("]")
		case "xml":
			fmt.Println("</MactopOutputList>")
		}
	} else if format == "xml" {
		fmt.Println("</MactopOutputList>")
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
		// Even for infinite stream, YAML docs are best separated by ---
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

func processHeadlessSample(format string, tbInfo *ThunderboltOutput, sysInfo SystemInfo) error {
	output := collectHeadlessData(tbInfo, sysInfo)
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
	case "csv":
		// Use encoding/csv for correct escaping
		writer := csv.NewWriter(os.Stdout)

		var record []string

		// Standard fields
		record = append(record,
			output.Timestamp,
			output.SystemInfo.Name,
			fmt.Sprintf("%d", output.SystemInfo.CoreCount),
			fmt.Sprintf("%d", output.SystemInfo.ECoreCount),
			fmt.Sprintf("%d", output.SystemInfo.PCoreCount),
			fmt.Sprintf("%d", output.SystemInfo.GPUCoreCount),
			fmt.Sprintf("%.2f", output.CPUUsage),
			fmt.Sprintf("%.2f", safeFloat64At(output.ECPUUsage, 0)),
			fmt.Sprintf("%.2f", safeFloat64At(output.ECPUUsage, 1)),
			fmt.Sprintf("%.2f", safeFloat64At(output.PCPUUsage, 0)),
			fmt.Sprintf("%.2f", safeFloat64At(output.PCPUUsage, 1)),
			fmt.Sprintf("%.2f", output.GPUUsage),
			fmt.Sprintf("%d", output.Memory.Used),
			fmt.Sprintf("%d", output.Memory.Total),
			fmt.Sprintf("%d", output.Memory.SwapUsed),
			fmt.Sprintf("%.2f", output.NetDisk.ReadKBytesPerSec),
			fmt.Sprintf("%.2f", output.NetDisk.WriteKBytesPerSec),
			fmt.Sprintf("%.2f", output.NetDisk.InBytesPerSec),
			fmt.Sprintf("%.2f", output.NetDisk.OutBytesPerSec),
			fmt.Sprintf("%.2f", output.TBNetTotalBytesInSec),
			fmt.Sprintf("%.2f", output.TBNetTotalBytesOutSec),
			fmt.Sprintf("%.2f", output.SocMetrics.TotalPower),
			fmt.Sprintf("%.2f", output.SocMetrics.SystemPower),
			fmt.Sprintf("%.2f", output.SocMetrics.CPUTemp),
			fmt.Sprintf("%.2f", output.SocMetrics.GPUTemp),
			output.ThermalState,
			fmt.Sprintf("%t", output.RDMAStatus.Available),
			output.RDMAStatus.Status,
			fmt.Sprintf("%d", len(output.RDMAStatus.Devices)),
		)

		for i := 0; i < output.SystemInfo.CoreCount; i++ {
			val := 0.0
			if i < len(output.CoreUsages) {
				val = output.CoreUsages[i]
			}
			record = append(record, fmt.Sprintf("%.2f", val))
		}

		tbJSON, _ := json.Marshal(output.ThunderboltInfo)
		record = append(record, string(tbJSON))

		writer.Write(record)
		writer.Flush()
		return nil
	}

	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func collectHeadlessData(tbInfo *ThunderboltOutput, sysInfo SystemInfo) HeadlessOutput {
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

	// Get RDMA status and map devices to TB buses
	rdmaStatus := CheckRDMAAvailable()
	mapRDMADevicesToBuses(rdmaStatus.Devices, tbInfo)

	// Calculate TFLOPs
	var fp32TFLOPs, fp16TFLOPs float64
	maxGPUFreq := GetMaxGPUFrequency()
	if maxGPUFreq > 0 && sysInfo.GPUCoreCount > 0 {
		fp32TFLOPs = float64(sysInfo.GPUCoreCount) * float64(maxGPUFreq) * 0.000256
		fp16TFLOPs = fp32TFLOPs * 2
	}

	return HeadlessOutput{
		Timestamp:             time.Now().Format(time.RFC3339),
		SocMetrics:            m,
		Memory:                mem,
		NetDisk:               netDisk,
		CPUUsage:              cpuUsage,
		ECPUUsage:             []float64{float64(m.EClusterFreqMHz), m.EClusterActive},
		PCPUUsage:             []float64{float64(m.PClusterFreqMHz), m.PClusterActive},
		GPUUsage:              m.GPUActive,
		TFLOPsFP32:            fp32TFLOPs,
		TFLOPsFP16:            fp16TFLOPs,
		CoreUsages:            percentages,
		SystemInfo:            sysInfo,
		ThunderboltInfo:       tbInfo,
		TBNetTotalBytesInSec:  tbNetTotalIn,
		TBNetTotalBytesOutSec: tbNetTotalOut,
		RDMAStatus:            rdmaStatus,
		ThermalState:          thermalStr,
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

// mapRDMADevicesToBuses associates RDMA devices with their corresponding TB buses
// by matching the RDMA device interface (e.g., "en2") with the bus NetworkStats interface
func mapRDMADevicesToBuses(rdmaDevices []RDMADevice, tbInfo *ThunderboltOutput) {
	if tbInfo == nil || len(rdmaDevices) == 0 {
		return
	}

	// Build a map of interface name to RDMA device for quick lookup
	rdmaByInterface := make(map[string]*RDMADevice)
	for i := range rdmaDevices {
		if rdmaDevices[i].Interface != "" {
			rdmaByInterface[rdmaDevices[i].Interface] = &rdmaDevices[i]
		}
	}

	// Match RDMA devices to buses based on NetworkStats interface name
	for i := range tbInfo.Buses {
		bus := &tbInfo.Buses[i]
		if bus.NetworkStats != nil && bus.NetworkStats.InterfaceName != "" {
			if rdmaDev, ok := rdmaByInterface[bus.NetworkStats.InterfaceName]; ok {
				bus.RDMADevice = rdmaDev
			}
		}
	}
}
