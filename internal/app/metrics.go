package app

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func startPrometheusServer(port string) {
	port = strings.TrimPrefix(port, ":")
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
	registry.MustRegister(scoreUsage)
	registry.MustRegister(dramBandwidth)
	registry.MustRegister(batteryPercent)
	registry.MustRegister(batteryCharging)
	registry.MustRegister(cpuCoreUsage)
	registry.MustRegister(systemInfoGauge)
	registry.MustRegister(fanRPM)
	registry.MustRegister(tempSensorGauge)

	initializePrometheusSeries(getSOCInfo())

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	http.Handle("/metrics", handler)
	go func() {
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			stderrLogger.Printf("Failed to start Prometheus metrics server: %v\n", err)
		}
	}()
}

type prometheusMetricsSnapshot struct {
	SystemInfo   SystemInfo
	CPUMetrics   CPUMetrics
	GPUMetrics   GPUMetrics
	Memory       MemoryMetrics
	TBNetStats   []ThunderboltNetStats
	RDMAStatus   RDMAStatus
	ThermalLevel thermalStateLevel
}

func initializePrometheusSeries(sysInfo SystemInfo) {
	updatePrometheusSystemInfo(sysInfo)

	for _, component := range []string{"cpu", "gpu", "ane", "dram", "gpu_sram", "system", "total"} {
		powerUsage.With(prometheus.Labels{"component": component}).Set(0)
	}
	for _, memoryType := range []string{"used", "total", "swap_used", "swap_total"} {
		memoryUsage.With(prometheus.Labels{"type": memoryType}).Set(0)
	}
	for _, direction := range []string{"upload", "download"} {
		networkSpeed.With(prometheus.Labels{"direction": direction}).Set(0)
	}
	for _, operation := range []string{"read", "write"} {
		diskIOSpeed.With(prometheus.Labels{"operation": operation}).Set(0)
		diskIOPS.With(prometheus.Labels{"operation": operation}).Set(0)
	}
	for _, direction := range []string{"read", "write", "combined"} {
		dramBandwidth.With(prometheus.Labels{"direction": direction}).Set(0)
	}
	for _, direction := range []string{"upload", "download"} {
		tbNetworkSpeed.With(prometheus.Labels{"direction": direction}).Set(0)
	}
	for i := 0; i < sysInfo.CoreCount; i++ {
		cpuCoreUsage.With(prometheus.Labels{"core": fmt.Sprintf("%d", i), "type": coreTypeForIndex(i, sysInfo)}).Set(0)
	}
}

func normalizeSocMetricsPower(m SocMetrics) SocMetrics {
	componentSum := m.TotalPower
	totalPower := m.SystemPower
	if totalPower < componentSum {
		totalPower = componentSum
	}
	m.SystemPower = totalPower - componentSum
	m.TotalPower = totalPower
	return m
}

func cpuMetricsFromSoc(m SocMetrics, coreUsages []float64, avgUsage float64, throttled bool) CPUMetrics {
	return CPUMetrics{
		CPUW:            m.CPUPower,
		GPUW:            m.GPUPower,
		ANEW:            m.ANEPower,
		ANEActive:        m.ANEActive,
		ANEPowered:       m.ANEPowered,
		ANEExclave:       m.ANEExclave,
		ANEClusterCount:  m.ANEClusterCount,
		ANEClusterActive: m.ANEClusterActive,
		ANEReadBW:       m.ANEReadBW,
		ANEWriteBW:      m.ANEWriteBW,
		DRAMW:           m.DRAMPower,
		GPUSRAMW:        m.GPUSRAMPower,
		SystemW:         m.SystemPower,
		PackageW:        m.TotalPower,
		Throttled:       throttled,
		CPUTemp:         float64(m.CPUTemp),
		GPUTemp:         float64(m.GPUTemp),
		EClusterActive:  int(m.EClusterActive),
		PClusterActive:  int(m.PClusterActive),
		EClusterFreqMHz: int(m.EClusterFreqMHz),
		PClusterFreqMHz: int(m.PClusterFreqMHz),
		SClusterActive:  int(m.SClusterActive),
		SClusterFreqMHz: int(m.SClusterFreqMHz),
		DRAMReadBW:      m.DRAMReadBW,
		DRAMWriteBW:     m.DRAMWriteBW,
		DRAMBWCombined:  m.DRAMBWCombined,
		ANEBW:           m.ANEBWCombined,
		Fans:            m.Fans,
		TempSensors:     m.TempSensors,
		CoreUsages:      coreUsages,
		AvgUsage:        avgUsage,
	}
}

