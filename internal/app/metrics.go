package app

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
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

	netStats, err := net.IOCounters(false)
	if err == nil && len(netStats) > 0 {
		current := netStats[0]
		if lastNetDiskTime.IsZero() {
			lastNetStats = current
		} else {
			metrics.InBytesPerSec = float64(current.BytesRecv-lastNetStats.BytesRecv) / elapsed
			metrics.OutBytesPerSec = float64(current.BytesSent-lastNetStats.BytesSent) / elapsed
			metrics.InPacketsPerSec = float64(current.PacketsRecv-lastNetStats.PacketsRecv) / elapsed
			metrics.OutPacketsPerSec = float64(current.PacketsSent-lastNetStats.PacketsSent) / elapsed
		}
		lastNetStats = current
	}

	diskStats, err := disk.IOCounters()
	if err == nil {
		var totalReadBytes, totalWriteBytes, totalReadOps, totalWriteOps uint64
		for _, d := range diskStats {
			totalReadBytes += d.ReadBytes
			totalWriteBytes += d.WriteBytes
			totalReadOps += d.ReadCount
			totalWriteOps += d.WriteCount
		}
		if !lastNetDiskTime.IsZero() {
			metrics.ReadKBytesPerSec = float64(totalReadBytes-lastDiskStats.ReadBytes) / elapsed / 1024
			metrics.WriteKBytesPerSec = float64(totalWriteBytes-lastDiskStats.WriteBytes) / elapsed / 1024
			metrics.ReadOpsPerSec = float64(totalReadOps-lastDiskStats.ReadCount) / elapsed
			metrics.WriteOpsPerSec = float64(totalWriteOps-lastDiskStats.WriteCount) / elapsed
		}
		lastDiskStats = disk.IOCountersStat{
			ReadBytes:  totalReadBytes,
			WriteBytes: totalWriteBytes,
			ReadCount:  totalReadOps,
			WriteCount: totalWriteOps,
		}
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
			CPUW:      m.CPUPower,
			GPUW:      m.GPUPower,
			ANEW:      m.ANEPower,
			DRAMW:     m.DRAMPower,
			GPUSRAMW:  m.GPUSRAMPower,
			SystemW:   systemResidual,
			PackageW:  totalPower,
			Throttled: throttled,
			CPUTemp:   float64(m.CPUTemp),
			GPUTemp:   float64(m.GPUTemp),
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
	v, _ := mem.VirtualMemory()
	s, _ := mem.SwapMemory()
	totalMemory := v.Total
	usedMemory := v.Used
	availableMemory := v.Available
	swapTotal := s.Total
	swapUsed := s.Used
	return MemoryMetrics{
		Total:     totalMemory,
		Used:      usedMemory,
		Available: availableMemory,
		SwapTotal: swapTotal,
		SwapUsed:  swapUsed,
	}
}
