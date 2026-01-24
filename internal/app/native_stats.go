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

// CoreType: 0 = unknown, 1 = E-core, 2 = P-core
typedef struct {
    int cpu_id;
    int core_type;  // 0=unknown, 1=E, 2=P
} core_info_t;

// Get core topology from IORegistry - works on all M-series chips
int get_core_topology(core_info_t *cores, int max_cores) {
    mach_port_t main_port = get_io_main_port();
    if (main_port == MACH_PORT_NULL) {
        return -1;
    }

    CFMutableDictionaryRef match = IOServiceMatching("IOPlatformDevice");
    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(main_port, match, &iter);
    if (kr != kIOReturnSuccess) {
        return -1;
    }

    int count = 0;
    io_registry_entry_t entry;

    while ((entry = IOIteratorNext(iter)) && count < max_cores) {
        CFMutableDictionaryRef properties = NULL;
        if (IORegistryEntryCreateCFProperties(entry, &properties, kCFAllocatorDefault, 0) == kIOReturnSuccess && properties) {
            // Check if this is a CPU device (name starts with "cpu")
            CFDataRef nameData = (CFDataRef)CFDictionaryGetValue(properties, CFSTR("name"));
            if (nameData && CFGetTypeID(nameData) == CFDataGetTypeID()) {
                const char *name = (const char *)CFDataGetBytePtr(nameData);
                if (name && strncmp(name, "cpu", 3) == 0) {
                    // Extract CPU ID from name (e.g., "cpu0" -> 0)
                    int cpu_id = atoi(name + 3);

                    // Get cluster-type property
                    CFDataRef clusterData = (CFDataRef)CFDictionaryGetValue(properties, CFSTR("cluster-type"));
                    if (clusterData && CFGetTypeID(clusterData) == CFDataGetTypeID()) {
                        const char *cluster_type = (const char *)CFDataGetBytePtr(clusterData);

                        cores[count].cpu_id = cpu_id;
                        if (cluster_type && cluster_type[0] == 'E') {
                            cores[count].core_type = 1; // E-core
                        } else if (cluster_type && cluster_type[0] == 'P') {
                            cores[count].core_type = 2; // P-core
                        } else {
                            cores[count].core_type = 0; // Unknown
                        }
                        count++;
                    }
                }
            }
            CFRelease(properties);
        }
        IOObjectRelease(entry);
    }
    IOObjectRelease(iter);

    return count;
}

// Per-process GPU statistics
typedef struct {
    int pid;
    uint64_t gpu_time_ns;  // accumulated GPU time in nanoseconds
} gpu_process_stat_t;

// Extract PID from "IOUserClientCreator" string like "pid 682, WindowServer"
static int extract_pid_from_creator(CFStringRef creator) {
    if (creator == NULL) return -1;

    char buf[256];
    if (!CFStringGetCString(creator, buf, sizeof(buf), kCFStringEncodingUTF8)) {
        return -1;
    }

    int pid = -1;
    if (sscanf(buf, "pid %d,", &pid) == 1) {
        return pid;
    }
    return -1;
}

// Sum all accumulatedGPUTime from AppUsage array
static uint64_t sum_gpu_time(CFArrayRef appUsage) {
    if (appUsage == NULL || CFGetTypeID(appUsage) != CFArrayGetTypeID()) {
        return 0;
    }

    uint64_t total = 0;
    CFIndex count = CFArrayGetCount(appUsage);

    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef entry = (CFDictionaryRef)CFArrayGetValueAtIndex(appUsage, i);
        if (entry == NULL || CFGetTypeID(entry) != CFDictionaryGetTypeID()) {
            continue;
        }

        CFNumberRef gpuTimeNum = (CFNumberRef)CFDictionaryGetValue(entry, CFSTR("accumulatedGPUTime"));
        if (gpuTimeNum != NULL && CFGetTypeID(gpuTimeNum) == CFNumberGetTypeID()) {
            int64_t gpuTime = 0;
            CFNumberGetValue(gpuTimeNum, kCFNumberSInt64Type, &gpuTime);
            if (gpuTime > 0) {
                total += (uint64_t)gpuTime;
            }
        }
    }

    return total;
}