// aneMaxPowerW is the assumed full-tilt ANE power draw used for the
// power-based utilization estimate (historical mactop behavior).
const aneMaxPowerW = 8.0

// aneBWRefFloorGBs is the minimum bandwidth treated as "100% ANE activity"
// for the bandwidth-based estimate. A saturating conv workload measured
// ~3.5-3.8 GB/s of ANE fabric traffic on M1 Ultra; the reference grows
// adaptively if higher bandwidth is ever observed (maxANEBWSeen).
const aneBWRefFloorGBs = 4.0

// aneUtilizationPercent estimates Neural Engine utilization. It prefers the
// power-based estimate from the Energy Model ANE channel; when that channel
// is dead (macOS 27 beta zeroed all per-block energy counters) but the AMC
// "ANE RD/WR" byte counters show traffic, it falls back to a bandwidth-based
// activity estimate so ANE usage doesn't silently read 0 on newer OSes.
func aneUtilizationPercent(m CPUMetrics) float64 {
	// 0a. Exclave ANE (M5 / M5 Max): CurrentPowerState is pinned high by
	// background macOS ML services, so the duty cycle is only meaningful as a
	// binary powered/idle state. Collapse to 0/100 and — crucially — do NOT
	// latch bandwidth/residency modes here: those make the gauge fall through to
	// a %-form label and render this binary value as a misleading percentage.
	if m.ANEExclave {
		if m.ANEActive > 0 {
			return 100
		}
		return 0
	}
	// 0b. IORegistry power-domain duty cycle (non-exclave Ultra dies on macOS 27
	// when PMP channels are empty for non-root): binary powered/idle per window.
	// Do NOT latch bandwidth mode here (like the exclave tier above): this signal
	// has no bandwidth component, and aneBWLabelMode already exempts ANEPowered so
	// the gauge shows "powered/idle (W)" rather than a misleading "@ 0.00 GB/s".
	if m.ANEPowered {
		pct := m.ANEActive
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
		return pct
	}
	// 1. PMP state-residency utilization (macOS 27+/M5): true time-above-
	//    idle-floor measurement parsed from the ANE-AF-BW / ANE-DCS-BW
	//    channels — the most accurate signal where it exists. Latch the
	//    bandwidth-form label: residency flowing while watts read 0 proves
	//    the energy counter is dead.
	if m.ANEActive > 0 {
		// This machine has a live PMP residency signal (M5-class). Latch it so
		// the history chart plots stored residency percentages as-is rather
		// than re-deriving them from bandwidth (which is a different quantity
		// and would diverge from this gauge).
		aneResidencyLatched.Store(true)
		if m.ANEW <= 0 {
			aneBWModeLatched.Store(true)
		}
		pct := m.ANEActive
		if pct > 100 {
			pct = 100
		}
		return pct
	}
	// 2. Energy Model power estimate (macOS 26 and any OS with working
	//    per-block energy counters).
	if m.ANEW > 0 {
		pct := m.ANEW / aneMaxPowerW * 100
		if pct > 100 {
			pct = 100
		}
		return pct
	}
	// 3. Bandwidth activity estimate (M1-M4 on macOS 27: AMC byte counters).
	if m.ANEBW > 0 {
		// ANE traffic with zero watts proves the energy counter is dead (an
		// idle ANE produces neither). Latch bandwidth mode for the session so
		// the UI label stays in GB/s form even when traffic later drops to 0,
		// instead of reverting to a misleading "@ 0.00 W".
		aneBWModeLatched.Store(true)
		// Monotonic session max via CAS (callers run on several goroutines).
		// 3% ratchet hysteresis: a single burst-aligned sample window
		// marginally above the sustained plateau would otherwise become the
		// permanent 100% reference, pinning genuine saturation at a
		// misleading 96-98%. Bursts within 3% read as 100% via the clamp
		// below; real step-ups beyond 3% still re-scale the reference.
		for {
			cur := math.Float64frombits(maxANEBWSeenBits.Load())
			if m.ANEBW <= cur*1.03 {
				break
			}
			if maxANEBWSeenBits.CompareAndSwap(math.Float64bits(cur), math.Float64bits(m.ANEBW)) {
				break
			}
		}
		ref := max(math.Float64frombits(maxANEBWSeenBits.Load()), aneBWRefFloorGBs)
		pct := m.ANEBW / ref * 100
		if pct > 100 {
			pct = 100
		}
		return pct
	}
	return 0
}

