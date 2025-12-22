package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func runHeadless(count int) {
	if err := initSocMetrics(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize metrics: %v\n", err)
		os.Exit(1)
	}
	defer cleanupSocMetrics()

	if prometheusPort != "" {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(prometheusPort, nil); err != nil {
				fmt.Fprintf(os.Stderr, "Prometheus server error: %v\n", err)
			}
		}()
	}

	ticker := time.NewTicker(time.Duration(updateInterval) * time.Millisecond)
	defer ticker.Stop()

	type HeadlessOutput struct {
		Timestamp             string                `json:"timestamp"`
		SocMetrics            SocMetrics            `json:"soc_metrics"`
		Memory                MemoryMetrics         `json:"memory"`
		NetDisk               NetDiskMetrics        `json:"net_disk"`
		CPUUsage              float64               `json:"cpu_usage"`
		GPUUsage              float64               `json:"gpu_usage"`
		CoreUsages            []float64             `json:"core_usages"`
		SystemInfo            SystemInfo            `json:"system_info"`
		ThermalState          string                `json:"thermal_state"`
		ThunderboltInfo       *ThunderboltOutput    `json:"thunderbolt_info"`
		ThunderboltNetStats   []ThunderboltNetStats `json:"thunderbolt_net_stats,omitempty"`
		TBNetTotalBytesInSec  float64               `json:"tb_net_total_bytes_in_per_sec"`
		TBNetTotalBytesOutSec float64               `json:"tb_net_total_bytes_out_per_sec"`
		RDMAStatus            RDMAStatus            `json:"rdma_status"`
		CPUTemp               float32               `json:"cpu_temp"`
		GPUTemp               float32               `json:"gpu_temp"`
	}

	encoder := json.NewEncoder(os.Stdout)

	GetCPUPercentages()

	if count > 0 {
		fmt.Print("[")
	}

	// Fetch formatted info
	tbInfo, _ := GetFormattedThunderboltInfo()

	samplesCollected := 0
	for range ticker.C {
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

		// Refresh TB info if needed, but for now we use the cached one
		// tbInfo, _ = GetThunderboltInfo()

		tbNetStats := GetThunderboltNetStats()
		var tbNetTotalIn, tbNetTotalOut float64
		for _, stat := range tbNetStats {
			tbNetTotalIn += stat.BytesInPerSec
			tbNetTotalOut += stat.BytesOutPerSec
		}

		output := HeadlessOutput{
			Timestamp:             time.Now().Format(time.RFC3339),
			SocMetrics:            m,
			Memory:                mem,
			NetDisk:               netDisk,
			CPUUsage:              cpuUsage,
			GPUUsage:              m.GPUActive,
			CoreUsages:            percentages,
			SystemInfo:            getSOCInfo(),
			ThunderboltInfo:       tbInfo,
			ThunderboltNetStats:   tbNetStats,
			TBNetTotalBytesInSec:  tbNetTotalIn,
			TBNetTotalBytesOutSec: tbNetTotalOut,
			RDMAStatus:            CheckRDMAAvailable(),
			ThermalState:          thermalStr,
			CPUTemp:               m.CPUTemp,
			GPUTemp:               m.GPUTemp,
		}

		if samplesCollected > 0 && count > 0 {
			fmt.Print(",")
		}

		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		}

		samplesCollected++
		if count > 0 && samplesCollected >= count {
			fmt.Println("]")
			return
		}
	}
}