// Query AGXDeviceUserClient for per-process GPU statistics
// AGXDeviceUserClient objects are children of AGXAccelerator, not standalone services
int get_gpu_process_stats(gpu_process_stat_t *stats, int max_stats) {
    // Find the AGXAccelerator service
    CFMutableDictionaryRef match = IOServiceMatching("AGXAccelerator");
    io_service_t accelerator = IOServiceGetMatchingService(kIOMainPortDefault, match);
    if (accelerator == 0) {
        return 0;
    }

    // Get child iterator to find AGXDeviceUserClient objects
    io_iterator_t childIter;
    kern_return_t kr = IORegistryEntryGetChildIterator(accelerator, kIOServicePlane, &childIter);
    if (kr != kIOReturnSuccess) {
        IOObjectRelease(accelerator);
        return 0;
    }

    int count = 0;
    io_registry_entry_t child;

    while ((child = IOIteratorNext(childIter)) && count < max_stats) {
        // Verify this is an AGXDeviceUserClient
        io_name_t className;
        IOObjectGetClass(child, className);
        if (strncmp(className, "AGXDeviceUserClient", 19) != 0) {
            IOObjectRelease(child);
            continue;
        }

        CFMutableDictionaryRef properties = NULL;
        if (IORegistryEntryCreateCFProperties(child, &properties, kCFAllocatorDefault, 0) == kIOReturnSuccess && properties) {
            // Get PID from IOUserClientCreator
            CFStringRef creator = (CFStringRef)CFDictionaryGetValue(properties, CFSTR("IOUserClientCreator"));
            int pid = extract_pid_from_creator(creator);

            if (pid > 0) {
                // Get GPU time from AppUsage
                CFArrayRef appUsage = (CFArrayRef)CFDictionaryGetValue(properties, CFSTR("AppUsage"));
                uint64_t gpuTime = sum_gpu_time(appUsage);

                // Check if we already have this PID (processes can have multiple GPU clients)
                int found = 0;
                for (int i = 0; i < count; i++) {
                    if (stats[i].pid == pid) {
                        stats[i].gpu_time_ns += gpuTime;
                        found = 1;
                        break;
                    }
                }

                if (!found && gpuTime > 0) {
                    stats[count].pid = pid;
                    stats[count].gpu_time_ns = gpuTime;
                    count++;
                }
            }
            CFRelease(properties);
        }
        IOObjectRelease(child);
    }
    IOObjectRelease(childIter);
    IOObjectRelease(accelerator);

    return count;
}

int get_gpu_core_count() {
    CFMutableDictionaryRef match = IOServiceMatching("AGXAccelerator");
    io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, match);
    if (service == 0) {
        return 0;
    }

    int core_count = 0;

    CFNumberRef coreCountRef = (CFNumberRef)IORegistryEntrySearchCFProperty(
        service,
        kIOServicePlane,
        CFSTR("gpu-core-count"),
        kCFAllocatorDefault,
        kIORegistryIterateRecursively
    );

    if (coreCountRef != NULL) {
        CFNumberGetValue(coreCountRef, kCFNumberIntType, &core_count);
        CFRelease(coreCountRef);
    }

    IOObjectRelease(service);
    return core_count;
}

typedef struct {
    uint64_t uid;
    uint64_t parent_uid;
    int router_id;
    int vendor_id;
    int device_id;
    char vendor_name[64];
    char device_name[128];
    int port_count;
    int depth;
    int thunderbolt_version;
    uint64_t link_speed;
} tb_switch_info_t;

static void get_cf_string(CFTypeRef ref, char *buf, size_t bufsize) {
    buf[0] = '\0';
    if (ref == NULL) return;
    if (CFGetTypeID(ref) == CFStringGetTypeID()) {
        CFStringGetCString((CFStringRef)ref, buf, bufsize, kCFStringEncodingUTF8);
    }
}

static int get_cf_int(CFTypeRef ref) {
    if (ref == NULL) return 0;
    if (CFGetTypeID(ref) == CFNumberGetTypeID()) {
        int val = 0;
        CFNumberGetValue((CFNumberRef)ref, kCFNumberIntType, &val);
        return val;
    }
    return 0;
}