// aneBWLabelMode reports whether ANE displays should use the bandwidth-form
// label (GB/s) instead of watts. True while the power channel yields nothing
// and either traffic is currently flowing or bandwidth mode was latched
// earlier this session. On OSes with a working energy counter (macOS 26) the
// latch never trips, so labels behave exactly as before.
func aneBWLabelMode(m CPUMetrics) bool {
	// The binary IORegistry power-state tiers have no non-root bandwidth signal
	// and render as a powered/idle word, so they never use the bandwidth-form
	// label: exclave ANE (M5 / M5 Max) and the non-exclave power-state fallback
	// (Ultra dies on macOS 27 where PMP is empty). For the latter, m.ANEActive is
	// the 0/100 duty cycle, which would otherwise trip the (m.ANEActive > 0) term
	// below and force a misleading "@ 0.00 GB/s" label every powered sample.
	if m.ANEExclave || m.ANEPowered {
		return false
	}
	return m.ANEW <= 0 && (m.ANEActive > 0 || m.ANEBW > 0 || aneBWModeLatched.Load())
}

func gpuMetricsFromSoc(m SocMetrics) GPUMetrics {
	return GPUMetrics{
		FreqMHz:       int(m.GPUFreqMHz),
		ActivePercent: m.GPUActive,
		Power:         m.GPUPower + m.GPUSRAMPower,
		Temp:          m.GPUTemp,
	}
}

func averageCPUUsage(coreUsages []float64) float64 {
	if len(coreUsages) == 0 {
		return 0
	}
	total := 0.0
	for _, usage := range coreUsages {
		total += usage
	}
	return total / float64(len(coreUsages))
}

func averageCoreRange(coreUsages []float64, start, count int) float64 {
	if count <= 0 || start < 0 || len(coreUsages) < start+count {
		return 0
	}
	total := 0.0
	for _, usage := range coreUsages[start : start+count] {
		total += usage
	}
	return total / float64(count)
}

func calculateCoreAveragesForSystem(coreUsages []float64, sysInfo SystemInfo) (ecoreAvg, pcoreAvg, scoreAvg float64) {
	ecoreAvg = averageCoreRange(coreUsages, 0, sysInfo.ECoreCount)
	pcoreAvg = averageCoreRange(coreUsages, sysInfo.ECoreCount, sysInfo.PCoreCount)
	scoreAvg = averageCoreRange(coreUsages, sysInfo.ECoreCount+sysInfo.PCoreCount, sysInfo.SCoreCount)
	return ecoreAvg, pcoreAvg, scoreAvg
}

func coreTypeForIndex(index int, sysInfo SystemInfo) string {
	if index < sysInfo.ECoreCount {
		return "e"
	}
	if index < sysInfo.ECoreCount+sysInfo.PCoreCount {
		return "p"
	}
	return "s"
}

// prometheusThermalStateValue maps the pressure level to a numeric gauge value
// matching the OSThermalPressureLevel scale (Nominal=0 .. Sleeping=4). Unknown
// reports 0.
func prometheusThermalStateValue(level thermalStateLevel) float64 {
	switch level {
	case thermalStateModerate:
		return 1
	case thermalStateHeavy:
		return 2
	case thermalStateTrapping:
		return 3
	case thermalStateSleeping:
		return 4
	default:
		return 0
	}
}

