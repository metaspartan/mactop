package app

import (
	"log"
	"os"
	"sync"
	"time"

	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/net"
)

var (
	version                                                     = "v2.0.3"
	cpuGauge, gpuGauge, memoryGauge, aneGauge                   *w.Gauge
	mainBlock                                                   *ui.Block
	modelText, PowerChart, NetworkInfo, helpText, infoParagraph *w.Paragraph
	tbInfoParagraph                                             *w.Paragraph
	grid                                                        *ui.Grid
	processList                                                 *w.List
	sparkline, gpuSparkline                                     *w.Sparkline
	sparklineGroup, gpuSparklineGroup                           *w.SparklineGroup

	tbNetSparklineIn, tbNetSparklineOut *w.Sparkline
	tbNetSparklineGroup                 *w.SparklineGroup
	cpuCoreWidget                       *CPUCoreWidget
	powerValues                         = make([]float64, 35)
	tbNetInValues                       = make([]float64, 100)
	tbNetOutValues                      = make([]float64, 100)
	lastTBInBytes, lastTBOutBytes       float64
	lastUpdateTime                      time.Time
	stderrLogger                        = log.New(os.Stderr, "", 0)
	showHelp, partyMode                 = false, false
	updateInterval                      = 1000
	done                                = make(chan struct{})
	partyTicker                         *time.Ticker
	lastCPUTimes                        []CPUUsage
	firstRun                            = true
	sortReverse                         = false
	columns                             = []string{"PID", "USER", "VIRT", "RES", "CPU", "MEM", "TIME", "CMD"}
	selectedColumn                      = 4
	maxPowerSeen                        = 0.1
	gpuValues                           = make([]float64, 100)

	prometheusPort     string
	headless           bool
	headlessPretty     bool
	headlessCount      int
	interruptChan      = make(chan struct{}, 10)
	lastNetStats       net.IOCountersStat
	lastDiskStats      disk.IOCountersStat
	lastNetDiskTime    time.Time
	netDiskMutex       sync.Mutex
	killPending        bool
	killPID            int
	currentUser        string
	lastProcesses      []ProcessMetrics
	networkUnit        string
	diskUnit           string
	tempUnit           string
	currentLayoutNum   int
	totalLayouts       int
	currentColorName   string
	lastCPUMetrics     CPUMetrics
	lastGPUMetrics     GPUMetrics
	lastNetDiskMetrics NetDiskMetrics
	lastActiveLayout   string = "default"
	cpuMetricsChan            = make(chan CPUMetrics, 1)
	gpuMetricsChan            = make(chan GPUMetrics, 1)
	netdiskMetricsChan        = make(chan NetDiskMetrics, 1)
	tbNetStatsChan            = make(chan []ThunderboltNetStats, 1)
	processMetricsChan        = make(chan []ProcessMetrics, 1)
	ticker             *time.Ticker

	cachedHostname      string
	cachedCurrentUser   string
	cachedShell         string
	cachedKernelVersion string
	cachedOSVersion     string

	cachedModelName  string
	cachedSystemInfo SystemInfo
	tbDeviceInfo     string
	tbInfoMutex      sync.Mutex
	infoScrollOffset int
	currentBgIndex   int // Index for background color cycling
)

var (
	cpuUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_cpu_usage_percent",
			Help: "Current total CPU usage percentage",
		},
	)

	ecoreUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_ecore_usage_percent",
			Help: "Current E-core CPU usage percentage",
		},
	)

	pcoreUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_pcore_usage_percent",
			Help: "Current P-core CPU usage percentage",
		},
	)

	gpuUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_gpu_usage_percent",
			Help: "Current GPU usage percentage",
		},
	)

	gpuFreqMHz = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_gpu_freq_mhz",
			Help: "Current GPU frequency in MHz",
		},
	)
	powerUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_power_watts",
			Help: "Current power usage in watts",
		},
		[]string{"component"},
	)

	socTemp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_soc_temp_celsius",
			Help: "Current SoC temperature in Celsius",
		},
	)
	gpuTemp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mactop_gpu_temperature_celsius",
		Help: "Current GPU temperature in Celsius",
	})
	thermalState = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mactop_thermal_state",
		Help: "Current thermal state (0=Nominal, 1=Fair, 2=Serious, 3=Critical)",
	},
	)

	memoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_memory_gb",
			Help: "Memory usage in GB",
		},
		[]string{"type"},
	)

	networkSpeed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_network_kbytes_per_sec",
			Help: "Network speed in KB/s",
		},
		[]string{"direction"},
	)

	diskIOSpeed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_disk_kbytes_per_sec",
			Help: "Disk I/O speed in KB/s",
		},
		[]string{"operation"},
	)

	diskIOPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_disk_iops",
			Help: "Disk I/O operations per second",
		},
		[]string{"operation"},
	)

	// Thunderbolt network metrics
	tbNetworkSpeed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_thunderbolt_network_bytes_per_sec",
			Help: "Thunderbolt network throughput in bytes per second",
		},
		[]string{"direction"},
	)

	// RDMA status metric
	rdmaAvailable = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mactop_rdma_available",
			Help: "RDMA availability status (1=available, 0=unavailable)",
		},
	)

	// Per-core CPU usage metrics
	cpuCoreUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_cpu_core_usage_percent",
			Help: "Per-core CPU usage percentage",
		},
		[]string{"core", "type"},
	)

	// System info metrics (static labels)
	systemInfoGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mactop_system_info",
			Help: "System information (value is always 1, labels contain info)",
		},
		[]string{"model", "core_count", "e_core_count", "p_core_count", "gpu_core_count"},
	)
)