static uint64_t get_cf_uint64(CFTypeRef ref) {
    if (ref == NULL) return 0;
    if (CFGetTypeID(ref) == CFNumberGetTypeID()) {
        uint64_t val = 0;
        CFNumberGetValue((CFNumberRef)ref, kCFNumberSInt64Type, &val);
        return val;
    }
    return 0;
}

// Helper to find the UID of the root switch (Bus) for a given device entry
static uint64_t find_root_switch_uid(io_registry_entry_t entry) {
    io_registry_entry_t current = entry;
    IOObjectRetain(current);

    uint64_t root_uid = 0;

    while (current) {
        io_registry_entry_t parent;
        kern_return_t kr = IORegistryEntryGetParentEntry(current, kIOServicePlane, &parent);

        IOObjectRelease(current); // Release current as we move up
        if (kr != kIOReturnSuccess) {
            break;
        }

        current = parent; // Parent is now current

        // Check if this parent is a Thunderbolt Switch with Depth == 0
        io_name_t className;
        IOObjectGetClass(current, className);

        // We are looking for the host controller (Depth 0)
        // Checking class name is a loose check, checking properties is better
        CFMutableDictionaryRef props = NULL;
        if (IORegistryEntryCreateCFProperties(current, &props, kCFAllocatorDefault, 0) == kIOReturnSuccess && props) {
            int depth = get_cf_int(CFDictionaryGetValue(props, CFSTR("Depth")));
            uint64_t uid = get_cf_uint64(CFDictionaryGetValue(props, CFSTR("UID")));

            // Check if it's a switch (has UID and Depth property)
            // Note: Depth 0 is the host
            if (CFDictionaryContainsKey(props, CFSTR("UID")) && depth == 0) {
                 root_uid = uid;
                 CFRelease(props);
                 IOObjectRelease(current); // Release final ref
                 return root_uid;
            }
            CFRelease(props);
        }
    }
    return 0;
}

int get_thunderbolt_switches(tb_switch_info_t *switches, int max_switches) {
    // Match generic IOThunderboltSwitch to get both Type5 (host) and Type3 (device)
    CFMutableDictionaryRef match = IOServiceMatching("IOThunderboltSwitch");
    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, match, &iter);
    if (kr != kIOReturnSuccess) {
        return 0;
    }

    int count = 0;
    io_registry_entry_t entry;
    while ((entry = IOIteratorNext(iter)) != 0 && count < max_switches) {
        CFMutableDictionaryRef props = NULL;
        if (IORegistryEntryCreateCFProperties(entry, &props, kCFAllocatorDefault, 0) == kIOReturnSuccess && props) {
            switches[count].uid = get_cf_uint64(CFDictionaryGetValue(props, CFSTR("UID")));
            switches[count].router_id = get_cf_int(CFDictionaryGetValue(props, CFSTR("Router ID")));
            switches[count].vendor_id = get_cf_int(CFDictionaryGetValue(props, CFSTR("Vendor ID")));
            switches[count].device_id = get_cf_int(CFDictionaryGetValue(props, CFSTR("Device ID")));
            switches[count].port_count = get_cf_int(CFDictionaryGetValue(props, CFSTR("Max Port Number")));
            switches[count].depth = get_cf_int(CFDictionaryGetValue(props, CFSTR("Depth")));
            switches[count].thunderbolt_version = get_cf_int(CFDictionaryGetValue(props, CFSTR("Thunderbolt Version")));

            uint64_t speed = get_cf_uint64(CFDictionaryGetValue(props, CFSTR("Link Speed")));
            if (speed == 0) speed = get_cf_uint64(CFDictionaryGetValue(props, CFSTR("Speed")));
            if (speed == 0) speed = get_cf_uint64(CFDictionaryGetValue(props, CFSTR("IOLinkSpeed")));
            switches[count].link_speed = speed;

            get_cf_string(CFDictionaryGetValue(props, CFSTR("Device Vendor Name")), switches[count].vendor_name, sizeof(switches[count].vendor_name));
            get_cf_string(CFDictionaryGetValue(props, CFSTR("Device Model Name")), switches[count].device_name, sizeof(switches[count].device_name));

            // If depth > 0, find parent UID
            if (switches[count].depth > 0) {
                switches[count].parent_uid = find_root_switch_uid(entry);
            } else {
                switches[count].parent_uid = 0;
            }

            count++;
            CFRelease(props);
        }
        IOObjectRelease(entry);
    }
    IOObjectRelease(iter);
    return count;
}

