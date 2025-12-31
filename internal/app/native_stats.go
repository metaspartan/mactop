package app

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit
#include <sys/sysctl.h>
#include <sys/mount.h>
#include <sys/param.h>
#include <mach/mach_host.h>
#include <mach/mach_init.h>
#include <mach/mach_error.h>
#include <mach/vm_map.h>
#include <stdlib.h>
#include <time.h>
#include <ifaddrs.h>
#include <net/if.h>
#include <net/if_dl.h>
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// Wrapper for host_statistics64
kern_return_t get_vm_statistics(vm_statistics64_data_t *vm_stat) {
    mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
    return host_statistics64(mach_host_self(), HOST_VM_INFO64, (host_info64_t)vm_stat, &count);
}

typedef struct {
    char name[64];
    uint64_t read_bytes;
    uint64_t write_bytes;
    uint64_t read_ops;
    uint64_t write_ops;
    uint64_t read_time;
    uint64_t write_time;
} disk_stat_t;

static inline mach_port_t get_io_main_port(void) {
    mach_port_t port = MACH_PORT_NULL;
    #if __MAC_OS_X_VERSION_MIN_REQUIRED >= 120000
    IOMainPort(MACH_PORT_NULL, &port);
    #else
    #pragma clang diagnostic push
    #pragma clang diagnostic ignored "-Wdeprecated-declarations"
    IOMasterPort(MACH_PORT_NULL, &port);
    #pragma clang diagnostic pop
    #endif
    return port;
}

// Get 64-bit value from CFNumber safely
static inline uint64_t get_cf_number_value(CFDictionaryRef dict, CFStringRef key) {
    CFNumberRef num = NULL;
    uint64_t value = 0;
    if (CFDictionaryGetValueIfPresent(dict, key, (const void**)&num) && num) {
        CFNumberGetValue(num, kCFNumberSInt64Type, &value);
    }
    return value;
}

int get_disk_stats(disk_stat_t *stats, int max_stats) {
    mach_port_t main_port = get_io_main_port();
    if (main_port == MACH_PORT_NULL) {
        return -1;
    }

    // Query AppleAPFSVolume - this is where actual I/O statistics live on Apple Silicon
    CFMutableDictionaryRef match = IOServiceMatching("AppleAPFSVolume");
    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(main_port, match, &iter);

    if (kr != kIOReturnSuccess) {
        // Fallback to IOBlockStorageDriver for older systems
        match = IOServiceMatching("IOBlockStorageDriver");
        kr = IOServiceGetMatchingServices(main_port, match, &iter);
        if (kr != kIOReturnSuccess) {
            return -1;
        }
    }

    int count = 0;
    io_registry_entry_t entry;

    // We aggregate all volumes into a single stat entry for simplicity
    // The first entry will hold the totals
    memset(&stats[0], 0, sizeof(disk_stat_t));
    snprintf(stats[0].name, 64, "all");

    while ((entry = IOIteratorNext(iter))) {
        CFMutableDictionaryRef properties = NULL;
        if (IORegistryEntryCreateCFProperties(entry, &properties, kCFAllocatorDefault, 0) == kIOReturnSuccess && properties) {
            CFDictionaryRef stats_dict = (CFDictionaryRef)CFDictionaryGetValue(properties, CFSTR("Statistics"));
            if (stats_dict && CFGetTypeID(stats_dict) == CFDictionaryGetTypeID()) {
                // APFS uses different key names than traditional block storage
                // Try APFS keys first, then fallback to traditional keys
                uint64_t read_bytes = get_cf_number_value(stats_dict, CFSTR("Bytes read from block device"));
                if (read_bytes == 0) {
                    read_bytes = get_cf_number_value(stats_dict, CFSTR("Bytes (Read)"));
                }

                uint64_t write_bytes = get_cf_number_value(stats_dict, CFSTR("Bytes written to block device"));
                if (write_bytes == 0) {
                    write_bytes = get_cf_number_value(stats_dict, CFSTR("Bytes (Write)"));
                }

                uint64_t read_ops = get_cf_number_value(stats_dict, CFSTR("Read requests sent to block device"));
                if (read_ops == 0) {
                    read_ops = get_cf_number_value(stats_dict, CFSTR("Operations (Read)"));
                }

                uint64_t write_ops = get_cf_number_value(stats_dict, CFSTR("Write requests sent to block device"));
                if (write_ops == 0) {
                    write_ops = get_cf_number_value(stats_dict, CFSTR("Operations (Write)"));
                }

                // Aggregate into the first entry
                stats[0].read_bytes += read_bytes;
                stats[0].write_bytes += write_bytes;
                stats[0].read_ops += read_ops;
                stats[0].write_ops += write_ops;

                // Time stats (may not be available)
                stats[0].read_time += get_cf_number_value(stats_dict, CFSTR("Total Time (Read)"));
                stats[0].write_time += get_cf_number_value(stats_dict, CFSTR("Total Time (Write)"));
            }
            CFRelease(properties);
        }
        IOObjectRelease(entry);
    }
    IOObjectRelease(iter);

    // Return 1 if we found any stats
    if (stats[0].read_bytes > 0 || stats[0].write_bytes > 0 ||
        stats[0].read_ops > 0 || stats[0].write_ops > 0) {
        count = 1;
    }

    return count;
}
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type NativeMemoryMetrics struct {
	Total     uint64
	Used      uint64
	Available uint64
	SwapTotal uint64
	SwapUsed  uint64
}