func updatePrometheusSystemInfo(sysInfo SystemInfo) {
	systemInfoGauge.With(prometheus.Labels{
		"model":          sysInfo.Name,
		"core_count":     fmt.Sprintf("%d", sysInfo.CoreCount),
		"e_core_count":   fmt.Sprintf("%d", sysInfo.ECoreCount),
		"p_core_count":   fmt.Sprintf("%d", sysInfo.PCoreCount),
		"s_core_count":   fmt.Sprintf("%d", sysInfo.SCoreCount),
		"gpu_core_count": fmt.Sprintf("%d", sysInfo.GPUCoreCount),
	}).Set(1)
}

func publishPrometheusMetrics(snapshot prometheusMetricsSnapshot) {
	// Skip all Set/With calls when the exporter is disabled. Each Gauge.Set on
	// a *Vec metric allocates a prometheus.Labels map plus fmt.Sprintf strings
	// per fan / sensor / core — pure waste in TUI-only mode and the dominant
	// per-tick allocation source on systems with many temperature sensors.
	if prometheusPort == "" {
		return
	}
	updatePrometheusSystemInfo(snapshot.SystemInfo)

	cpuMetrics := snapshot.CPUMetrics
	totalUsage := cpuMetrics.AvgUsage
	if len(cpuMetrics.CoreUsages) > 0 {
		totalUsage = averageCPUUsage(cpuMetrics.CoreUsages)
	}
	ecoreAvg, pcoreAvg, scoreAvg := calculateCoreAveragesForSystem(cpuMetrics.CoreUsages, snapshot.SystemInfo)

	cpuUsage.Set(totalUsage)
	ecoreUsage.Set(ecoreAvg)
	pcoreUsage.Set(pcoreAvg)
	scoreUsage.Set(scoreAvg)
	powerUsage.With(prometheus.Labels{"component": "cpu"}).Set(cpuMetrics.CPUW)
	powerUsage.With(prometheus.Labels{"component": "gpu"}).Set(cpuMetrics.GPUW)
	powerUsage.With(prometheus.Labels{"component": "ane"}).Set(cpuMetrics.ANEW)
	powerUsage.With(prometheus.Labels{"component": "dram"}).Set(cpuMetrics.DRAMW)
	powerUsage.With(prometheus.Labels{"component": "gpu_sram"}).Set(cpuMetrics.GPUSRAMW)
	powerUsage.With(prometheus.Labels{"component": "system"}).Set(cpuMetrics.SystemW)
	powerUsage.With(prometheus.Labels{"component": "total"}).Set(cpuMetrics.PackageW)
	socTemp.Set(cpuMetrics.CPUTemp)
	gpuTemp.Set(cpuMetrics.GPUTemp)
	thermalState.Set(prometheusThermalStateValue(snapshot.ThermalLevel))
	dramBandwidth.With(prometheus.Labels{"direction": "read"}).Set(cpuMetrics.DRAMReadBW)
	dramBandwidth.With(prometheus.Labels{"direction": "write"}).Set(cpuMetrics.DRAMWriteBW)
	dramBandwidth.With(prometheus.Labels{"direction": "combined"}).Set(cpuMetrics.DRAMBWCombined)

	memoryUsage.With(prometheus.Labels{"type": "used"}).Set(float64(snapshot.Memory.Used) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "total"}).Set(float64(snapshot.Memory.Total) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "swap_used"}).Set(float64(snapshot.Memory.SwapUsed) / 1024 / 1024 / 1024)
	memoryUsage.With(prometheus.Labels{"type": "swap_total"}).Set(float64(snapshot.Memory.SwapTotal) / 1024 / 1024 / 1024)

	for i, usage := range cpuMetrics.CoreUsages {
		cpuCoreUsage.With(prometheus.Labels{"core": fmt.Sprintf("%d", i), "type": coreTypeForIndex(i, snapshot.SystemInfo)}).Set(usage)
	}

	gpuUsage.Set(snapshot.GPUMetrics.ActivePercent)
	gpuFreqMHz.Set(float64(snapshot.GPUMetrics.FreqMHz))

	updatePrometheusThunderbolt(snapshot.TBNetStats, snapshot.RDMAStatus)
	updatePrometheusSensors(cpuMetrics.Fans, cpuMetrics.TempSensors)

	// -1 is the "battery unavailable" sentinel — used both when there is no
	// battery and when one is present but its capacity is unreadable (Percent
	// would otherwise be a bogus -1 charge level).
	if bat := GetBatteryInfo(); bat.Displayable() {
		batteryPercent.Set(float64(*bat.Percent))
		if bat.Charging {
			batteryCharging.Set(1)
		} else {
			batteryCharging.Set(0)
		}
	} else {
		batteryPercent.Set(-1)
		batteryCharging.Set(0)
	}
}