typedef struct {
    int vendor_id;
    int product_id;
    uint32_t location_id;
    char vendor_name[64];
    char product_name[128];
    char serial[64];
} usb_device_info_t;

int get_usb_devices(usb_device_info_t *devices, int max_devices) {
    CFMutableDictionaryRef match = IOServiceMatching("IOUSBHostDevice");
    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, match, &iter);
    if (kr != kIOReturnSuccess) {
        return 0;
    }

    int count = 0;
    io_registry_entry_t entry;
    while ((entry = IOIteratorNext(iter)) != 0 && count < max_devices) {
        CFMutableDictionaryRef props = NULL;
        if (IORegistryEntryCreateCFProperties(entry, &props, kCFAllocatorDefault, 0) == kIOReturnSuccess && props) {
            devices[count].vendor_id = get_cf_int(CFDictionaryGetValue(props, CFSTR("idVendor")));
            devices[count].product_id = get_cf_int(CFDictionaryGetValue(props, CFSTR("idProduct")));
            devices[count].location_id = (uint32_t)get_cf_int(CFDictionaryGetValue(props, CFSTR("locationID")));
            get_cf_string(CFDictionaryGetValue(props, CFSTR("USB Vendor Name")), devices[count].vendor_name, sizeof(devices[count].vendor_name));
            get_cf_string(CFDictionaryGetValue(props, CFSTR("USB Product Name")), devices[count].product_name, sizeof(devices[count].product_name));
            get_cf_string(CFDictionaryGetValue(props, CFSTR("USB Serial Number")), devices[count].serial, sizeof(devices[count].serial));
            count++;
            CFRelease(props);
        }
        IOObjectRelease(entry);
    }
    IOObjectRelease(iter);
    return count;
}

typedef struct {
    char name[128];
    char bsd_name[32];
    char protocol[32];
    char medium_type[32];
    int is_internal;
    int is_whole;
    uint64_t size_bytes;
} storage_device_info_t;

int get_storage_devices(storage_device_info_t *devices, int max_devices) {
    CFMutableDictionaryRef match = IOServiceMatching("IOMedia");
    io_iterator_t iter;
    kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, match, &iter);
    if (kr != kIOReturnSuccess) {
        return 0;
    }

    int count = 0;
    io_registry_entry_t entry;
    while ((entry = IOIteratorNext(iter)) != 0 && count < max_devices) {
        CFMutableDictionaryRef props = NULL;
        if (IORegistryEntryCreateCFProperties(entry, &props, kCFAllocatorDefault, 0) == kIOReturnSuccess && props) {
            CFBooleanRef wholeRef = CFDictionaryGetValue(props, CFSTR("Whole"));
            int is_whole = (wholeRef && CFBooleanGetValue(wholeRef)) ? 1 : 0;

            if (is_whole) {
                devices[count].is_whole = 1;
                devices[count].size_bytes = get_cf_uint64(CFDictionaryGetValue(props, CFSTR("Size")));
                get_cf_string(CFDictionaryGetValue(props, CFSTR("BSD Name")), devices[count].bsd_name, sizeof(devices[count].bsd_name));

                io_name_t name;
                IORegistryEntryGetName(entry, name);
                strncpy(devices[count].name, name, sizeof(devices[count].name) - 1);

                CFDictionaryRef protoDict = (CFDictionaryRef)CFDictionaryGetValue(props, CFSTR("Protocol Characteristics"));
                if (protoDict && CFGetTypeID(protoDict) == CFDictionaryGetTypeID()) {
                    CFStringRef locRef = (CFStringRef)CFDictionaryGetValue(protoDict, CFSTR("Physical Interconnect Location"));
                    if (locRef && CFGetTypeID(locRef) == CFStringGetTypeID()) {
                        char locBuf[32];
                        if (CFStringGetCString(locRef, locBuf, sizeof(locBuf), kCFStringEncodingUTF8)) {
                            if (strncmp(locBuf, "Internal", 8) == 0) {
                                devices[count].is_internal = 1;
                            }
                        }
                    }
                    CFStringRef protoRef = (CFStringRef)CFDictionaryGetValue(protoDict, CFSTR("Physical Interconnect"));
                    if (protoRef && CFGetTypeID(protoRef) == CFStringGetTypeID()) {
                        CFStringGetCString(protoRef, devices[count].protocol, sizeof(devices[count].protocol), kCFStringEncodingUTF8);
                    }
                }

                // Fallback: Check direct boolean properties "Internal" or "OSInternal"
                if (devices[count].is_internal == 0) {
                     CFBooleanRef internalRef = CFDictionaryGetValue(props, CFSTR("Internal"));
                     if (internalRef && CFBooleanGetValue(internalRef)) {
                         devices[count].is_internal = 1;
                     }
                }

                if (devices[count].is_internal == 0) {
                     CFBooleanRef osInternalRef = CFDictionaryGetValue(props, CFSTR("OSInternal"));
                     if (osInternalRef && CFBooleanGetValue(osInternalRef)) {
                         devices[count].is_internal = 1;
                     }
                }

                count++;
            }
            CFRelease(props);
        }
        IOObjectRelease(entry);
    }
    IOObjectRelease(iter);
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

