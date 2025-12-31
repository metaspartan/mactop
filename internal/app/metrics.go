package app

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func startPrometheusServer(port string) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(cpuUsage)
	registry.MustRegister(ecoreUsage)
	registry.MustRegister(pcoreUsage)
	registry.MustRegister(gpuUsage)
	registry.MustRegister(gpuFreqMHz)
	registry.MustRegister(powerUsage)
	registry.MustRegister(socTemp)
	registry.MustRegister(gpuTemp)
	registry.MustRegister(thermalState)
	registry.MustRegister(memoryUsage)
	registry.MustRegister(networkSpeed)
	registry.MustRegister(diskIOSpeed)
	registry.MustRegister(diskIOPS)
	registry.MustRegister(tbNetworkSpeed)
	registry.MustRegister(rdmaAvailable)
	registry.MustRegister(cpuCoreUsage)
	registry.MustRegister(systemInfoGauge)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	http.Handle("/metrics", handler)
	go func() {
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			stderrLogger.Printf("Failed to start Prometheus metrics server: %v\n", err)
		}
	}()
}

func GetCPUPercentages() ([]float64, error) {
	currentTimes, err := GetCPUUsage()
	if err != nil {
		return nil, err
	}
	if firstRun {
		lastCPUTimes = currentTimes
		firstRun = false
		return make([]float64, len(currentTimes)), nil
	}
	percentages := make([]float64, len(currentTimes))
	for i := range currentTimes {
		totalDelta := (currentTimes[i].User - lastCPUTimes[i].User) +
			(currentTimes[i].System - lastCPUTimes[i].System) +
			(currentTimes[i].Idle - lastCPUTimes[i].Idle) +
			(currentTimes[i].Nice - lastCPUTimes[i].Nice)

		activeDelta := (currentTimes[i].User - lastCPUTimes[i].User) +
			(currentTimes[i].System - lastCPUTimes[i].System) +
			(currentTimes[i].Nice - lastCPUTimes[i].Nice)

		if totalDelta > 0 {
			percentages[i] = (activeDelta / totalDelta) * 100.0
		}
		if percentages[i] < 0 {
			percentages[i] = 0
		} else if percentages[i] > 100 {
			percentages[i] = 100
		}
	}
	lastCPUTimes = currentTimes
	return percentages, nil
}