func updatePrometheusThunderbolt(tbStats []ThunderboltNetStats, rdmaStatus RDMAStatus) {
	var totalBytesIn, totalBytesOut float64
	for _, stat := range tbStats {
		totalBytesIn += stat.BytesInPerSec
		totalBytesOut += stat.BytesOutPerSec
	}
	tbNetworkSpeed.With(prometheus.Labels{"direction": "download"}).Set(totalBytesIn)
	tbNetworkSpeed.With(prometheus.Labels{"direction": "upload"}).Set(totalBytesOut)
	if rdmaStatus.Available {
		rdmaAvailable.Set(1)
	} else {
		rdmaAvailable.Set(0)
	}
}

func publishPrometheusNetDiskMetrics(metrics NetDiskMetrics) {
	// Same allocation-avoidance guard as publishPrometheusMetrics — every
	// .With(prometheus.Labels{...}).Set() builds a transient labels map, and
	// this is called every tick from collectNetDiskMetrics regardless of
	// whether the exporter is enabled.
	if prometheusPort == "" {
		return
	}
	networkSpeed.With(prometheus.Labels{"direction": "upload"}).Set(metrics.OutBytesPerSec / 1024)
	networkSpeed.With(prometheus.Labels{"direction": "download"}).Set(metrics.InBytesPerSec / 1024)
	diskIOSpeed.With(prometheus.Labels{"operation": "read"}).Set(metrics.ReadKBytesPerSec)
	diskIOSpeed.With(prometheus.Labels{"operation": "write"}).Set(metrics.WriteKBytesPerSec)
	diskIOPS.With(prometheus.Labels{"operation": "read"}).Set(metrics.ReadOpsPerSec)
	diskIOPS.With(prometheus.Labels{"operation": "write"}).Set(metrics.WriteOpsPerSec)
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

	lastNetDiskTime = now
	return metrics
}