var (
	pageSize    uint64
	totalMemory uint64
)

func initNativeStats() error {
	// Get page size
	var size C.size_t = C.sizeof_int
	var pSize C.int
	namePage := C.CString("hw.pagesize")
	defer C.free(unsafe.Pointer(namePage))
	if C.sysctlbyname(namePage, unsafe.Pointer(&pSize), &size, nil, 0) != 0 {
		return fmt.Errorf("failed to get page size")
	}
	pageSize = uint64(pSize)

	// Get total memory
	var mSize C.uint64_t
	size = C.sizeof_uint64_t
	nameMem := C.CString("hw.memsize")
	defer C.free(unsafe.Pointer(nameMem))
	if C.sysctlbyname(nameMem, unsafe.Pointer(&mSize), &size, nil, 0) != 0 {
		return fmt.Errorf("failed to get memsize")
	}
	totalMemory = uint64(mSize)
	return nil
}

func GetNativeMemoryMetrics() (NativeMemoryMetrics, error) {
	if totalMemory == 0 {
		if err := initNativeStats(); err != nil {
			return NativeMemoryMetrics{}, err
		}
	}

	var vmStat C.vm_statistics64_data_t
	if ret := C.get_vm_statistics(&vmStat); ret != C.KERN_SUCCESS {
		return NativeMemoryMetrics{}, fmt.Errorf("failed to get vm statistics: %d", ret)
	}

	free := uint64(vmStat.free_count) * pageSize
	// active := uint64(vmStat.active_count) * pageSize
	inactive := uint64(vmStat.inactive_count) * pageSize
	// wired := uint64(vmStat.wire_count) * pageSize
	// compressed := uint64(vmStat.compressor_page_count) * pageSize

	available := free + inactive
	used := totalMemory - available

	// Swap
	var xsw C.struct_xsw_usage
	size := C.size_t(C.sizeof_struct_xsw_usage)
	nameSwap := C.CString("vm.swapusage")
	defer C.free(unsafe.Pointer(nameSwap))
	if C.sysctlbyname(nameSwap, unsafe.Pointer(&xsw), &size, nil, 0) != 0 {
		// Swap might be disabled or failed, just return 0s
		return NativeMemoryMetrics{
			Total:     totalMemory,
			Used:      used,
			Available: available,
			SwapTotal: 0,
			SwapUsed:  0,
		}, nil
	}

	return NativeMemoryMetrics{
		Total:     totalMemory,
		Used:      used,
		Available: available,
		SwapTotal: uint64(xsw.xsu_total),
		SwapUsed:  uint64(xsw.xsu_used),
	}, nil
}

// NativeDiskUsage represents filesystem usage
type NativeDiskUsage struct {
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// NativePartitionInfo represents a mounted partition
type NativePartitionInfo struct {
	Device     string
	Mountpoint string
	Fstype     string
}

// GetNativeUptime returns the system uptime in seconds
func GetNativeUptime() (uint64, error) {
	var boottime C.struct_timeval
	size := C.size_t(C.sizeof_struct_timeval)
	name := C.CString("kern.boottime")
	defer C.free(unsafe.Pointer(name))

	if C.sysctlbyname(name, unsafe.Pointer(&boottime), &size, nil, 0) != 0 {
		return 0, fmt.Errorf("failed to get boottime")
	}

	var now C.struct_timeval
	C.gettimeofday(&now, nil)

	return uint64(now.tv_sec - boottime.tv_sec), nil
}

// GetNativePartitions returns a list of mounted partitions
func GetNativePartitions(all bool) ([]NativePartitionInfo, error) {
	var mntbuf *C.struct_statfs
	// getmntinfo returns the number of mounted filesystems
	// MNT_NOWAIT = 2
	count := C.getmntinfo(&mntbuf, 2)
	if count == 0 {
		return nil, fmt.Errorf("getmntinfo failed")
	}

	// Convert C array to Go slice
	entries := (*[1 << 30]C.struct_statfs)(unsafe.Pointer(mntbuf))[:count:count]

	var partitions []NativePartitionInfo
	for _, entry := range entries {
		mountPoint := C.GoString(&entry.f_mntonname[0])
		device := C.GoString(&entry.f_mntfromname[0])
		fstype := C.GoString(&entry.f_fstypename[0])

		partitions = append(partitions, NativePartitionInfo{
			Device:     device,
			Mountpoint: mountPoint,
			Fstype:     fstype,
		})
	}

	return partitions, nil
}

// GetNativeDiskUsage returns usage stats for a specific path
func GetNativeDiskUsage(path string) (NativeDiskUsage, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var buf C.struct_statfs
	if C.statfs(cPath, &buf) != 0 {
		return NativeDiskUsage{}, fmt.Errorf("statfs failed")
	}

	total := uint64(buf.f_blocks) * uint64(buf.f_bsize)
	free := uint64(buf.f_bfree) * uint64(buf.f_bsize)
	avail := uint64(buf.f_bavail) * uint64(buf.f_bsize)
	used := total - free

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100.0
	}

	return NativeDiskUsage{
		Total:       total,
		Used:        used,
		Free:        avail, // Usually 'Free' in APIs means Available to user
		UsedPercent: usedPercent,
	}, nil
}