// CoreType represents the type of CPU core
type CoreType int

const (
	CoreTypeUnknown CoreType = 0
	CoreTypeE       CoreType = 1 // Efficiency core
	CoreTypeP       CoreType = 2 // Performance core
)

// CoreTopologyEntry represents a single CPU core's topology information
type CoreTopologyEntry struct {
	CPUID    int
	CoreType CoreType
}

// GetCoreTopology returns the core topology detected from IORegistry.
// This is the authoritative source for E-core vs P-core identification
// and works across all M-series chips without hardcoding.
func GetCoreTopology() ([]CoreTopologyEntry, error) {
	maxCores := 128 // Support up to 128 cores (future-proofing)
	cores := make([]C.core_info_t, maxCores)

	count := C.get_core_topology(&cores[0], C.int(maxCores))
	if count < 0 {
		return nil, fmt.Errorf("failed to get core topology")
	}

	result := make([]CoreTopologyEntry, count)
	for i := 0; i < int(count); i++ {
		result[i] = CoreTopologyEntry{
			CPUID:    int(cores[i].cpu_id),
			CoreType: CoreType(cores[i].core_type),
		}
	}

	// Sort by CPU ID to ensure consistent ordering
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].CPUID > result[j].CPUID {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// BuildCoreLabels creates the correct E/P labels based on dynamic topology detection.
// Returns: labels (sorted E first, then P), eCount, pCount, and cpuIndexMap (maps display index -> Mach API index)
func BuildCoreLabels() ([]string, int, int, []int) {
	topology, err := GetCoreTopology()
	if err != nil || len(topology) == 0 {
		// Fallback to sysctl-based counts (old behavior)
		return nil, 0, 0, nil
	}

	// topology is already sorted by CPUID (done in GetCoreTopology)
	// The position in this sorted array corresponds to the Mach API index (0, 1, 2, ...)
	// We need to separate E-cores and P-cores while preserving their Mach API indices

	// Track the Mach API index (position in sorted topology) for each core
	type coreWithMachIndex struct {
		machIndex int
		coreType  CoreType
	}

	var eCores []coreWithMachIndex
	var pCores []coreWithMachIndex

	for machIdx, entry := range topology {
		switch entry.CoreType {
		case CoreTypeE:
			eCores = append(eCores, coreWithMachIndex{machIdx, entry.CoreType})
		case CoreTypeP:
			pCores = append(pCores, coreWithMachIndex{machIdx, entry.CoreType})
		}
	}

	// Build sorted list: E-cores first, then P-cores
	// cpuIndexMap maps display index -> Mach API index
	totalCores := len(eCores) + len(pCores)
	labels := make([]string, totalCores)
	cpuIndexMap := make([]int, totalCores)

	idx := 0
	for i, core := range eCores {
		labels[idx] = fmt.Sprintf("E%d", i)
		cpuIndexMap[idx] = core.machIndex
		idx++
	}
	for i, core := range pCores {
		labels[idx] = fmt.Sprintf("P%d", i)
		cpuIndexMap[idx] = core.machIndex
		idx++
	}

	return labels, len(eCores), len(pCores), cpuIndexMap
}

// GPUProcessStat represents per-process GPU usage
type GPUProcessStat struct {
	PID       int
	GPUTimeNs uint64 // accumulated GPU time in nanoseconds
}

// GetGPUProcessStats returns per-process GPU statistics from IOKit AGXDeviceUserClient
func GetGPUProcessStats() map[int]uint64 {
	maxStats := 256
	stats := make([]C.gpu_process_stat_t, maxStats)

	count := C.get_gpu_process_stats(&stats[0], C.int(maxStats))
	if count <= 0 {
		return nil
	}

	result := make(map[int]uint64)
	for i := 0; i < int(count); i++ {
		pid := int(stats[i].pid)
		gpuTime := uint64(stats[i].gpu_time_ns)
		result[pid] = gpuTime
	}

	return result
}

func GetGPUCoreCountFast() int {
	return int(C.get_gpu_core_count())
}

type ThunderboltSwitchInfo struct {
	UID                uint64
	ParentUID          uint64
	RouterID           int
	VendorID           int
	DeviceID           int
	VendorName         string
	DeviceName         string
	PortCount          int
	Depth              int
	ThunderboltVersion int
	LinkSpeed          uint64
}

func GetThunderboltSwitchesIOKit() []ThunderboltSwitchInfo {
	maxSwitches := 32
	switches := make([]C.tb_switch_info_t, maxSwitches)
	count := C.get_thunderbolt_switches(&switches[0], C.int(maxSwitches))
	if count <= 0 {
		return nil
	}

	result := make([]ThunderboltSwitchInfo, int(count))
	for i := 0; i < int(count); i++ {
		result[i] = ThunderboltSwitchInfo{
			UID:                uint64(switches[i].uid),
			ParentUID:          uint64(switches[i].parent_uid),
			RouterID:           int(switches[i].router_id),
			VendorID:           int(switches[i].vendor_id),
			DeviceID:           int(switches[i].device_id),
			VendorName:         C.GoString(&switches[i].vendor_name[0]),
			DeviceName:         C.GoString(&switches[i].device_name[0]),
			PortCount:          int(switches[i].port_count),
			Depth:              int(switches[i].depth),
			ThunderboltVersion: int(switches[i].thunderbolt_version),
			LinkSpeed:          uint64(switches[i].link_speed),
		}
	}
	return result
}

type USBDeviceInfo struct {
	VendorID    int
	ProductID   int
	LocationID  uint32
	VendorName  string
	ProductName string
	Serial      string
}

func GetUSBDevicesIOKit() []USBDeviceInfo {
	maxDevices := 64
	devices := make([]C.usb_device_info_t, maxDevices)
	count := C.get_usb_devices(&devices[0], C.int(maxDevices))
	if count <= 0 {
		return nil
	}

	result := make([]USBDeviceInfo, int(count))
	for i := 0; i < int(count); i++ {
		result[i] = USBDeviceInfo{
			VendorID:    int(devices[i].vendor_id),
			ProductID:   int(devices[i].product_id),
			LocationID:  uint32(devices[i].location_id),
			VendorName:  C.GoString(&devices[i].vendor_name[0]),
			ProductName: C.GoString(&devices[i].product_name[0]),
			Serial:      C.GoString(&devices[i].serial[0]),
		}
	}
	return result
}

type StorageDeviceInfo struct {
	Name       string
	BSDName    string
	Protocol   string
	MediumType string
	IsInternal bool
	IsWhole    bool
	SizeBytes  uint64
}

func GetStorageDevicesIOKit() []StorageDeviceInfo {
	maxDevices := 32
	devices := make([]C.storage_device_info_t, maxDevices)
	count := C.get_storage_devices(&devices[0], C.int(maxDevices))
	if count <= 0 {
		return nil
	}

	result := make([]StorageDeviceInfo, int(count))
	for i := 0; i < int(count); i++ {
		result[i] = StorageDeviceInfo{
			Name:       C.GoString(&devices[i].name[0]),
			BSDName:    C.GoString(&devices[i].bsd_name[0]),
			Protocol:   C.GoString(&devices[i].protocol[0]),
			MediumType: C.GoString(&devices[i].medium_type[0]),
			IsInternal: devices[i].is_internal != 0,
			IsWhole:    devices[i].is_whole != 0,
			SizeBytes:  uint64(devices[i].size_bytes),
		}
	}
	return result
}