func getNetDiskMetrics() NetDiskMetrics {
	var metrics NetDiskMetrics

	netDiskMutex.Lock()
	defer netDiskMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(lastNetDiskTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	// Native Network Metrics
	netMap, err := GetNativeNetworkMetrics()
	if err == nil {
		var totalNet NativeNetMetric
		for _, iface := range netMap {
			totalNet.BytesRecv += iface.BytesRecv
			totalNet.BytesSent += iface.BytesSent
			totalNet.PacketsRecv += iface.PacketsRecv
			totalNet.PacketsSent += iface.PacketsSent
		}

		if lastNetDiskTime.IsZero() {
			lastNetStats = totalNet
		} else {
			metrics.InBytesPerSec = float64(totalNet.BytesRecv-lastNetStats.BytesRecv) / elapsed
			metrics.OutBytesPerSec = float64(totalNet.BytesSent-lastNetStats.BytesSent) / elapsed
			metrics.InPacketsPerSec = float64(totalNet.PacketsRecv-lastNetStats.PacketsRecv) / elapsed
			metrics.OutPacketsPerSec = float64(totalNet.PacketsSent-lastNetStats.PacketsSent) / elapsed
		}
		lastNetStats = totalNet
	}

	// Native Disk Metrics
	diskMap, err := GetNativeDiskMetrics()
	if err == nil {
		var totalDisk NativeDiskMetric
		for _, d := range diskMap {
			totalDisk.ReadBytes += d.ReadBytes
			totalDisk.WriteBytes += d.WriteBytes
			totalDisk.ReadOps += d.ReadOps
			totalDisk.WriteOps += d.WriteOps
		}

		if !lastNetDiskTime.IsZero() {
			metrics.ReadKBytesPerSec = float64(totalDisk.ReadBytes-lastDiskStats.ReadBytes) / elapsed / 1024
			metrics.WriteKBytesPerSec = float64(totalDisk.WriteBytes-lastDiskStats.WriteBytes) / elapsed / 1024
			metrics.ReadOpsPerSec = float64(totalDisk.ReadOps-lastDiskStats.ReadOps) / elapsed
			metrics.WriteOpsPerSec = float64(totalDisk.WriteOps-lastDiskStats.WriteOps) / elapsed
		}
		lastDiskStats = totalDisk
	}

	networkSpeed.With(prometheus.Labels{"direction": "upload"}).Set(metrics.OutBytesPerSec)
	networkSpeed.With(prometheus.Labels{"direction": "download"}).Set(metrics.InBytesPerSec)
	diskIOSpeed.With(prometheus.Labels{"operation": "read"}).Set(metrics.ReadKBytesPerSec * 1024)
	diskIOSpeed.With(prometheus.Labels{"operation": "write"}).Set(metrics.WriteKBytesPerSec * 1024)
	diskIOPS.With(prometheus.Labels{"operation": "read"}).Set(metrics.ReadOpsPerSec)
	diskIOPS.With(prometheus.Labels{"operation": "write"}).Set(metrics.WriteOpsPerSec)

	lastNetDiskTime = now
	return metrics
}

func collectNetDiskMetrics(done chan struct{}, netdiskMetricsChan chan NetDiskMetrics) {
	for {
		start := time.Now()

		netdiskMetrics := getNetDiskMetrics()
		select {
		case <-done:
			return
		case netdiskMetricsChan <- netdiskMetrics:
		default:
		}

		elapsed := time.Since(start)
		sleepTime := time.Duration(updateInterval)*time.Millisecond - elapsed
		if sleepTime > 0 {
			select {
			case <-time.After(sleepTime):
			case <-interruptChan:
			}
		}
	}
}

func collectMetrics(done chan struct{}, cpumetricsChan chan CPUMetrics, gpumetricsChan chan GPUMetrics, tbNetStatsChan chan []ThunderboltNetStats) {
	for {
		start := time.Now()

		sampleDuration := updateInterval
		if sampleDuration < 100 {
			sampleDuration = 100
		}

		m := sampleSocMetrics(sampleDuration / 2)

		_, throttled := getThermalStateString()

		componentSum := m.TotalPower
		totalPower := componentSum
		systemResidual := 0.0

		if m.SystemPower > componentSum {
			totalPower = m.SystemPower
			systemResidual = m.SystemPower - componentSum
		}

		cpuMetrics := CPUMetrics{
			CPUW:            m.CPUPower,
			GPUW:            m.GPUPower,
			ANEW:            m.ANEPower,
			DRAMW:           m.DRAMPower,
			GPUSRAMW:        m.GPUSRAMPower,
			SystemW:         systemResidual,
			PackageW:        totalPower,
			Throttled:       throttled,
			CPUTemp:         float64(m.CPUTemp),
			GPUTemp:         float64(m.GPUTemp),
			EClusterActive:  int(m.EClusterActive),
			PClusterActive:  int(m.PClusterActive),
			EClusterFreqMHz: int(m.EClusterFreqMHz),
			PClusterFreqMHz: int(m.PClusterFreqMHz),
		}

		gpuMetrics := GPUMetrics{
			FreqMHz:       int(m.GPUFreqMHz),
			ActivePercent: m.GPUActive,
			Power:         m.GPUPower + m.GPUSRAMPower,
			Temp:          m.GPUTemp,
		}

		tbNetStats := GetThunderboltNetStats()

		select {
		case <-done:
			return
		case cpumetricsChan <- cpuMetrics:
		default:
		}
		select {
		case gpumetricsChan <- gpuMetrics:
		default:
		}
		select {
		case tbNetStatsChan <- tbNetStats:
		default:
		}

		elapsed := time.Since(start)
		sleepTime := time.Duration(updateInterval)*time.Millisecond - elapsed
		if sleepTime > 0 {
			select {
			case <-time.After(sleepTime):
			case <-interruptChan:
			}
		}
	}
}

func collectProcessMetrics(done chan struct{}, processMetricsChan chan []ProcessMetrics) {
	for {
		start := time.Now()

		select {
		case <-done:
			return
		default:
			if processes, err := getProcessList(); err == nil {
				processMetricsChan <- processes
			} else {
				stderrLogger.Printf("Error getting process list: %v\n", err)
			}
		}

		elapsed := time.Since(start)
		sleepTime := time.Duration(updateInterval)*time.Millisecond - elapsed
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
	}
}

func getMemoryMetrics() MemoryMetrics {
	native, err := GetNativeMemoryMetrics()
	if err != nil {
		stderrLogger.Printf("Error getting native memory metrics: %v\n", err)
		return MemoryMetrics{}
	}
	return MemoryMetrics{
		Total:     native.Total,
		Used:      native.Used,
		Available: native.Available,
		SwapTotal: native.SwapTotal,
		SwapUsed:  native.SwapUsed,
	}
}