// NativeNetMetric represents network interface statistics
type NativeNetMetric struct {
	Name        string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

// GetNativeNetworkMetrics returns network statistics for all interfaces
func GetNativeNetworkMetrics() (map[string]NativeNetMetric, error) {
	var ifap *C.struct_ifaddrs
	if C.getifaddrs(&ifap) != 0 {
		return nil, fmt.Errorf("getifaddrs failed")
	}
	defer C.freeifaddrs(ifap)

	metrics := make(map[string]NativeNetMetric)

	for ifa := ifap; ifa != nil; ifa = ifa.ifa_next {
		if ifa.ifa_addr == nil || ifa.ifa_addr.sa_family != C.AF_LINK {
			continue
		}

		data := (*C.struct_if_data)(unsafe.Pointer(ifa.ifa_data))
		if data == nil {
			continue
		}

		name := C.GoString(ifa.ifa_name)

		m := NativeNetMetric{
			Name:        name,
			BytesSent:   uint64(data.ifi_obytes),
			BytesRecv:   uint64(data.ifi_ibytes),
			PacketsSent: uint64(data.ifi_opackets),
			PacketsRecv: uint64(data.ifi_ipackets),
		}

		if existing, ok := metrics[name]; ok {
			existing.BytesSent += m.BytesSent
			existing.BytesRecv += m.BytesRecv
			existing.PacketsSent += m.PacketsSent
			existing.PacketsRecv += m.PacketsRecv
			metrics[name] = existing
		} else {
			metrics[name] = m
		}
	}
	return metrics, nil
}

// NativeDiskMetric represents disk I/O statistics
type NativeDiskMetric struct {
	Name       string
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
	ReadTime   uint64
	WriteTime  uint64
}

// GetNativeDiskMetrics returns disk I/O statistics
func GetNativeDiskMetrics() (map[string]NativeDiskMetric, error) {
	maxStats := 32 // Reasonable limit for internal disks
	stats := make([]C.disk_stat_t, maxStats)

	count := C.get_disk_stats(&stats[0], C.int(maxStats))
	if count < 0 {
		return nil, fmt.Errorf("failed to get disk stats")
	}

	result := make(map[string]NativeDiskMetric)
	for i := 0; i < int(count); i++ {
		name := C.GoString(&stats[i].name[0])
		if name == "" {
			continue // Should have name
		}

		result[name] = NativeDiskMetric{
			Name:       name,
			ReadBytes:  uint64(stats[i].read_bytes),
			WriteBytes: uint64(stats[i].write_bytes),
			ReadOps:    uint64(stats[i].read_ops),
			WriteOps:   uint64(stats[i].write_ops),
			ReadTime:   uint64(stats[i].read_time),
			WriteTime:  uint64(stats[i].write_time),
		}
	}

	return result, nil
}

// NativeHostInfo represents host information
type NativeHostInfo struct {
	Hostname      string
	OSVersion     string
	KernelVersion string
	Uptime        uint64
	BootTime      uint64
}

func getSysctlString(name string) (string, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	// Get size first
	var size C.size_t
	if C.sysctlbyname(cName, nil, &size, nil, 0) != 0 {
		return "", fmt.Errorf("failed to get size for %s", name)
	}

	buf := C.malloc(size)
	defer C.free(buf)

	if C.sysctlbyname(cName, buf, &size, nil, 0) != 0 {
		return "", fmt.Errorf("failed to get value for %s", name)
	}

	return C.GoString((*C.char)(buf)), nil
}

// GetNativeHostInfo returns host information
func GetNativeHostInfo() (NativeHostInfo, error) {
	hostname, _ := getSysctlString("kern.hostname")
	osVersion, _ := getSysctlString("kern.osproductversion") // macOS 10.13+
	kernelVersion, _ := getSysctlString("kern.osrelease")

	uptime, _ := GetNativeUptime()

	// BootTime = Now - Uptime
	bootTime := uint64(time.Now().Unix()) - uptime

	return NativeHostInfo{
		Hostname:      hostname,
		OSVersion:     osVersion,
		KernelVersion: kernelVersion,
		Uptime:        uptime,
		BootTime:      bootTime,
	}, nil
}