func collectNetDiskMetrics(done chan struct{}, netdiskMetricsChan chan NetDiskMetrics) {
	for {
		start := time.Now()

		netdiskMetrics := getNetDiskMetrics()
		publishPrometheusNetDiskMetrics(netdiskMetrics)
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

// dispatchMetrics sends metrics to channels without blocking, checking done for exit.
func dispatchMetrics(done chan struct{}, cpuCh chan CPUMetrics, gpuCh chan GPUMetrics,
	tbCh chan []ThunderboltNetStats, triggerCh chan struct{},
	cpu CPUMetrics, gpu GPUMetrics, tb []ThunderboltNetStats) bool {
	select {
	case <-done:
		return true
	case cpuCh <- cpu:
	default:
	}
	select {
	case gpuCh <- gpu:
	default:
	}
	select {
	case tbCh <- tb:
	default:
	}
	select {
	case triggerCh <- struct{}{}:
	default:
	}
	return false
}

func collectMetrics(done chan struct{}, cpumetricsChan chan CPUMetrics, gpumetricsChan chan GPUMetrics, tbNetStatsChan chan []ThunderboltNetStats, triggerProcessCollectionChan chan struct{}) {
	// Pre-calculate static info
	sysInfo := getSOCInfo()
	maxGPUFreq := GetMaxGPUFrequency()
	var maxFP32TFLOPs float64
	if maxGPUFreq > 0 && sysInfo.GPUCoreCount > 0 {
		maxFP32TFLOPs = float64(sysInfo.GPUCoreCount) * float64(maxGPUFreq) * 0.000256
	}

	for {
		start := time.Now()

		sampleDuration := max(updateInterval, 100)

		m := normalizeSocMetricsPower(sampleSocMetrics(sampleDuration / 2))

		thermalLevel := getThermalStateLevel()
		thermalStr := thermalStateString(thermalLevel)
		throttled := thermalStateThrottled(thermalLevel)
		rdmaStatus := CheckRDMAAvailable()
		rdmaStat := rdmaStatus.Status

		coreUsages, _ := GetCPUPercentages()
		avgUsage := averageCPUUsage(coreUsages)
		cpuMetrics := cpuMetricsFromSoc(m, coreUsages, avgUsage, throttled)
		gpuMetrics := gpuMetricsFromSoc(m)

		// Compute frequency-adjusted effective GPU load (used by history_soc)
		if gpuMetrics.FreqMHz > 0 {
			maxFreq := GetGPUMaxFreqMHz()
			if maxFreq > 0 {
				eff := gpuMetrics.ActivePercent * (float64(gpuMetrics.FreqMHz) / float64(maxFreq))
				if eff > 100 {
					eff = 100
				}
				gpuMetrics.EffectiveLoad = eff
			} else {
				gpuMetrics.EffectiveLoad = gpuMetrics.ActivePercent
			}
		} else {
			gpuMetrics.EffectiveLoad = gpuMetrics.ActivePercent
		}

		tbNetStats := GetThunderboltNetStats()
		publishPrometheusMetrics(prometheusMetricsSnapshot{
			SystemInfo:   sysInfo,
			CPUMetrics:   cpuMetrics,
			GPUMetrics:   gpuMetrics,
			Memory:       getMemoryMetrics(),
			TBNetStats:   tbNetStats,
			RDMAStatus:   rdmaStatus,
			ThermalLevel: thermalLevel,
		})

		if dispatchMetrics(done, cpumetricsChan, gpumetricsChan, tbNetStatsChan, triggerProcessCollectionChan, cpuMetrics, gpuMetrics, tbNetStats) {
			return
		}

		// Push to menubar worker — snapshot net metrics under lock to avoid race
		if menubar {
			renderMutex.Lock()
			nd := lastNetDiskMetrics
			renderMutex.Unlock()
			pushMenuBarMetricsToWorker(m, cpuMetrics, gpuMetrics, nd, sysInfo, maxFP32TFLOPs, cpuMetrics.AvgUsage, thermalStr, rdmaStat)
		}

		// Push to overlay worker
		if overlay {
			renderMutex.Lock()
			nd := lastNetDiskMetrics
			renderMutex.Unlock()
			pushOverlayMetrics(m, cpuMetrics, gpuMetrics, nd, sysInfo, maxFP32TFLOPs, cpuMetrics.AvgUsage, thermalStr, rdmaStat)
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

func updatePrometheusSensors(fans []FanInfo, sensors []TempSensor) {
	for _, fan := range fans {
		fanRPM.With(prometheus.Labels{"fan_id": fmt.Sprintf("%d", fan.ID), "fan_name": fan.Name}).Set(float64(fan.ActualRPM))
	}
	for _, sensor := range sensors {
		tempSensorGauge.With(prometheus.Labels{"key": sensor.Key, "name": sensor.Name}).Set(sensor.Value)
	}
}

func collectProcessMetrics(done chan struct{}, processMetricsChan chan []ProcessMetrics, triggerChan chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-triggerChan:
			renderMutex.Lock()
			sysPct := lastGPUMetrics.ActivePercent
			renderMutex.Unlock()

			if processes, err := getProcessList(sysPct); err == nil {
				processMetricsChan <- processes
			} else {
				stderrLogger.Printf("Error getting process list: %v\n", err)
			}
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
