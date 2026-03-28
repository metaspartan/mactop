// Copyright (c) 2024-2026 Carsen Klock under MIT License
// ioreport.m - Objective-C implementation for IOReport power/thermal metrics

#include "smc.h"
#import <CoreFoundation/CoreFoundation.h>
#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>
#import <IOKit/IOKitLib.h>
#import <IOKit/IOCFPlugIn.h>
#include <mach/mach_host.h>
#include <mach/mach_init.h>
#include <mach/processor_info.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/sysctl.h>
#include <dlfcn.h>
#include <unistd.h>
#include <pthread.h>

// Wi-Fi link info structure
typedef struct {
  char interface_name[32];
  char phy_mode[32];        // "802.11n", "802.11ac", "802.11ax", "802.11be"
  char wifi_generation[16]; // "Wi-Fi 4", "Wi-Fi 5", "Wi-Fi 6", "Wi-Fi 7"
  int tx_rate_mbps;         // Current transmit rate in Mbps
  int is_connected;         // 1 if associated to network
} wifi_link_info_t;

// Get Wi-Fi link information using CoreWLAN
int get_wifi_link_info(wifi_link_info_t *info) {
  @autoreleasepool {
    memset(info, 0, sizeof(wifi_link_info_t));

    CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
    if (!client)
      return -1;

    CWInterface *iface = [client interface];
    if (!iface)
      return -1;

    // Get interface name
    NSString *ifName = [iface interfaceName];
    if (ifName) {
      strncpy(info->interface_name, [ifName UTF8String],
              sizeof(info->interface_name) - 1);
    }

    // Get transmit rate
    info->tx_rate_mbps = (int)[iface transmitRate];

    // Check if connected — use serviceActive instead of ssid
    info->is_connected = [iface serviceActive] ? 1 : 0;

    // Map PHY mode to string and Wi-Fi generation
    CWPHYMode mode = [iface activePHYMode];
    switch (mode) {
    case kCWPHYModeNone:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "None");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "");
      break;
    case kCWPHYMode11a:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11a");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 2");
      break;
    case kCWPHYMode11b:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11b");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 1");
      break;
    case kCWPHYMode11g:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11g");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 3");
      break;
    case kCWPHYMode11n:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11n");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 4");
      break;
    case kCWPHYMode11ac:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11ac");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 5");
      break;
    case kCWPHYMode11ax:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11ax");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 6");
      break;
#ifdef kCWPHYMode11be
    case kCWPHYMode11be:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11be");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 7");
      break;
#else
    case 7: // kCWPHYMode11be not yet in SDK enum
      snprintf(info->phy_mode, sizeof(info->phy_mode), "802.11be");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "Wi-Fi 7");
      break;
#endif
    default:
      snprintf(info->phy_mode, sizeof(info->phy_mode), "Unknown");
      snprintf(info->wifi_generation, sizeof(info->wifi_generation), "");
      break;
    }

    return 0;
  }
}

typedef struct IOReportSubscriptionRef *IOReportSubscriptionRef;

extern CFDictionaryRef IOReportCopyChannelsInGroup(CFStringRef group,
                                                   CFStringRef subgroup,
                                                   uint64_t a, uint64_t b,
                                                   uint64_t c);
extern void IOReportMergeChannels(CFDictionaryRef a, CFDictionaryRef b,
                                  CFTypeRef unused);
extern IOReportSubscriptionRef
IOReportCreateSubscription(void *a, CFMutableDictionaryRef channels,
                           CFMutableDictionaryRef *out, uint64_t d,
                           CFTypeRef e);
extern CFDictionaryRef IOReportCreateSamples(IOReportSubscriptionRef sub,
                                             CFMutableDictionaryRef channels,
                                             CFTypeRef unused);
extern CFDictionaryRef IOReportCreateSamplesDelta(CFDictionaryRef a,
                                                  CFDictionaryRef b,
                                                  CFTypeRef unused);
extern int64_t IOReportSimpleGetIntegerValue(CFDictionaryRef item, int32_t idx);
extern CFStringRef IOReportChannelGetGroup(CFDictionaryRef item);
extern CFStringRef IOReportChannelGetSubGroup(CFDictionaryRef item);
extern CFStringRef IOReportChannelGetChannelName(CFDictionaryRef item);
extern CFStringRef IOReportChannelGetUnitLabel(CFDictionaryRef item);
extern int32_t IOReportStateGetCount(CFDictionaryRef item);
extern CFStringRef IOReportStateGetNameForIndex(CFDictionaryRef item,
                                                int32_t idx);
extern int64_t IOReportStateGetResidency(CFDictionaryRef item, int32_t idx);

typedef void *IOHIDEventSystemClientRef;
typedef void *IOHIDServiceClientRef;
typedef void *IOHIDEventRef;

extern IOHIDEventSystemClientRef
IOHIDEventSystemClientCreate(CFAllocatorRef allocator);
extern int IOHIDEventSystemClientSetMatching(IOHIDEventSystemClientRef client,
                                             CFDictionaryRef matching);
extern CFArrayRef
IOHIDEventSystemClientCopyServices(IOHIDEventSystemClientRef client);
extern CFStringRef IOHIDServiceClientCopyProperty(IOHIDServiceClientRef service,
                                                  CFStringRef key);
extern IOHIDEventRef IOHIDServiceClientCopyEvent(IOHIDServiceClientRef service,
                                                 int64_t type, int32_t options,
                                                 int64_t timeout);
extern double IOHIDEventGetFloatValue(IOHIDEventRef event, int64_t field);

#define kHIDPage_AppleVendor 0xff00
#define kHIDUsage_AppleVendor_TemperatureSensor 0x0005
#define kIOHIDEventTypeTemperature 15

// Fan info structure
typedef struct {
  char name[32];
  int actualRPM;
  int minRPM;
  int maxRPM;
  int targetRPM;
  int mode; // 0=auto, 1=forced
  int id;
} fan_info_t;

// Temperature sensor structure
typedef struct {
  char key[5];
  char name[64];
  float value;
} temp_sensor_t;

static IOReportSubscriptionRef g_subscription = NULL;
static CFMutableDictionaryRef g_channels = NULL;
static io_connect_t g_smcConn = 0;
static uint32_t g_gpu_freqs[64];
static int g_gpu_freq_count = 0;
static uint32_t g_ecpu_freqs[64];
static int g_ecpu_freq_count = 0;
static uint32_t g_pcpu_freqs[64];
static int g_pcpu_freq_count = 0;
static uint32_t g_scpu_freqs[64];
static int g_scpu_freq_count = 0;

// All discovered temperature sensors
static temp_sensor_t g_all_temp_sensors[512];
static int g_all_temp_sensor_count = 0;

// Expected physical core counts (set from Go via setExpectedCoreCounts)
static int g_expected_ecores = 0;
static int g_expected_pcores = 0;
static int g_expected_scores = 0;

void setExpectedCoreCounts(int eCores, int pCores, int sCores) {
  g_expected_ecores = eCores;
  g_expected_pcores = pCores;
  g_expected_scores = sCores;
}

// Cached IOHIDEventSystemClient — creating one is expensive (~2-5ms IPC to hidd).
// Reuse across ticks instead of create+destroy each time.
static IOHIDEventSystemClientRef g_hidClient = NULL;
static CFDictionaryRef g_hidMatching = NULL;

static IOHIDEventSystemClientRef getHIDClient(void) {
  if (g_hidClient == NULL) {
    g_hidClient = IOHIDEventSystemClientCreate(kCFAllocatorDefault);
    if (g_hidClient != NULL && g_hidMatching == NULL) {
      const void *keys[2] = {CFSTR("PrimaryUsagePage"), CFSTR("PrimaryUsage")};
      int page = kHIDPage_AppleVendor;
      int usage = kHIDUsage_AppleVendor_TemperatureSensor;
      CFNumberRef pageNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &page);
      CFNumberRef usageNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &usage);
      const void *values[2] = {pageNum, usageNum};
      g_hidMatching = CFDictionaryCreate(
          kCFAllocatorDefault, keys, values, 2,
          &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
      CFRelease(pageNum);
      CFRelease(usageNum);
    }
    if (g_hidClient != NULL && g_hidMatching != NULL) {
      IOHIDEventSystemClientSetMatching(g_hidClient, g_hidMatching);
    }
  }
  return g_hidClient;
}

static int cfStringStartsWith(CFStringRef str, const char *prefix);
static void loadSMCTempKeys();
static void loadAllTempSensors();

static void parseFreqData(CFDataRef data, uint32_t *outFreqs, int *outCount) {
  if (data == NULL)
    return;
  CFIndex len = CFDataGetLength(data);
  const uint8_t *bytes = CFDataGetBytePtr(data);
  int totalEntries = (int)(len / 8);

  *outCount = 0;
  for (int i = 0; i < totalEntries; i++) {
    uint32_t freq = 0;
    memcpy(&freq, bytes + (i * 8), 4);
    uint32_t freqMHz;
    if (freq >= 100000000) {
      // Hz format (M1-M4): e.g. 4,000,000,000 -> 4000 MHz
      freqMHz = freq / 1000000;
    } else if (freq >= 100000) {
      // kHz format (M5+): e.g. 4,608,000 -> 4608 MHz
      freqMHz = freq / 1000;
    } else {
      freqMHz = 0;
    }
    if (freqMHz > 0 && *outCount < 64) {
      outFreqs[(*outCount)++] = freqMHz;
    }
  }
}

static void loadCpuFrequencies() {
  if (g_ecpu_freq_count > 0 && g_pcpu_freq_count > 0)
    return;

  io_iterator_t iterator;
  io_object_t entry;

  CFMutableDictionaryRef matching = IOServiceMatching("AppleARMIODevice");
  if (IOServiceGetMatchingServices(kIOMainPortDefault, matching, &iterator) !=
      kIOReturnSuccess)
    return;

  while ((entry = IOIteratorNext(iterator)) != 0) {
    io_name_t name;
    IORegistryEntryGetName(entry, name);

    if (strcmp(name, "pmgr") == 0) {
      CFMutableDictionaryRef properties = NULL;
      if (IORegistryEntryCreateCFProperties(
              entry, &properties, kCFAllocatorDefault, 0) == kIOReturnSuccess) {

        // E-Cluster: voltage-states1-sram (M1-M4), voltage-states9-sram (M5+)
        if (g_ecpu_freq_count == 0) {
          CFDataRef eData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states1-sram"));
          if (eData != NULL) {
            parseFreqData(eData, g_ecpu_freqs, &g_ecpu_freq_count);
          }
        }
        if (g_ecpu_freq_count == 0) {
          CFDataRef eData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states9-sram"));
          if (eData != NULL) {
            parseFreqData(eData, g_ecpu_freqs, &g_ecpu_freq_count);
          }
        }

        // P-Cluster / S-Cluster (Super cores, M5+): voltage-states5-sram
        if (g_pcpu_freq_count == 0) {
          CFDataRef pData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states5-sram"));
          if (pData != NULL) {
            parseFreqData(pData, g_pcpu_freqs, &g_pcpu_freq_count);
          }
        }

        // M-Cluster (M5+): voltage-states22-sram or voltage-states23-sram
        if (g_scpu_freq_count == 0) {
          CFDataRef mData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states22-sram"));
          if (mData != NULL) {
            parseFreqData(mData, g_scpu_freqs, &g_scpu_freq_count);
          }
        }
        if (g_scpu_freq_count == 0) {
          CFDataRef mData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states23-sram"));
          if (mData != NULL) {
            parseFreqData(mData, g_scpu_freqs, &g_scpu_freq_count);
          }
        }
        // Fallback for S-Cluster: voltage-states3-sram (if 22/23 not found)
        if (g_scpu_freq_count == 0) {
          CFDataRef sData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states3-sram"));
          if (sData != NULL) {
            parseFreqData(sData, g_scpu_freqs, &g_scpu_freq_count);
          }
        }

        CFRelease(properties);
      }
    }
    IOObjectRelease(entry);
    if (g_ecpu_freq_count > 0 && g_pcpu_freq_count > 0)
      break;
  }
  IOObjectRelease(iterator);
}

static void loadGpuFrequencies() {
  if (g_gpu_freq_count > 0)
    return;

  io_iterator_t iterator;
  io_object_t entry;

  CFMutableDictionaryRef matching = IOServiceMatching("AppleARMIODevice");
  if (IOServiceGetMatchingServices(kIOMainPortDefault, matching, &iterator) !=
      kIOReturnSuccess)
    return;

  while ((entry = IOIteratorNext(iterator)) != 0) {
    io_name_t name;
    IORegistryEntryGetName(entry, name);

    if (strcmp(name, "pmgr") == 0 || strcmp(name, "clpc") == 0) {
      CFMutableDictionaryRef properties = NULL;
      if (IORegistryEntryCreateCFProperties(
              entry, &properties, kCFAllocatorDefault, 0) == kIOReturnSuccess) {

        CFIndex count = CFDictionaryGetCount(properties);
        const void *keys[count];
        const void *values[count];
        CFDictionaryGetKeysAndValues(properties, keys, values);

        CFDataRef bestData = NULL;
        uint32_t bestMaxFreq = 0xFFFFFFFF;
        int bestValidFreqs = 0;

        for (CFIndex i = 0; i < count; i++) {
          CFStringRef key = (CFStringRef)keys[i];
          char keyName[512];
          CFStringGetCString(key, keyName, sizeof(keyName),
                             kCFStringEncodingUTF8);

          if (strcmp(keyName, "voltage-states9-sram") == 0 ||
              strcmp(keyName, "voltage-states9") == 0) {
            bestData = (CFDataRef)values[i];
            break;
          }
        }

        if (bestData == NULL) {
          for (CFIndex i = 0; i < count; i++) {
            CFStringRef key = (CFStringRef)keys[i];
            if (cfStringStartsWith(key, "voltage-states")) {
              CFDataRef data = (CFDataRef)values[i];
              const uint8_t *bytes = CFDataGetBytePtr(data);
              CFIndex len = CFDataGetLength(data);
              int totalEntries = (int)(len / 8);

              int validFreqs = 0;
              uint32_t currentMaxFreq = 0;

              for (int j = 0; j < totalEntries; j++) {
                uint32_t val;
                memcpy(&val, bytes + (j * 8), 4);

                if (val > 100000000) {
                  validFreqs++;
                  if (val > currentMaxFreq) {
                    currentMaxFreq = val;
                  }
                }
              }

              if (validFreqs > 0) {
                if (currentMaxFreq < bestMaxFreq) {
                  bestMaxFreq = currentMaxFreq;
                  bestData = data;
                  bestValidFreqs = validFreqs;
                }
              }
            }
          }
        }

        if (bestData != NULL) {
          CFIndex len = CFDataGetLength(bestData);
          const uint8_t *bytes = CFDataGetBytePtr(bestData);
          int totalFreqs = (int)(len / 8);
          if (totalFreqs > 64)
            totalFreqs = 64;
          g_gpu_freq_count = 0;
          for (int i = 0; i < totalFreqs; i++) {
            uint32_t freq = 0;
            memcpy(&freq, bytes + (i * 8), 4);
            uint32_t freqMHz = freq / 1000000;
            if (freqMHz > 0) {
              g_gpu_freqs[g_gpu_freq_count++] = freqMHz;
            }
          }
        }
        CFRelease(properties);
      }
    }
    IOObjectRelease(entry);
  }
  IOObjectRelease(iterator);
}

// === kperf-based DRAM BW monitoring (fallback for M5+ chips) ===
// On M5+ chips, IOReport AMC Stats channels are kernel-blocked.
// We use hardware PMU counters (L1D cache miss events) via Apple's
// private kperf/kpep frameworks to measure CPU-side DRAM bandwidth.
// Each L1D cache miss = 128-byte cache line fetch from L2/DRAM.
// Requires root for PMU access; works without root but BW shows 0.

#define KPC_CLASS_FIXED  1
#define KPC_CLASS_CONFIG 2
#define KPC_CLASS_POWER  4

typedef uint32_t (*kpc_get_counter_count_fn)(uint32_t);
typedef int (*kpc_force_all_ctrs_set_fn)(int);
typedef int (*kpc_set_counting_fn)(uint32_t);
typedef int (*kpc_set_thread_counting_fn)(uint32_t);
typedef int (*kpc_get_config_fn)(uint32_t, void *);
typedef int (*kpc_get_cpu_counters_fn)(int, uint32_t, int *, uint64_t *);
typedef uint32_t (*kpc_get_config_count_fn)(uint32_t);

typedef int (*kpep_db_create_fn)(const char *, void **);
typedef void (*kpep_db_free_fn)(void *);
typedef int (*kpep_db_event_fn)(void *, const char *, void **);
typedef int (*kpep_config_create_fn)(void *, void **);
typedef void (*kpep_config_free_fn)(void *);
typedef int (*kpep_config_add_event_fn)(void *, void **, uint32_t, uint32_t *);
typedef int (*kpep_config_force_counters_fn)(void *);
typedef int (*kpep_config_apply_fn)(void *);

static int g_kperf_active = 0;
static int g_kperf_ncpu = 0;
static uint32_t g_kperf_nFixed = 0;
static uint32_t g_kperf_nConfig = 0;
static uint32_t g_kperf_perCpu = 0;
static uint32_t g_kperf_classes = 0;
static kpc_get_cpu_counters_fn g_getCpuCounters = NULL;
static kpc_force_all_ctrs_set_fn g_forceCtrs = NULL;
static uint64_t *g_kperf_prev = NULL;  // Previous sample buffer

static void initKperfDramBW(void) {
  void *kperfdata = dlopen("/System/Library/PrivateFrameworks/kperfdata.framework/kperfdata", RTLD_NOW);
  void *kperf_lib = dlopen("/System/Library/PrivateFrameworks/kperf.framework/kperf", RTLD_NOW);
  if (!kperfdata || !kperf_lib) return;

  kpc_get_counter_count_fn counterCount = dlsym(kperf_lib, "kpc_get_counter_count");
  kpc_force_all_ctrs_set_fn forceCtrs = dlsym(kperf_lib, "kpc_force_all_ctrs_set");
  kpc_set_counting_fn setCounting = dlsym(kperf_lib, "kpc_set_counting");
  kpc_set_thread_counting_fn setThreadCounting = dlsym(kperf_lib, "kpc_set_thread_counting");
  kpc_get_cpu_counters_fn getCpuCounters = dlsym(kperf_lib, "kpc_get_cpu_counters");

  kpep_db_create_fn dbCreate = dlsym(kperfdata, "kpep_db_create");
  kpep_db_free_fn dbFree = dlsym(kperfdata, "kpep_db_free");
  kpep_db_event_fn dbEvent = dlsym(kperfdata, "kpep_db_event");
  kpep_config_create_fn cfgCreate = dlsym(kperfdata, "kpep_config_create");
  kpep_config_free_fn cfgFree = dlsym(kperfdata, "kpep_config_free");
  kpep_config_add_event_fn cfgAdd = dlsym(kperfdata, "kpep_config_add_event");
  kpep_config_force_counters_fn cfgForce = dlsym(kperfdata, "kpep_config_force_counters");
  kpep_config_apply_fn cfgApply = dlsym(kperfdata, "kpep_config_apply");

  if (!counterCount || !forceCtrs || !setCounting || !getCpuCounters) return;

  // Get CPU count and counter layout first (no privileges needed)
  int ncpu = 0;
  size_t ncpuSz = sizeof(ncpu);
  sysctlbyname("hw.ncpu", &ncpu, &ncpuSz, NULL, 0);
  if (ncpu <= 0) return;

  uint32_t nFixed = counterCount(KPC_CLASS_FIXED);
  uint32_t nConfig = counterCount(KPC_CLASS_CONFIG);
  uint32_t nPower = counterCount(KPC_CLASS_POWER);
  uint32_t perCpu = nFixed + nConfig + nPower;
  uint32_t allClasses = KPC_CLASS_FIXED | KPC_CLASS_CONFIG | KPC_CLASS_POWER;

  int hasRoot = (forceCtrs(1) == 0);

  if (hasRoot) {
    // Root path: configure PMU for L1D cache miss events via kpep
    if (dbCreate && dbEvent && cfgCreate && cfgAdd && cfgForce && cfgApply) {
      void *db = NULL;
      if (dbCreate(NULL, &db) == 0 && db) {
        void *cfg = NULL;
        if (cfgCreate(db, &cfg) == 0 && cfg) {
          void *evLd = NULL, *evSt = NULL;
          dbEvent(db, "L1D_CACHE_MISS_LD_NONSPEC", &evLd);
          dbEvent(db, "L1D_CACHE_MISS_ST_NONSPEC", &evSt);
          uint32_t idx;
          if (evLd) cfgAdd(cfg, &evLd, 0, &idx);
          if (evSt) cfgAdd(cfg, &evSt, 0, &idx);
          cfgForce(cfg);
          cfgApply(cfg);
          if (cfgFree) cfgFree(cfg);
        }
        if (dbFree) dbFree(db);
      }
    }
    setCounting(allClasses);
    if (setThreadCounting) setThreadCounting(allClasses);
  } else {
    // Non-root path: try to read existing counters without configuration.
    // Enable counting (may succeed without root on some macOS versions).
    setCounting(allClasses);
    if (setThreadCounting) setThreadCounting(allClasses);
  }

  // Allocate buffer for previous sample
  g_kperf_prev = calloc((size_t)ncpu * perCpu, sizeof(uint64_t));
  if (!g_kperf_prev) {
    if (hasRoot) forceCtrs(0);
    return;
  }

  // Test if counter reading actually works
  int cpu;
  int ret = getCpuCounters(1, allClasses, &cpu, g_kperf_prev);
  if (ret != 0) {
    free(g_kperf_prev);
    g_kperf_prev = NULL;
    if (hasRoot) forceCtrs(0);
    return;
  }

  // Verify we got non-zero data in configurable counters
  int hasData = 0;
  for (int c = 0; c < ncpu && !hasData; c++) {
    size_t base = (size_t)c * perCpu;
    if (g_kperf_prev[base + nFixed] > 0) hasData = 1;
  }
  if (!hasData) {
    free(g_kperf_prev);
    g_kperf_prev = NULL;
    if (hasRoot) forceCtrs(0);
    return;
  }

  g_kperf_active = 1;
  g_kperf_ncpu = ncpu;
  g_kperf_nFixed = nFixed;
  g_kperf_nConfig = nConfig;
  g_kperf_perCpu = perCpu;
  g_kperf_classes = allClasses;
  g_getCpuCounters = getCpuCounters;
  g_forceCtrs = hasRoot ? forceCtrs : NULL;
}

// Read kperf DRAM BW: returns read/write bytes since last call
static void readKperfDramBW(int64_t *readBytes, int64_t *writeBytes) {
  *readBytes = 0;
  *writeBytes = 0;
  if (!g_kperf_active || !g_getCpuCounters || !g_kperf_prev) return;

  size_t bufSize = (size_t)g_kperf_ncpu * g_kperf_perCpu;
  uint64_t *cur = calloc(bufSize, sizeof(uint64_t));
  if (!cur) return;

  int cpu;
  g_getCpuCounters(1, g_kperf_classes, &cpu, cur);

  // Sum L1D miss deltas across all CPUs
  // Counter layout per CPU: [fixed0, fixed1, cfg0, cfg1, cfg2, ..., pwr0, ...]
  // cfg0 = L1D_CACHE_MISS_LD_NONSPEC, cfg1 = L1D_CACHE_MISS_ST_NONSPEC
  //
  // Per-CPU delta cap: A single Apple Silicon core maxes ~280M L1D misses/sec
  // under extreme memory stress. Any delta > 500M per core is a CPU sleep/wake
  // artifact (counter jumping from 0 to accumulated boot-time value).
  static const int64_t MAX_MISS_PER_CPU = 500000000LL;  // 500M

  int64_t totalLdMiss = 0, totalStMiss = 0;
  for (int c = 0; c < g_kperf_ncpu; c++) {
    size_t base = (size_t)c * g_kperf_perCpu;
    uint64_t ldCur = cur[base + g_kperf_nFixed];
    uint64_t ldPrev = g_kperf_prev[base + g_kperf_nFixed];
    uint64_t stCur = cur[base + g_kperf_nFixed + 1];
    uint64_t stPrev = g_kperf_prev[base + g_kperf_nFixed + 1];

    // Only sum positive deltas within the per-CPU cap
    if (ldCur > ldPrev) {
      int64_t d = (int64_t)(ldCur - ldPrev);
      if (d <= MAX_MISS_PER_CPU) totalLdMiss += d;
    }
    if (stCur > stPrev) {
      int64_t d = (int64_t)(stCur - stPrev);
      if (d <= MAX_MISS_PER_CPU) totalStMiss += d;
    }
  }

  // Each L1D miss = 128-byte cache line transfer
  *readBytes = totalLdMiss * 128;
  *writeBytes = totalStMiss * 128;

  // Save current as previous for next call
  memcpy(g_kperf_prev, cur, bufSize * sizeof(uint64_t));
  free(cur);
}

// Forward declarations for calibration function
static int cfStringMatch(CFStringRef str, const char *cStr);
static int cfStringStartsWith(CFStringRef str, const char *prefix);
static double energyToWatts(int64_t energy, CFStringRef unitRef, double durationMs);

// === Auto-calibration for DRAM power → bandwidth conversion ===
// On M5+ chips where AMC Stats is kernel-blocked, we estimate DRAM BW from
// DRAM power. The conversion factor (GB/s per watt) is chip-specific, so we
// auto-calibrate at startup by running a brief known workload:
//   1. Spawn 4 threads reading 256MB buffers (>> L2 cache → every read hits DRAM)
//   2. Measure exact bytes read and DRAM power delta simultaneously
//   3. Derive: g_dramGBsPerWatt = measured_throughput / measured_power_delta
// This makes the formula accurate on ANY chip without hard-coding.
static double g_dramGBsPerWatt = 25.1; // default fallback
static double g_dramIdlePowerW = 0.3;  // DRAM idle/static power (leakage + refresh)
static volatile int g_calib_running = 0;
static volatile int64_t g_calib_bytes = 0;

static void *calibThread(void *arg) {
  size_t sz = 256ULL * 1024 * 1024; // 256MB per thread (>> L2 cache per core)
  volatile char *buf = malloc(sz);
  if (!buf) return NULL;
  memset((void *)buf, 0xAB, sz);
  while (g_calib_running) {
    volatile uint64_t sum = 0;
    for (size_t i = 0; i < sz; i += 128)
      sum += buf[i];
    // Update byte counter atomically inside the loop so the main thread
    // can read g_calib_bytes at any point during the measurement window.
    __sync_fetch_and_add(&g_calib_bytes, (int64_t)sz);
  }
  free((void *)buf);
  return NULL;
}

// Run auto-calibration. Called once at init when power-based DRAM BW is needed.
// Takes ~2 seconds. Measures DRAM power during known workload to derive
// the GB/s-per-watt constant for this specific chip.
static void calibrateDramBwFromPower(void) {
  if (g_channels == NULL || g_subscription == NULL)
    return;

  // Step 1: idle baseline (500ms)
  CFDictionaryRef s1 = IOReportCreateSamples(g_subscription, g_channels, NULL);
  usleep(500000);
  CFDictionaryRef s2 = IOReportCreateSamples(g_subscription, g_channels, NULL);

  double idleDramPower = 0;
  if (s1 && s2) {
    CFDictionaryRef delta = IOReportCreateSamplesDelta(s1, s2, NULL);
    if (delta) {
      CFArrayRef arr = CFDictionaryGetValue(delta, CFSTR("IOReportChannels"));
      CFIndex cnt = arr ? CFArrayGetCount(arr) : 0;
      for (CFIndex i = 0; i < cnt; i++) {
        CFDictionaryRef ch = (CFDictionaryRef)CFArrayGetValueAtIndex(arr, i);
        CFStringRef grp = IOReportChannelGetGroup(ch);
        CFStringRef name = IOReportChannelGetChannelName(ch);
        if (grp && name && cfStringMatch(grp, "Energy Model") &&
            cfStringStartsWith(name, "DRAM")) {
          int64_t val = IOReportSimpleGetIntegerValue(ch, 0);
          CFStringRef unitRef = IOReportChannelGetUnitLabel(ch);
          idleDramPower += energyToWatts(val, unitRef, 500.0);
        }
      }
      CFRelease(delta);
    }
  }
  if (s1) CFRelease(s1);
  if (s2) CFRelease(s2);

  // Step 2: run 4 threads for ~1.5 seconds, measure during 1 second
  __sync_lock_test_and_set(&g_calib_bytes, 0);
  g_calib_running = 1;
  pthread_t threads[4];
  for (int t = 0; t < 4; t++)
    pthread_create(&threads[t], NULL, calibThread, NULL);

  usleep(500000); // 500ms ramp — let DRAM frequency settle

  // Reset counter, then measure for exactly 1 second
  __sync_lock_test_and_set(&g_calib_bytes, 0);
  s1 = IOReportCreateSamples(g_subscription, g_channels, NULL);
  usleep(1000000); // 1 second measurement
  s2 = IOReportCreateSamples(g_subscription, g_channels, NULL);
  int64_t bytesRead = g_calib_bytes;

  g_calib_running = 0;
  for (int t = 0; t < 4; t++)
    pthread_join(threads[t], NULL);

  // Extract DRAM power during stress
  double stressDramPower = 0;
  if (s1 && s2) {
    CFDictionaryRef delta = IOReportCreateSamplesDelta(s1, s2, NULL);
    if (delta) {
      CFArrayRef arr = CFDictionaryGetValue(delta, CFSTR("IOReportChannels"));
      CFIndex cnt = arr ? CFArrayGetCount(arr) : 0;
      for (CFIndex i = 0; i < cnt; i++) {
        CFDictionaryRef ch = (CFDictionaryRef)CFArrayGetValueAtIndex(arr, i);
        CFStringRef grp = IOReportChannelGetGroup(ch);
        CFStringRef name = IOReportChannelGetChannelName(ch);
        if (grp && name && cfStringMatch(grp, "Energy Model") &&
            cfStringStartsWith(name, "DRAM")) {
          int64_t val = IOReportSimpleGetIntegerValue(ch, 0);
          CFStringRef unitRef = IOReportChannelGetUnitLabel(ch);
          stressDramPower += energyToWatts(val, unitRef, 1000.0);
        }
      }
      CFRelease(delta);
    }
  }
  if (s1) CFRelease(s1);
  if (s2) CFRelease(s2);

  // Derive calibration constant
  double deltaPower = stressDramPower - idleDramPower;
  double throughputGBs = (double)bytesRead / 1e9; // GB in 1 second

  if (deltaPower > 0.01 && throughputGBs > 1.0) {
    g_dramGBsPerWatt = throughputGBs / deltaPower;
    // Use 90% of measured idle as baseline: some "idle" power includes
    // background memory traffic We want to subtract static/leakage only.
    g_dramIdlePowerW = idleDramPower * 0.9;
    // Sanity check: should be between 5 and 500 GB/s per watt
    if (g_dramGBsPerWatt < 5.0 || g_dramGBsPerWatt > 500.0) {
      g_dramGBsPerWatt = 25.1; // fall back to default
    }
    if (g_dramIdlePowerW < 0) g_dramIdlePowerW = 0;
  }
}


int initIOReport() {
  if (g_channels != NULL) {
    return 0;
  }

  CFStringRef energyGroup = CFSTR("Energy Model");
  CFStringRef gpuGroup = CFSTR("GPU Stats");
  CFStringRef cpuGroup = CFSTR("CPU Stats");
  CFStringRef amcGroup = CFSTR("AMC Stats");

  CFDictionaryRef energyChan =
      IOReportCopyChannelsInGroup(energyGroup, NULL, 0, 0, 0);
  CFDictionaryRef gpuChan =
      IOReportCopyChannelsInGroup(gpuGroup, NULL, 0, 0, 0);
  int hasGpu = (gpuChan != NULL);
  int hasCpu = 0, hasAmc = 0;

  if (energyChan == NULL) {
    fprintf(stderr, "initIOReport: 'Energy Model' channel group not found\n");
    return -1;
  }

  if (gpuChan != NULL) {
    IOReportMergeChannels(energyChan, gpuChan, NULL);
    CFRelease(gpuChan);
  } else {
    fprintf(stderr, "initIOReport: warning: 'GPU Stats' channel group not found\n");
  }

  CFDictionaryRef cpuChan =
      IOReportCopyChannelsInGroup(cpuGroup, NULL, 0, 0, 0);
  hasCpu = (cpuChan != NULL);
  if (cpuChan != NULL) {
    IOReportMergeChannels(energyChan, cpuChan, NULL);
    CFRelease(cpuChan);
  } else {
    fprintf(stderr, "initIOReport: warning: 'CPU Stats' channel group not found\n");
  }

  CFDictionaryRef amcChan =
      IOReportCopyChannelsInGroup(amcGroup, NULL, 0, 0, 0);
  hasAmc = (amcChan != NULL);
  if (amcChan != NULL) {
    IOReportMergeChannels(energyChan, amcChan, NULL);
    CFRelease(amcChan);
  }

  // DON'T subscribe to PMP yet — probe AMC Stats first.
  // PMP adds hundreds of channels that increase kernel IPC cost
  // on every IOReportCreateSamples call. Only needed on A-series
  // where AMC Stats doesn't produce data.

  CFIndex size = CFDictionaryGetCount(energyChan);
  g_channels =
      CFDictionaryCreateMutableCopy(kCFAllocatorDefault, size, energyChan);
  CFRelease(energyChan);

  if (g_channels == NULL) {
    fprintf(stderr, "initIOReport: failed to create mutable channel dictionary (size=%ld)\n", (long)size);
    return -2;
  }

  CFMutableDictionaryRef subsystem = NULL;
  g_subscription =
      IOReportCreateSubscription(NULL, g_channels, &subsystem, 0, NULL);

  if (g_subscription == NULL) {
    // Count channels for diagnostic info
    CFArrayRef chArr = CFDictionaryGetValue(g_channels, CFSTR("IOReportChannels"));
    CFIndex chCount = chArr ? CFArrayGetCount(chArr) : 0;
    fprintf(stderr, "initIOReport: IOReportCreateSubscription failed "
            "(channels=%ld, groups: Energy=%s GPU=%s CPU=%s AMC=%s)\n",
            (long)chCount,
            "yes",
            hasGpu ? "yes" : "no",
            hasCpu ? "yes" : "no",
            hasAmc ? "yes" : "no");
    CFRelease(g_channels);
    g_channels = NULL;
    return -3;
  }

  loadGpuFrequencies();
  loadCpuFrequencies();

  g_smcConn = SMCOpen();
  loadSMCTempKeys();

  // Probe AMC Stats with a quick 50ms sample to check if it produces data.
  // This determines whether we need PMP channels and DRAM BW calibration.
  int hasDirectBW = 0;
  {
    CFDictionaryRef probe1 = IOReportCreateSamples(g_subscription, g_channels, NULL);
    usleep(50000); // 50ms probe (sufficient to detect non-zero AMC data)
    CFDictionaryRef probe2 = IOReportCreateSamples(g_subscription, g_channels, NULL);
    if (probe1 && probe2) {
      CFDictionaryRef probeDelta = IOReportCreateSamplesDelta(probe1, probe2, NULL);
      if (probeDelta) {
        CFArrayRef arr = CFDictionaryGetValue(probeDelta, CFSTR("IOReportChannels"));
        CFIndex cnt = arr ? CFArrayGetCount(arr) : 0;
        for (CFIndex i = 0; i < cnt && !hasDirectBW; i++) {
          CFDictionaryRef ch = (CFDictionaryRef)CFArrayGetValueAtIndex(arr, i);
          CFStringRef grp = IOReportChannelGetGroup(ch);
          if (!grp) continue;
          char grpStr[64] = {0};
          CFStringGetCString(grp, grpStr, sizeof(grpStr), kCFStringEncodingUTF8);
          if (strcmp(grpStr, "AMC Stats") == 0) {
            char name[256] = {0};
            CFStringRef nameRef = IOReportChannelGetChannelName(ch);
            if (nameRef)
              CFStringGetCString(nameRef, name, sizeof(name), kCFStringEncodingUTF8);
            if (strstr(name, "RD") || strstr(name, "WR")) {
              int64_t val = IOReportSimpleGetIntegerValue(ch, 0);
              if (val > 0) hasDirectBW = 1;
            }
          }
        }
        CFRelease(probeDelta);
      }
    }
    if (probe1) CFRelease(probe1);
    if (probe2) CFRelease(probe2);
  }

  // Only add PMP channels if AMC Stats doesn't produce data (A-series chips).
  // On M-series, this saves hundreds of kernel-iterated channels per tick.
  if (!hasDirectBW) {
    CFDictionaryRef pmpChan =
        IOReportCopyChannelsInGroup(CFSTR("PMP"), NULL, 0, 0, 0);
    if (pmpChan != NULL) {
      IOReportMergeChannels((CFDictionaryRef)g_channels, pmpChan, NULL);
      CFRelease(pmpChan);

      // Re-create subscription with PMP channels included.
      // Guard: don't lose the working subscription if this fails.
      IOReportSubscriptionRef newSub =
          IOReportCreateSubscription(NULL, g_channels, &subsystem, 0, NULL);
      if (newSub != NULL) {
        g_subscription = newSub;
      }
    }

    // Initialize kperf-based DRAM BW monitoring as additional fallback.
    // Only needed when AMC Stats doesn't work (M5+ / A-series).
    // Requires root; fails silently without root.
    initKperfDramBW();

    // Auto-calibrate DRAM power → bandwidth conversion.
    // Only needed on M5+ where AMC Stats is blocked.
    calibrateDramBwFromPower();
  }

  return 0;
}

void debugIOReport() {
  if (initIOReport() != 0) {
    printf("Failed to initialize IOReport\n");
    return;
  }

  // Subscribe to everything for debugging
  IOReportSubscriptionRef sub = NULL;
  CFMutableDictionaryRef allChannels = NULL;
  CFMutableDictionaryRef subChannels = NULL;

  // Try to get ALL channels first
  CFDictionaryRef allChans = IOReportCopyChannelsInGroup(NULL, NULL, 0, 0, 0);

  if (allChans == NULL) {
    printf("Wildcard channel copy failed. Trying specific groups...\n");
    allChannels = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
                                            &kCFTypeDictionaryKeyCallBacks,
                                            &kCFTypeDictionaryValueCallBacks);

    const char *groups[] = {
        "Energy Model",           "GPU Stats", "CPU Stats", "ODS",
        "Performance Statistics", "CLPC",      "PMP",       NULL};

    for (int i = 0; groups[i] != NULL; i++) {
      CFStringRef groupStr = CFStringCreateWithCString(
          kCFAllocatorDefault, groups[i], kCFStringEncodingUTF8);
      CFDictionaryRef groupChans =
          IOReportCopyChannelsInGroup(groupStr, NULL, 0, 0, 0);
      if (groupChans != NULL) {
        printf("Found channels in group: %s\n", groups[i]);
        IOReportMergeChannels(allChannels, groupChans, NULL);
        CFRelease(groupChans);
      } else {
        printf("No channels in group: %s\n", groups[i]);
      }
      CFRelease(groupStr);
    }
  } else {
    allChannels = CFDictionaryCreateMutableCopy(
        kCFAllocatorDefault, CFDictionaryGetCount(allChans), allChans);
    CFRelease(allChans);
  }

  if (CFDictionaryGetCount(allChannels) == 0) {
    printf("No channels found after all attempts.\n");
    CFRelease(allChannels);
    return;
  }

  sub = IOReportCreateSubscription(NULL, allChannels, &subChannels, 0, NULL);
  if (sub == NULL) {
    printf("Failed to create subscription\n");
    CFRelease(allChannels);
    return;
  }

  // Create a sample
  CFDictionaryRef sample = IOReportCreateSamples(sub, allChannels, NULL);
  if (sample == NULL) {
    printf("Failed to create samples\n");
    CFRelease(allChannels);
    return;
  }

  CFArrayRef channels = CFDictionaryGetValue(sample, CFSTR("IOReportChannels"));
  if (channels != NULL) {
    CFIndex count = CFArrayGetCount(channels);
    printf("--- IOReport Channels Dump (%ld channels) ---\n", count);
    printf("%-20s | %-30s | %-40s | %s\n", "GROUP", "SUBGROUP", "CHANNEL",
           "UNIT");
    printf("-------------------------------------------------------------------"
           "-----------------------------------------------\n");

    for (CFIndex i = 0; i < count; i++) {
      CFDictionaryRef item =
          (CFDictionaryRef)CFArrayGetValueAtIndex(channels, i);
      if (item == NULL)
        continue;

      CFStringRef groupRef = IOReportChannelGetGroup(item);
      CFStringRef subGroupRef = IOReportChannelGetSubGroup(item);
      CFStringRef channelRef = IOReportChannelGetChannelName(item);
      CFStringRef unitRef = IOReportChannelGetUnitLabel(item);

      char group[64] = {0};
      char subGroup[64] = {0};
      char channel[512] = {0};
      char unit[32] = {0};

      if (groupRef)
        CFStringGetCString(groupRef, group, sizeof(group),
                           kCFStringEncodingUTF8);
      if (subGroupRef)
        CFStringGetCString(subGroupRef, subGroup, sizeof(subGroup),
                           kCFStringEncodingUTF8);
      if (channelRef)
        CFStringGetCString(channelRef, channel, sizeof(channel),
                           kCFStringEncodingUTF8);
      if (unitRef)
        CFStringGetCString(unitRef, unit, sizeof(unit), kCFStringEncodingUTF8);

      printf("%-20s | %-30s | %-40s | %s\n", group, subGroup, channel, unit);
    }
  }

  CFRelease(sample);
  CFRelease(allChannels);
}

// Standalone diagnostic dump — works even if initIOReport() fails.
// Designed for users to paste output into GitHub issues.
void dumpIOReportDebug(void) {
  printf("=== mactop IOReport Debug Dump ===\n\n");

  // 1. Probe all known IOReport channel groups
  printf("--- IOReport Channel Groups ---\n");
  const char *groups[] = {
    "Energy Model", "GPU Stats", "CPU Stats", "AMC Stats",
    "PMP", "CLPC", "ODS", "Performance Statistics",
    NULL
  };
  for (int i = 0; groups[i] != NULL; i++) {
    CFStringRef groupStr = CFStringCreateWithCString(
        kCFAllocatorDefault, groups[i], kCFStringEncodingUTF8);
    CFDictionaryRef ch = IOReportCopyChannelsInGroup(groupStr, NULL, 0, 0, 0);
    if (ch != NULL) {
      CFArrayRef arr = CFDictionaryGetValue(ch, CFSTR("IOReportChannels"));
      CFIndex count = arr ? CFArrayGetCount(arr) : 0;
      printf("  %-25s [OK]  %ld channels\n", groups[i], (long)count);
      CFRelease(ch);
    } else {
      printf("  %-25s [NOT FOUND]\n", groups[i]);
    }
    CFRelease(groupStr);
  }

  // 2. Test subscription with Energy Model only (minimal)
  printf("\n--- Subscription Test ---\n");
  CFDictionaryRef energyChan =
      IOReportCopyChannelsInGroup(CFSTR("Energy Model"), NULL, 0, 0, 0);
  if (energyChan != NULL) {
    CFMutableDictionaryRef mutable =
        CFDictionaryCreateMutableCopy(kCFAllocatorDefault,
                                     CFDictionaryGetCount(energyChan), energyChan);
    CFRelease(energyChan);
    CFMutableDictionaryRef subsystem = NULL;
    IOReportSubscriptionRef sub =
        IOReportCreateSubscription(NULL, mutable, &subsystem, 0, NULL);
    if (sub != NULL) {
      printf("  Energy-only subscription: [OK]\n");
      // Test a quick sample
      CFDictionaryRef s = IOReportCreateSamples(sub, mutable, NULL);
      printf("  Quick sample:             %s\n", s ? "[OK]" : "[FAIL]");
      if (s) CFRelease(s);
    } else {
      printf("  Energy-only subscription: [FAIL] — this is the root cause\n");
      printf("  Possible causes:\n");
      printf("    - IOReport access denied (check: log show --predicate 'eventMessage CONTAINS \"IOReport\"' --last 1m)\n");
      printf("    - Binary not signed (try: codesign -s - /path/to/mactop)\n");
      printf("    - System restriction on this macOS version\n");
    }
    CFRelease(mutable);
  } else {
    printf("  Energy Model not available — cannot test subscription\n");
  }

  // 3. Test full subscription (as initIOReport does)
  printf("\n--- Full Subscription Test ---\n");
  energyChan = IOReportCopyChannelsInGroup(CFSTR("Energy Model"), NULL, 0, 0, 0);
  if (energyChan != NULL) {
    CFDictionaryRef gpuChan = IOReportCopyChannelsInGroup(CFSTR("GPU Stats"), NULL, 0, 0, 0);
    CFDictionaryRef cpuChan = IOReportCopyChannelsInGroup(CFSTR("CPU Stats"), NULL, 0, 0, 0);
    CFDictionaryRef amcChan = IOReportCopyChannelsInGroup(CFSTR("AMC Stats"), NULL, 0, 0, 0);
    if (gpuChan) { IOReportMergeChannels(energyChan, gpuChan, NULL); CFRelease(gpuChan); }
    if (cpuChan) { IOReportMergeChannels(energyChan, cpuChan, NULL); CFRelease(cpuChan); }
    if (amcChan) { IOReportMergeChannels(energyChan, amcChan, NULL); CFRelease(amcChan); }
    CFMutableDictionaryRef merged =
        CFDictionaryCreateMutableCopy(kCFAllocatorDefault,
                                     CFDictionaryGetCount(energyChan), energyChan);
    CFRelease(energyChan);
    CFArrayRef mergedArr = CFDictionaryGetValue(merged, CFSTR("IOReportChannels"));
    CFIndex mergedCount = mergedArr ? CFArrayGetCount(mergedArr) : 0;
    printf("  Merged channels: %ld\n", (long)mergedCount);
    CFMutableDictionaryRef subsystem = NULL;
    IOReportSubscriptionRef sub =
        IOReportCreateSubscription(NULL, merged, &subsystem, 0, NULL);
    printf("  Full subscription: %s\n", sub ? "[OK]" : "[FAIL]");
    CFRelease(merged);
  }

  // 4. SMC connectivity
  printf("\n--- SMC ---\n");
  io_connect_t smc = SMCOpen();
  printf("  SMC connection: %s (use --dump-temps for full key list)\n",
         smc ? "[OK]" : "[FAIL]");
  if (smc) SMCClose(smc);

  // 5. HID sensor availability
  printf("\n--- HID Temperature Sensors ---\n");
  IOHIDEventSystemClientRef hidClient = getHIDClient();
  if (hidClient) {
    CFArrayRef services = IOHIDEventSystemClientCopyServices(hidClient);
    if (services) {
      CFIndex count = CFArrayGetCount(services);
      int eCount = 0, pCount = 0, sCount = 0, gpuCount = 0, nandCount = 0, otherCount = 0;
      for (CFIndex i = 0; i < count; i++) {
        IOHIDServiceClientRef svc =
            (IOHIDServiceClientRef)CFArrayGetValueAtIndex(services, i);
        CFStringRef prodRef = IOHIDServiceClientCopyProperty(svc, CFSTR("Product"));
        if (!prodRef) continue;
        char prod[256] = {0};
        CFStringGetCString(prodRef, prod, sizeof(prod), kCFStringEncodingUTF8);
        if (strstr(prod, "eACC")) eCount++;
        else if (strstr(prod, "pACC") || strstr(prod, "mACC")) pCount++;
        else if (strstr(prod, "sACC")) sCount++;
        else if (strstr(prod, "GPU")) gpuCount++;
        else if (strstr(prod, "NAND")) nandCount++;
        else otherCount++;
        CFRelease(prodRef);
      }
      printf("  Total HID temp services: %ld\n", (long)count);
      printf("  E-Core(eACC): %d  P-Core(pACC/mACC): %d  S-Core(sACC): %d\n",
             eCount, pCount, sCount);
      printf("  GPU: %d  NAND: %d  Other: %d\n", gpuCount, nandCount, otherCount);
      CFRelease(services);
    } else {
      printf("  HID services: [NOT AVAILABLE]\n");
    }
  } else {
    printf("  HID client: [FAIL]\n");
  }

  // 6. NVMe SMART capability
  printf("\n--- NVMe SMART ---\n");
  CFMutableDictionaryRef match = IOServiceMatching("IOBlockStorageDevice");
  io_iterator_t iter;
  kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, match, &iter);
  if (kr == kIOReturnSuccess) {
    io_service_t svc;
    int nvmeCount = 0;
    while ((svc = IOIteratorNext(iter)) != 0) {
      CFTypeRef smart = IORegistryEntryCreateCFProperty(
          svc, CFSTR("NVMe SMART Capable"), kCFAllocatorDefault, 0);
      if (smart) {
        nvmeCount++;
        // Get class name
        io_name_t className;
        IOObjectGetClass(svc, className);
        // Get model
        char model[64] = "unknown";
        CFDictionaryRef devChars = IORegistryEntryCreateCFProperty(
            svc, CFSTR("Device Characteristics"), kCFAllocatorDefault, 0);
        if (devChars && CFGetTypeID(devChars) == CFDictionaryGetTypeID()) {
          CFStringRef prodName = CFDictionaryGetValue(devChars, CFSTR("Product Name"));
          if (prodName) CFStringGetCString(prodName, model, sizeof(model), kCFStringEncodingUTF8);
        }
        if (devChars) CFRelease(devChars);

        // Test plugin
        CFUUIDRef factoryID = CFUUIDGetConstantUUIDWithBytes(NULL,
            0xAA,0x0F,0xA6,0xF9,0xC2,0xD6,0x45,0x7F,
            0xB1,0x0B,0x59,0xA1,0x32,0x53,0x29,0x2F);
        IOCFPlugInInterface **plugin = NULL;
        SInt32 score = 0;
        kern_return_t pkr = IOCreatePlugInInterfaceForService(
            svc, factoryID, kIOCFPlugInInterfaceID, &plugin, &score);
        printf("  [%d] %s (%s) — SMART plugin: %s\n",
               nvmeCount, model, className,
               pkr == kIOReturnSuccess ? "OK" : "FAIL (using HID fallback)");
        if (plugin) (*plugin)->Release(plugin);
        CFRelease(smart);
      }
      IOObjectRelease(svc);
    }
    IOObjectRelease(iter);
    if (nvmeCount == 0) printf("  No SMART-capable NVMe devices found\n");
  } else {
    printf("  IOBlockStorageDevice matching failed\n");
  }

  printf("\n=== End Debug Dump ===\n");
}
typedef struct {
  double cpuPower;
  double gpuPower;
  double anePower;
  double dramPower;
  double gpuSramPower;
  double systemPower;
  int gpuFreqMHz;
  double gpuActive;
  double eClusterActive;
  double pClusterActive;
  double sClusterActive;
  int eClusterFreqMHz;
  int pClusterFreqMHz;
  int sClusterFreqMHz;
  float socTemp;
  float cpuTemp;
  float gpuTemp;
  int64_t dramReadBytes;
  int64_t dramWriteBytes;
  // Fan data
  int fanCount;
  fan_info_t fans[8];
  // Comprehensive temperature sensors
  int tempSensorCount;
  temp_sensor_t temps[512];
} PowerMetrics;

static int cfStringMatch(CFStringRef str, const char *match) {
  if (str == NULL || match == NULL)
    return 0;
  CFStringRef matchStr = CFStringCreateWithCString(kCFAllocatorDefault, match,
                                                   kCFStringEncodingUTF8);
  if (matchStr == NULL)
    return 0;
  int result = (CFStringCompare(str, matchStr, 0) == kCFCompareEqualTo);
  CFRelease(matchStr);
  return result;
}

static int cfStringContains(CFStringRef str, const char *substr) {
  if (str == NULL || substr == NULL)
    return 0;
  CFStringRef substrRef = CFStringCreateWithCString(kCFAllocatorDefault, substr,
                                                    kCFStringEncodingUTF8);
  if (substrRef == NULL)
    return 0;
  CFRange result = CFStringFind(str, substrRef, 0);
  CFRelease(substrRef);
  return (result.location != kCFNotFound);
}

static int cfStringStartsWith(CFStringRef str, const char *prefix) {
  if (str == NULL || prefix == NULL)
    return 0;
  CFStringRef prefixRef = CFStringCreateWithCString(kCFAllocatorDefault, prefix,
                                                    kCFStringEncodingUTF8);
  if (prefixRef == NULL)
    return 0;
  int result = CFStringHasPrefix(str, prefixRef);
  CFRelease(prefixRef);
  return result;
}

static double energyToWatts(int64_t energy, CFStringRef unitRef,
                            double durationMs) {
  if (durationMs <= 0)
    durationMs = 1;
  double val = (double)energy;
  double rate = val / (durationMs / 1000.0);

  if (unitRef == NULL)
    return rate / 1e6;

  char unit[32] = {0};
  CFStringGetCString(unitRef, unit, sizeof(unit), kCFStringEncodingUTF8);

  for (int i = 0; unit[i]; i++) {
    if (unit[i] == ' ')
      unit[i] = '\0';
  }

  if (strcmp(unit, "mJ") == 0) {
    return rate / 1e3;
  } else if (strcmp(unit, "uJ") == 0) {
    return rate / 1e6;
  } else if (strcmp(unit, "nJ") == 0) {
    return rate / 1e9;
  }
  return rate / 1e6;
}

static char g_cpu_keys[64][5];
static int g_cpu_key_count = 0;
static char g_gpu_keys[64][5];
static int g_gpu_key_count = 0;

static const char *tempSensorName(const char *key) {
  // NVMe SMART sensors use 'Nv' prefix (non-SMC, synthetic keys)
  if (key[0] == 'N' && key[1] == 'v')
    return "NVMe";

  if (key[0] != 'T')
    return "Unknown";

  // Multi-char prefix matching for accuracy on Apple Silicon
  // TPD* = SoC Package Die, TRD* = GPU Render Die, TCM* = CPU Die Max
  if (key[1] == 'P' && key[2] == 'D')
    return "SoC Package";
  if (key[1] == 'P' && key[2] == 'M')
    return "SoC Package";
  if (key[1] == 'P' && key[2] == 'S')
    return "SoC Package";
  if (key[1] == 'R' && key[2] == 'D')
    return "GPU";
  if (key[1] == 'C' && key[2] == 'M')
    return "CPU Die"; // TCMb, TCMz = die max
  if (key[1] == 'C' && key[2] == 'D')
    return "CPU Die"; // TCDX = die aggregate

  switch (key[1]) {
  case 'p':
    return "CPU P-Core"; // Tp* = P-core per-core temps (M1/M2/M4)
  case 'e':
    return "CPU E-Core"; // Te* = E-core per-core temps (M3/M4)
  case 'f':
    return "CPU P-Core"; // Tf* = P-core per-core temps (M3)
  case 'g':
    return "GPU"; // Tg* = GPU cluster temps
  case 'C':
    return "CPU Core"; // TC1x-TCAx = CPU core temps
  case 'c':
    return "CPU Core"; // Tc* = CPU core
  case 'm':
    return "Memory"; // Tm* = Memory controller/DRAM
  case 'M':
    return "Memory"; // TM* = Memory VRM
  case 's':
    return (g_expected_scores > 0) ? "CPU S-Core" : "SSD"; // Ts* = S-Core on M5+, SSD on M1-M4
  case 'S':
    return "SSD"; // TS* = SSD controller
  case 'H':
    return "NAND"; // TH* = NAND/NVMe controller
  case 'a':
    return "Ambient"; // Ta* = Ambient/airflow probes
  case 'A':
    return "Ambient"; // TA* = Ambient
  case 'B':
    return "Board"; // TB* = Board thermal sensors (not battery on desktops)
  case 'b':
    return "Board"; // Tb* = Board
  case 'V':
    return "VRM"; // TV* = Voltage regulator module
  case 'P':
    return "SoC Package"; // TP* = SoC package/power supply
  case 'R':
    return "GPU"; // TR* = GPU render die
  case 'T':
    return "Thunderbolt"; // TT* = Thunderbolt controller
  case 'I':
    return "Thunderbolt"; // TI* = Thunderbolt interface
  case 'w':
  case 'W':
    return "Wireless";
  case 'D':
  case 'd':
    return "Display";
  case 'N':
    return "NAND";
  case 'L':
    return "Display";
  case 'F':
    return "Ambient"; // TF* = Fan proximity
  default:
    return "Other";
  }
}

static void loadSMCTempKeys() {
  if (g_cpu_key_count > 0 || g_gpu_key_count > 0)
    return;

  if (!g_smcConn)
    return;

  int totalKeys = SMCGetKeyCount(g_smcConn);
  for (int i = 0; i < totalKeys; i++) {
    char key[5];
    if (SMCGetKeyFromIndex(g_smcConn, i, key) != kIOReturnSuccess) {
      continue;
    }

    SMCKeyData_keyInfo_t keyInfo;
    if (SMCGetKeyInfo(g_smcConn, key, &keyInfo) != kIOReturnSuccess)
      continue;

    // Filter for 'flt ' type (1718383648)
    if (keyInfo.dataType != 1718383648)
      continue;

    // CPU Keys: Tp*, Te*, and Ts* (only if chip has S-cores)
    if ((key[0] == 'T' && (key[1] == 'p' || key[1] == 'e' || (key[1] == 's' && g_expected_scores > 0)))) {
      if (g_cpu_key_count < 64) {
        strcpy(g_cpu_keys[g_cpu_key_count++], key);
      }
    }
    // GPU Keys: Tg*
    else if (key[0] == 'T' && key[1] == 'g') {
      if (g_gpu_key_count < 64) {
        strcpy(g_gpu_keys[g_gpu_key_count++], key);
      }
    }
  }
}

static void loadAllTempSensors() {
  if (g_all_temp_sensor_count > 0)
    return;

  if (!g_smcConn)
    return;

  int totalKeys = SMCGetKeyCount(g_smcConn);
  for (int i = 0; i < totalKeys && g_all_temp_sensor_count < 512; i++) {
    char key[5];
    if (SMCGetKeyFromIndex(g_smcConn, i, key) != kIOReturnSuccess)
      continue;

    // Only temperature keys start with 'T'
    if (key[0] != 'T')
      continue;

    SMCKeyData_keyInfo_t keyInfo;
    if (SMCGetKeyInfo(g_smcConn, key, &keyInfo) != kIOReturnSuccess)
      continue;

    // Filter for 'flt ' type (1718383648)
    if (keyInfo.dataType != 1718383648)
      continue;

    // Include ALL temperature keys during enumeration — don't filter by value.
    // This list is cached (loadAllTempSensors returns early once populated),
    // and sensors that initially read 0°C (idle component) may warm up later.
    // Filtering happens at refresh time in samplePowerMetrics instead.
    float val = (float)SMCGetFloatValue(g_smcConn, key);
    if (val > 200)
      continue; // Skip clearly broken sensors (>200°C) at enumeration

    temp_sensor_t *sensor = &g_all_temp_sensors[g_all_temp_sensor_count];
    strcpy(sensor->key, key);
    snprintf(sensor->name, sizeof(sensor->name), "%s %c%c", tempSensorName(key),
             key[2], key[3]);
    sensor->value = val;
    g_all_temp_sensor_count++;
  }
}

// Diagnostic dump: print ALL SMC temperature keys, including filtered ones
void dumpAllSMCTemps(void) {
  if (!g_smcConn) {
    printf("SMC connection not available\n");
    return;
  }

  printf("=== Raw SMC Temperature Keys ===\n");
  printf("%-6s  %-20s  %8s  %s\n", "Key", "Category", "Value", "Status");
  printf("------  --------------------  --------  ------\n");

  int totalKeys = SMCGetKeyCount(g_smcConn);
  int tempKeyCount = 0;
  int filteredCount = 0;

  for (int i = 0; i < totalKeys; i++) {
    char key[5];
    if (SMCGetKeyFromIndex(g_smcConn, i, key) != kIOReturnSuccess)
      continue;

    if (key[0] != 'T')
      continue;

    SMCKeyData_keyInfo_t keyInfo;
    if (SMCGetKeyInfo(g_smcConn, key, &keyInfo) != kIOReturnSuccess)
      continue;

    // Only float type (flt )
    if (keyInfo.dataType != 1718383648)
      continue;

    float val = (float)SMCGetFloatValue(g_smcConn, key);
    const char *name = tempSensorName(key);
    const char *status = "✅ OK";

    // Match the category-aware thresholds used in samplePowerMetrics
    char k1 = key[1];
    int isSilicon = (k1 == 'p' || k1 == 'e' || k1 == 'f' ||
                     k1 == 'c' || k1 == 'C' || k1 == 'g' || k1 == 'R');
    float minTemp = isSilicon ? 10.0f : 0.0f;

    if (val <= minTemp) {
      if (isSilicon && val > 0) {
        status = "⚠ FILTERED (<10°C silicon)";
      } else {
        status = "⚠ FILTERED (≤0°C)";
      }
      filteredCount++;
    } else if (val > 200) {
      status = "⚠ FILTERED (>200°C)";
      filteredCount++;
    }

    printf("%-6s  %-20s  %7.1f°C  %s\n", key, name, val, status);
    tempKeyCount++;
  }

  printf("\nTotal SMC temperature keys: %d (filtered: %d, active: %d)\n",
         tempKeyCount, filteredCount, tempKeyCount - filteredCount);
  printf("Note: When HID sensors are available, SMC core keys (Tp/Te/Tf/Tg/TR) are\n");
  printf("      replaced by per-physical-core HID sensors for accuracy.\n");

  // Also print core configuration
  printf("\n=== Core Configuration ===\n");
  printf("CPU temp keys (Tp*/Te*): %d\n", g_cpu_key_count);
  printf("GPU temp keys (Tg*):     %d\n", g_gpu_key_count);
  for (int i = 0; i < g_cpu_key_count; i++) {
    float val = (float)SMCGetFloatValue(g_smcConn, g_cpu_keys[i]);
    printf("  CPU[%d] = %s  %.1f°C\n", i, g_cpu_keys[i], val);
  }
  for (int i = 0; i < g_gpu_key_count; i++) {
    float val = (float)SMCGetFloatValue(g_smcConn, g_gpu_keys[i]);
    printf("  GPU[%d] = %s  %.1f°C\n", i, g_gpu_keys[i], val);
  }
}

// Read fan data from SMC
static int readFanInfo(fan_info_t *fans, int maxFans) {
  if (!g_smcConn)
    return 0;

  // Read number of fans
  SMCKeyData_t val;
  if (SMCReadKey(g_smcConn, "FNum", &val) != kIOReturnSuccess)
    return 0;

  // FNum is typically a ui8 (1 byte)
  int fanCount = (unsigned char)val.bytes[0];
  if (fanCount > maxFans)
    fanCount = maxFans;

  for (int i = 0; i < fanCount; i++) {
    char key[5];
    fans[i].id = i;

    // Read actual RPM: F%dAc
    snprintf(key, sizeof(key), "F%dAc", i);
    fans[i].actualRPM = (int)SMCGetFloatValue(g_smcConn, key);

    // Read min RPM: F%dMn
    snprintf(key, sizeof(key), "F%dMn", i);
    fans[i].minRPM = (int)SMCGetFloatValue(g_smcConn, key);

    // Read max RPM: F%dMx
    snprintf(key, sizeof(key), "F%dMx", i);
    fans[i].maxRPM = (int)SMCGetFloatValue(g_smcConn, key);

    // Read target RPM: F%dTg
    snprintf(key, sizeof(key), "F%dTg", i);
    fans[i].targetRPM = (int)SMCGetFloatValue(g_smcConn, key);

    // Read mode: F%dMd (flt type — 0.0=auto, 1.0=forced)
    snprintf(key, sizeof(key), "F%dMd", i);
    fans[i].mode = (int)SMCGetFloatValue(g_smcConn, key);

    // Fan name — use index-based naming
    snprintf(fans[i].name, sizeof(fans[i].name), "Fan %d", i);
  }

  return fanCount;
}

// Fan control functions
int setFanForceTest(int enabled) {
  if (!g_smcConn)
    return -1;
  float val = enabled ? 1.0f : 0.0f;
  return (SMCSetFloat(g_smcConn, "Ftst", val) == kIOReturnSuccess) ? 0 : -1;
}

int setFanMode(int fanIndex, int mode) {
  if (!g_smcConn)
    return -1;
  char key[5];
  snprintf(key, sizeof(key), "F%dMd", fanIndex);
  float val = (float)mode;
  return (SMCSetFloat(g_smcConn, key, val) == kIOReturnSuccess) ? 0 : -1;
}

int setFanTarget(int fanIndex, int rpm) {
  if (!g_smcConn)
    return -1;

  // Read bounds for clamping
  char key[5];
  snprintf(key, sizeof(key), "F%dMn", fanIndex);
  int minRPM = (int)SMCGetFloatValue(g_smcConn, key);
  snprintf(key, sizeof(key), "F%dMx", fanIndex);
  int maxRPM = (int)SMCGetFloatValue(g_smcConn, key);

  // Clamp to hardware bounds
  if (rpm < minRPM)
    rpm = minRPM;
  if (maxRPM > 0 && rpm > maxRPM)
    rpm = maxRPM;

  snprintf(key, sizeof(key), "F%dTg", fanIndex);
  float val = (float)rpm;
  return (SMCSetFloat(g_smcConn, key, val) == kIOReturnSuccess) ? 0 : -1;
}

int resetFansToAuto() {
  if (!g_smcConn)
    return -1;

  // Clear force test mode
  setFanForceTest(0);

  // Read fan count
  SMCKeyData_t val;
  if (SMCReadKey(g_smcConn, "FNum", &val) != kIOReturnSuccess)
    return -1;

  int fanCount = (unsigned char)val.bytes[0];
  for (int i = 0; i < fanCount && i < 8; i++) {
    setFanMode(i, 0); // 0 = auto
  }
  return 0;
}

// Cached NVMe SMART temps — refreshed periodically, seeded by HID NAND fallback
static temp_sensor_t g_nvme_temps[16];
static int g_nvme_temp_count = 0;
static int g_nvme_smart_active = 0;

// Read per-core temperature sensors from IOHIDEventSystemClient.
// Returns the number of sensors written into the output array.
// Also reports per-category counts through output parameters.
static int readHIDCoreTempSensors(temp_sensor_t *out, int maxSensors,
                                  int *outEcount, int *outPcount,
                                  int *outScount, int *outGPUcount) {
  int count = 0;
  *outEcount = 0;
  *outPcount = 0;
  *outScount = 0;
  *outGPUcount = 0;

  IOHIDEventSystemClientRef client = getHIDClient();
  if (client == NULL) {
    return 0;
  }

  CFArrayRef services = IOHIDEventSystemClientCopyServices(client);
  if (services == NULL) {
    return 0;
  }

  // Track per-type sequential indices
  int eIdx = 0, pIdx = 0, sIdx = 0, gpuIdx = 0;
  
  int hidNvmeCount = 0;
  temp_sensor_t hidNvmeTemps[16];

  CFIndex svcCount = CFArrayGetCount(services);
  for (CFIndex i = 0; i < svcCount && count < maxSensors; i++) {
    IOHIDServiceClientRef service =
        (IOHIDServiceClientRef)CFArrayGetValueAtIndex(services, i);
    if (service == NULL)
      continue;

    CFStringRef productRef =
        IOHIDServiceClientCopyProperty(service, CFSTR("Product"));
    if (productRef == NULL)
      continue;

    char product[512] = {0};
    CFStringGetCString(productRef, product, sizeof(product),
                       kCFStringEncodingUTF8);

    IOHIDEventRef event = IOHIDServiceClientCopyEvent(
        service, kIOHIDEventTypeTemperature, 0, 0);
    if (event == NULL) {
      CFRelease(productRef);
      continue;
    }

    double temp =
        IOHIDEventGetFloatValue(event, kIOHIDEventTypeTemperature << 16);
    CFRelease(event);
    CFRelease(productRef);

    if (temp <= 10 || temp > 150)
      continue;  // Apply same silicon minimum

    // Classify by product name
    const char *category = NULL;
    int *idxPtr = NULL;

    if (strstr(product, "eACC") != NULL) {
      category = "CPU E-Core";
      idxPtr = &eIdx;
    } else if (strstr(product, "sACC") != NULL) {
      // M5 Pro/Max/Ultra Super cores — check before pACC since
      // sACC won't match pACC, but order matters for clarity
      category = "CPU S-Core";
      idxPtr = &sIdx;
    } else if (strstr(product, "pACC") != NULL ||
               strstr(product, "mACC") != NULL) {
      // M-cores (Medium, M5) are treated as P-cores for display,
      // consistent with native_stats.go core classification
      category = "CPU P-Core";
      idxPtr = &pIdx;
    } else if (strstr(product, "GPU") != NULL) {
      category = "GPU";
      idxPtr = &gpuIdx;
    }

    if (category != NULL && idxPtr != NULL) {
      temp_sensor_t *s = &out[count];
      // Derive key prefix char from category
      char keyChar = 'g'; // default for GPU
      if (strcmp(category, "CPU E-Core") == 0) keyChar = 'e';
      else if (strcmp(category, "CPU P-Core") == 0) keyChar = 'p';
      else if (strcmp(category, "CPU S-Core") == 0) keyChar = 's';
      // Use synthetic key: He00, Hp00, Hs00, Hg00 (H prefix = HID source)
      snprintf(s->key, sizeof(s->key), "H%c%02X", keyChar, *idxPtr);
      snprintf(s->name, sizeof(s->name), "%s %02d", category, *idxPtr);
      s->value = (float)temp;
      (*idxPtr)++;
      count++;
    } else if (strstr(product, "NAND") != NULL && strstr(product, "temp") != NULL) {
      if (hidNvmeCount < 16) {
        temp_sensor_t *s = &hidNvmeTemps[hidNvmeCount];
        snprintf(s->key, sizeof(s->key), "Nv%02X", hidNvmeCount);
        snprintf(s->name, sizeof(s->name), "NVMe %s", product);
        s->value = (float)temp;
        hidNvmeCount++;
      }
    }
  }

  if (!g_nvme_smart_active && hidNvmeCount > 0) {
    memcpy(g_nvme_temps, hidNvmeTemps, hidNvmeCount * sizeof(temp_sensor_t));
    g_nvme_temp_count = hidNvmeCount;
  }

  // Report per-category counts
  *outEcount = eIdx;
  *outPcount = pIdx;
  *outScount = sIdx;
  *outGPUcount = gpuIdx;

  CFRelease(services);
  return count;
}

static float readSocTemperature(float *outCpuTemp, float *outGpuTemp) {
  float cpuSum = 0;
  int cpuCount = 0;
  float gpuSum = 0;
  int gpuCount = 0;

  // Try SMC First
  if (g_smcConn) {
    for (int i = 0; i < g_cpu_key_count; i++) {
      float val = (float)SMCGetFloatValue(g_smcConn, g_cpu_keys[i]);
      if (val > 0) {
        cpuSum += val;
        cpuCount++;
      }
    }
    for (int i = 0; i < g_gpu_key_count; i++) {
      float val = (float)SMCGetFloatValue(g_smcConn, g_gpu_keys[i]);
      if (val > 0) {
        gpuSum += val;
        gpuCount++;
      }
    }
  }

  // Fallback to HID if SMC failed — reuse cached client
  if (cpuCount == 0 || gpuCount == 0) {
    IOHIDEventSystemClientRef client = getHIDClient();
    if (client != NULL) {
      CFArrayRef services = IOHIDEventSystemClientCopyServices(client);
      if (services != NULL) {
        CFIndex count = CFArrayGetCount(services);
        for (CFIndex i = 0; i < count; i++) {
          IOHIDServiceClientRef service =
              (IOHIDServiceClientRef)CFArrayGetValueAtIndex(services, i);
          if (service == NULL)
            continue;

          CFStringRef productRef =
              IOHIDServiceClientCopyProperty(service, CFSTR("Product"));
          if (productRef == NULL)
            continue;

          char product[512] = {0};
          CFStringGetCString(productRef, product, sizeof(product),
                             kCFStringEncodingUTF8);

          IOHIDEventRef event = IOHIDServiceClientCopyEvent(
              service, kIOHIDEventTypeTemperature, 0, 0);
          if (event == NULL) {
            CFRelease(productRef);
            continue;
          }

          double temp =
              IOHIDEventGetFloatValue(event, kIOHIDEventTypeTemperature << 16);
          CFRelease(event);
          CFRelease(productRef);

          if (temp > 0 && temp < 150) {
            if (strstr(product, "PMU tdie") != NULL ||
                strstr(product, "pACC") != NULL ||
                strstr(product, "eACC") != NULL) {
              if (cpuCount == 0) { // Only use HID if SMC didn't find anything
                cpuSum += temp;
                cpuCount++;
              }
            } else if (strstr(product, "GPU") != NULL) {
              if (gpuCount == 0) {
                gpuSum += temp;
                gpuCount++;
              }
            }
          }
        }
        CFRelease(services);
      }
    }
  }

  if (cpuCount > 0)
    *outCpuTemp = cpuSum / cpuCount;
  if (gpuCount > 0)
    *outGpuTemp = gpuSum / gpuCount;

  // Return max of both as "SoC Temp" for backward compatibility if needed
  return (*outCpuTemp > *outGpuTemp) ? *outCpuTemp : *outGpuTemp;
}

// === NVMe SMART Temperature Reading ===
// Uses Apple's private NVMeSMARTLib.plugin to read SMART health log
// from NVMe drives (internal + external Thunderbolt SSDs).
// Temperature is at bytes 1-2 of the standard NVMe SMART log (Kelvin, LE16).

// NVMe SMART data structure (NVM Express Spec 5.10.1.2)
typedef struct {
  uint8_t  critical_warning;
  uint8_t  temperature[2];     // Composite temp in Kelvin (little-endian)
  uint8_t  available_spare;
  uint8_t  available_spare_threshold;
  uint8_t  percentage_used;
  uint8_t  reserved1[26];
  uint8_t  data_units_read[16];
  uint8_t  data_units_written[16];
  uint8_t  host_read_commands[16];
  uint8_t  host_write_commands[16];
  uint8_t  controller_busy_time[16];
  uint8_t  power_cycles[16];
  uint8_t  power_on_hours[16];
  uint8_t  unsafe_shutdowns[16];
  uint8_t  media_errors[16];
  uint8_t  num_err_log_entries[16];
  uint8_t  reserved2[320];
} NVMeSMARTData;

// IONVMeSMARTInterface vtable (matches NVMeSMARTLib.plugin / smartmontools)
typedef struct IONVMeSMARTInterface {
  IUNKNOWN_C_GUTS;
  UInt16 version;
  UInt16 revision;
  IOReturn (*SMARTReadData)(void *interface, NVMeSMARTData *data);
  IOReturn (*GetIdentifyData)(void *interface, void *data, unsigned int ns);
  UInt64 reserved0;
  UInt64 reserved1;
  IOReturn (*GetLogPage)(void *interface, void *data, unsigned int logPageId, unsigned int numDWords);
} IONVMeSMARTInterface;


static void readNVMeSMARTTemps(void) {
  // Use local storage — only update global cache if we get results.
  // This preserves cached data when the SMART plugin intermittently fails
  // (common on Apple Silicon embedded NVMe where the user client is unstable).
  temp_sensor_t localTemps[16];
  int localCount = 0;

  // kIONVMeSMARTUserClientTypeID
  CFUUIDRef smartFactory = CFUUIDGetConstantUUIDWithBytes(NULL,
      0xAA, 0x0F, 0xA6, 0xF9, 0xC2, 0xD6, 0x45, 0x7F,
      0xB1, 0x0B, 0x59, 0xA1, 0x32, 0x53, 0x29, 0x2F);
  // kIONVMeSMARTInterfaceID
  CFUUIDRef smartInterfaceID = CFUUIDGetConstantUUIDWithBytes(NULL,
      0xCC, 0xD1, 0xDB, 0x19, 0xFD, 0x9A, 0x4D, 0xAF,
      0xBF, 0x95, 0x12, 0x45, 0x4B, 0x23, 0x0A, 0xB6);

  // Match all block storage devices, then filter for NVMe SMART support
  CFMutableDictionaryRef match = IOServiceMatching("IOBlockStorageDevice");
  io_iterator_t iter;
  kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, match, &iter);
  if (kr != kIOReturnSuccess) return;

  io_service_t svc;
  while ((svc = IOIteratorNext(iter)) != 0 && localCount < 16) {
    // Check for "NVMe SMART Capable" property
    CFTypeRef smartCapable = IORegistryEntryCreateCFProperty(
        svc, CFSTR("NVMe SMART Capable"), kCFAllocatorDefault, 0);
    if (!smartCapable) {
      IOObjectRelease(svc);
      continue;
    }
    int isSmart = 0;
    if (CFGetTypeID(smartCapable) == CFBooleanGetTypeID()) {
      isSmart = CFBooleanGetValue((CFBooleanRef)smartCapable) ? 1 : 0;
    } else if (CFGetTypeID(smartCapable) == CFStringGetTypeID()) {
      char buf[8];
      CFStringGetCString((CFStringRef)smartCapable, buf, sizeof(buf), kCFStringEncodingUTF8);
      isSmart = (strcasecmp(buf, "Yes") == 0) ? 1 : 0;
    }
    CFRelease(smartCapable);
    if (!isSmart) {
      IOObjectRelease(svc);
      continue;
    }

    // Get model name from "Device Characteristics" dictionary
    char model[64] = "NVMe";
    CFDictionaryRef devChars = IORegistryEntryCreateCFProperty(
        svc, CFSTR("Device Characteristics"), kCFAllocatorDefault, 0);
    if (devChars && CFGetTypeID(devChars) == CFDictionaryGetTypeID()) {
      CFStringRef prodName = CFDictionaryGetValue(devChars, CFSTR("Product Name"));
      if (prodName && CFGetTypeID(prodName) == CFStringGetTypeID()) {
        CFStringGetCString(prodName, model, sizeof(model), kCFStringEncodingUTF8);
        size_t len = strlen(model);
        while (len > 0 && model[len - 1] == ' ') model[--len] = '\0';
      }
    }
    if (devChars) CFRelease(devChars);

    // Create plugin interface for SMART reading
    IOCFPlugInInterface **plugin = NULL;
    SInt32 score = 0;
    kr = IOCreatePlugInInterfaceForService(svc, smartFactory,
                                           kIOCFPlugInInterfaceID,
                                           &plugin, &score);
    if (kr != kIOReturnSuccess || !plugin) {
      IOObjectRelease(svc);
      continue;
    }

    // Query for the NVMe SMART interface
    IONVMeSMARTInterface **smartInterface = NULL;
    HRESULT res = (*plugin)->QueryInterface(plugin,
        CFUUIDGetUUIDBytes(smartInterfaceID), (LPVOID *)&smartInterface);
    (*plugin)->Release(plugin);

    if (res != S_OK || !smartInterface) {
      IOObjectRelease(svc);
      continue;
    }

    // Read SMART data
    NVMeSMARTData smartData;
    memset(&smartData, 0, sizeof(smartData));
    IOReturn readResult = (*smartInterface)->SMARTReadData(smartInterface, &smartData);

    if (readResult == kIOReturnSuccess) {
      // Temperature: bytes 1-2, little-endian uint16 in Kelvin
      uint16_t tempK = (uint16_t)smartData.temperature[0] |
                       ((uint16_t)smartData.temperature[1] << 8);
      if (tempK > 0 && tempK < 1000) {
        float tempC = (float)tempK - 273.15f;
        if (tempC > 0.0f && tempC < 150.0f) {
          temp_sensor_t *s = &localTemps[localCount];
          snprintf(s->key, sizeof(s->key), "Nv%02X", localCount);
          snprintf(s->name, sizeof(s->name), "NVMe %.50s", model);
          s->value = tempC;
          localCount++;
        }
      }
    }

    (*smartInterface)->Release(smartInterface);
    IOObjectRelease(svc);
  }
  IOObjectRelease(iter);

  // Only update global cache if we got results.
  // If all reads failed, preserve previous cached data.
  if (localCount > 0) {
    memcpy(g_nvme_temps, localTemps, localCount * sizeof(temp_sensor_t));
    g_nvme_temp_count = localCount;
    g_nvme_smart_active = 1;
  }
}

PowerMetrics samplePowerMetrics(int durationMs) {
  PowerMetrics metrics = {0};

  if (g_subscription == NULL || g_channels == NULL) {
    if (initIOReport() != 0) {
      return metrics;
    }
  }

  @autoreleasepool {

  CFDictionaryRef sample1 =
      IOReportCreateSamples(g_subscription, g_channels, NULL);

  if (sample1 == NULL)
    return metrics;

  usleep(durationMs * 1000);

  CFDictionaryRef sample2 =
      IOReportCreateSamples(g_subscription, g_channels, NULL);

  if (sample2 == NULL) {
    CFRelease(sample1);
    return metrics;
  }

  CFDictionaryRef delta = IOReportCreateSamplesDelta(sample1, sample2, NULL);
  CFRelease(sample1);
  CFRelease(sample2);

  if (delta == NULL)
    return metrics;

  CFArrayRef channels = CFDictionaryGetValue(delta, CFSTR("IOReportChannels"));
  if (channels == NULL) {
    CFRelease(delta);
    return metrics;
  }

  CFIndex count = CFArrayGetCount(channels);
  // Temporary accumulators for CPU cluster metrics.
  // We accumulate here first and assign to metrics after the loop,
  // because on M5+ chips the channel iteration order is not guaranteed
  // (PCPU may appear before MCPU0/MCPU1).
  double eClusterActive = 0, pClusterActive = 0, sClusterActive = 0;
  int eClusterFreq = 0, pClusterFreq = 0, sClusterFreq = 0;
  // M-cluster (M5+) accumulators
  double mClusterActiveSum = 0;
  int mClusterFreqMax = 0;
  int mClusterCount = 0;
  // PCPU accumulator (may be P-cluster on M1-M4, or S-cluster on M5+)
  double pcpuActive = 0;
  int pcpuFreq = 0;
  int hasPCPU = 0;

  int64_t pmpDramReadBytes = 0;
  int64_t pmpDramWriteBytes = 0;

  for (CFIndex i = 0; i < count; i++) {
    CFDictionaryRef item = (CFDictionaryRef)CFArrayGetValueAtIndex(channels, i);
    if (item == NULL)
      continue;

    CFStringRef groupRef = IOReportChannelGetGroup(item);
    CFStringRef channelRef = IOReportChannelGetChannelName(item);

    if (groupRef == NULL || channelRef == NULL)
      continue;

    // Extract group and channel to C-strings ONCE per channel.
    // This eliminates all temporary CFStringRef allocations from
    // cfStringMatch/Contains/StartsWith in the hot loop.
    char grp[64] = {0};
    char chn[256] = {0};
    CFStringGetCString(groupRef, grp, sizeof(grp), kCFStringEncodingUTF8);
    CFStringGetCString(channelRef, chn, sizeof(chn), kCFStringEncodingUTF8);

    if (strcmp(grp, "Energy Model") == 0) {
      CFStringRef unitRef = IOReportChannelGetUnitLabel(item);
      int64_t val = IOReportSimpleGetIntegerValue(item, 0);
      double watts = energyToWatts(val, unitRef, (double)durationMs);

      if (strstr(chn, "CPU Energy") != NULL) {
        metrics.cpuPower += watts;
      } else if (strcmp(chn, "GPU Energy") == 0) {
        metrics.gpuPower += watts;
      } else if (strncmp(chn, "ANE", 3) == 0) {
        metrics.anePower += watts;
      } else if (strncmp(chn, "DRAM", 4) == 0) {
        metrics.dramPower += watts;
      } else if (strncmp(chn, "GPU SRAM", 8) == 0) {
        metrics.gpuSramPower += watts;
      }
    } else if (strcmp(grp, "GPU Stats") == 0) {
      CFStringRef subgroupRef = IOReportChannelGetSubGroup(item);
      if (subgroupRef != NULL) {
        char sub[64] = {0};
        CFStringGetCString(subgroupRef, sub, sizeof(sub), kCFStringEncodingUTF8);
        if (strcmp(sub, "GPU Performance States") == 0 &&
            strcmp(chn, "GPUPH") == 0) {
          int32_t stateCount = IOReportStateGetCount(item);
          int64_t totalTime = 0;
          int64_t activeTime = 0;
          double weightedFreq = 0;
          int activeStateIdx = 0;

          for (int32_t s = 0; s < stateCount; s++) {
            int64_t residency = IOReportStateGetResidency(item, s);
            CFStringRef stateName = IOReportStateGetNameForIndex(item, s);
            totalTime += residency;

            if (stateName != NULL) {
              char sn[32] = {0};
              CFStringGetCString(stateName, sn, sizeof(sn), kCFStringEncodingUTF8);
              if (strcmp(sn, "OFF") != 0 && strcmp(sn, "IDLE") != 0 &&
                  strcmp(sn, "DOWN") != 0) {
                activeTime += residency;
                if (g_gpu_freq_count > 0 && activeStateIdx < g_gpu_freq_count) {
                  weightedFreq += (double)g_gpu_freqs[activeStateIdx] * residency;
                }
                activeStateIdx++;
              }
            }
          }

          if (totalTime > 0) {
            metrics.gpuActive = (double)activeTime / (double)totalTime * 100.0;
          }
          if (activeTime > 0 && g_gpu_freq_count > 0) {
            metrics.gpuFreqMHz = (int)(weightedFreq / activeTime);
          }
        }
      }
    } else if (strcmp(grp, "CPU Stats") == 0) {
      CFStringRef subgroupRef = IOReportChannelGetSubGroup(item);
      if (subgroupRef != NULL) {
        char sub[64] = {0};
        CFStringGetCString(subgroupRef, sub, sizeof(sub), kCFStringEncodingUTF8);
        if (strcmp(sub, "CPU Complex Performance States") != 0)
          continue;

        // Check MCPU first — on M5+ chips, MCPU0/MCPU1 contain "CPU0"/"CPU1"
        // which would falsely match the E-cluster/P-cluster fallbacks.
        int isMCluster = (strstr(chn, "MCPU") != NULL);
        int isSCluster = (strstr(chn, "SCPU") != NULL);

        // E-Cluster: ECPU (M1-M4), or legacy CPU0 fallback (but NOT MCPU0)
        int isECluster = (strstr(chn, "ECPU") != NULL) ||
                         (!isMCluster && strcmp(chn, "CPU0") == 0);
        // P-Cluster: PCPU (all chips), or legacy CPU1 fallback (but NOT MCPU1)
        int isPCluster = (strstr(chn, "PCPU") != NULL) ||
                         (!isMCluster && strcmp(chn, "CPU1") == 0);

        if (isECluster || isPCluster || isSCluster || isMCluster) {
          int32_t stateCount = IOReportStateGetCount(item);
          int64_t totalTime = 0;
          int64_t activeTime = 0;
          double weightedFreq = 0;

          for (int32_t s = 0; s < stateCount; s++) {
            int64_t residency = IOReportStateGetResidency(item, s);
            CFStringRef stateName = IOReportStateGetNameForIndex(item, s);
            totalTime += residency;

            if (stateName != NULL) {
              char nameBuf[64] = {0};
              CFStringGetCString(stateName, nameBuf, sizeof(nameBuf),
                                 kCFStringEncodingUTF8);

              if (strcmp(nameBuf, "OFF") != 0 && strcmp(nameBuf, "IDLE") != 0) {
                activeTime += residency;

                int freq = 0;

                // Heuristic for "V#..." format
                if (nameBuf[0] == 'V') {
                  int vIdx = -1;
                  // Parse index after 'V'
                  if (sscanf(nameBuf, "V%d", &vIdx) == 1 && vIdx >= 0) {
                    if (isECluster && vIdx < g_ecpu_freq_count) {
                      freq = g_ecpu_freqs[vIdx];
                    } else if (isMCluster && vIdx < g_scpu_freq_count) {
                      // M5+ M-cluster: use dedicated table (voltage-states22-sram)
                      freq = g_scpu_freqs[vIdx];
                    } else if (isMCluster && vIdx < g_pcpu_freq_count) {
                      // M-cluster fallback: use P table if M table not loaded
                      freq = g_pcpu_freqs[vIdx];
                    } else if (isPCluster && vIdx < g_pcpu_freq_count) {
                      freq = g_pcpu_freqs[vIdx];
                    } else if (isSCluster && vIdx < g_scpu_freq_count) {
                      freq = g_scpu_freqs[vIdx];
                    }
                  }
                }

                // Fallback to searching for explicit number in string
                if (freq == 0) {
                  char *numStart = NULL;
                  for (int c = 0; nameBuf[c]; c++) {
                    if (nameBuf[c] >= '0' && nameBuf[c] <= '9') {
                      numStart = &nameBuf[c];
                      break;
                    }
                  }
                  if (numStart) {
                    freq = atoi(numStart);
                  }
                }

                // Sanity check freq (usually > 300MHz)
                if (freq > 0) {
                  weightedFreq += (double)freq * residency;
                }
              }
            }
          }

          if (totalTime > 0) {
            double activePercent =
                (double)activeTime / (double)totalTime * 100.0;
            int avgFreq = 0;
            if (activeTime > 0) {
              avgFreq = (int)(weightedFreq / activeTime);
            }

            if (isECluster) {
              // Take max across E-clusters (multi-die chips have E0, E1)
              if (avgFreq > eClusterFreq) {
                eClusterFreq = avgFreq;
              }
              if (activePercent > eClusterActive) {
                eClusterActive = activePercent;
              }
            } else if (isMCluster) {
              // M5+ Medium/Performance tier — accumulate across MCPU0, MCPU1
              mClusterActiveSum += activePercent;
              mClusterCount++;
              if (avgFreq > mClusterFreqMax) {
                mClusterFreqMax = avgFreq;
              }
            } else if (isPCluster) {
              // PCPU — on M1-M4 this is the Performance cluster,
              // on M5+ this is the Super cluster. Take max across clusters
              // (multi-die chips have P0, P1, P2, P3).
              if (avgFreq > pcpuFreq) {
                pcpuFreq = avgFreq;
              }
              if (activePercent > pcpuActive) {
                pcpuActive = activePercent;
              }
              hasPCPU = 1;
            } else if (isSCluster) {
              sClusterActive = activePercent;
              sClusterFreq = avgFreq;
            }
          }
        }
      }
    } else if (strcmp(grp, "AMC Stats") == 0) {
      // Sum memory bandwidth from non-DCS channels to avoid double counting.
      // DCS (DRAM Command Scheduler) channels are a subset of the total.
      // Works on M-series chips (M1, M2, M3, M4, M5, etc.).
      if (strstr(chn, "DCS") == NULL) {
        int64_t val = IOReportSimpleGetIntegerValue(item, 0);
        if (strstr(chn, "RD") != NULL) {
          metrics.dramReadBytes += val;
        } else if (strstr(chn, "WR") != NULL) {
          metrics.dramWriteBytes += val;
        }
      }
    } else if (strcmp(grp, "PMP") == 0) {
      // PMP group provides DRAM bandwidth on A-series chips (A18 Pro, etc.)
      // where AMC Stats channels exist but produce no delta data.
      char sub[64] = {0};
      CFStringRef subgroupRef = IOReportChannelGetSubGroup(item);
      if (subgroupRef != NULL) {
        CFStringGetCString(subgroupRef, sub, sizeof(sub), kCFStringEncodingUTF8);
      }
      if (strcmp(sub, "DRAM BW") == 0) {
        int64_t val = IOReportSimpleGetIntegerValue(item, 0);
        if (val > 0) {
          if (strstr(chn, "RD") != NULL) {
            pmpDramReadBytes += val;
          } else if (strstr(chn, "WR") != NULL) {
            pmpDramWriteBytes += val;
          }
        }
      }
    }
  }

  // Post-loop: assign accumulated CPU cluster metrics to final metrics.
  // On M5+ chips (mClusterCount > 0): MCPU = Performance (pCluster), PCPU = Super (sCluster).
  // On M1-M4 chips (mClusterCount == 0): PCPU = Performance (pCluster), ECPU = Efficiency (eCluster).
  metrics.eClusterActive = eClusterActive;
  metrics.eClusterFreqMHz = eClusterFreq;

  if (mClusterCount > 0) {
    // M5+ chip: MCPU average -> pCluster, PCPU -> sCluster
    metrics.pClusterActive = mClusterActiveSum / mClusterCount;
    metrics.pClusterFreqMHz = mClusterFreqMax;
    if (hasPCPU) {
      metrics.sClusterActive = pcpuActive;
      metrics.sClusterFreqMHz = pcpuFreq;
    } else {
      metrics.sClusterActive = sClusterActive;
      metrics.sClusterFreqMHz = sClusterFreq;
    }
  } else {
    // M1-M4: PCPU -> pCluster, SCPU -> sCluster (if present)
    if (hasPCPU) {
      metrics.pClusterActive = pcpuActive;
      metrics.pClusterFreqMHz = pcpuFreq;
    }
    metrics.sClusterActive = sClusterActive;
    metrics.sClusterFreqMHz = sClusterFreq;
  }

  // Fallback: use PMP DRAM BW data when AMC Stats produces no bandwidth data.
  if (metrics.dramReadBytes == 0 && metrics.dramWriteBytes == 0) {
    metrics.dramReadBytes = pmpDramReadBytes;
    metrics.dramWriteBytes = pmpDramWriteBytes;
  }

  // Fallback: estimate DRAM BW from DRAM power (no root needed, M5+ only).
  // DRAM power = static (leakage/refresh) + dynamic (data transfer).
  // Only dynamic power correlates with bandwidth, so subtract idle baseline.
  // BW = max(0, (current_power - idle_power)) × calibration_constant
  // The calibration_constant is auto-derived at startup (see calibrateDramBwFromPower).
  // This path only fires on M5+ where AMC Stats is kernel-blocked.
  // On M1-M4/A-series, AMC Stats/PMP provides direct byte counters.
  if (metrics.dramReadBytes == 0 && metrics.dramWriteBytes == 0 &&
      metrics.dramPower > 0.001) {
    // Subtract static/idle power — only dynamic power indicates data transfer
    double activePower = metrics.dramPower - g_dramIdlePowerW;
    if (activePower < 0) activePower = 0;
    double dramBwGBs = activePower * g_dramGBsPerWatt;
    // Convert to bytes for this sample interval
    double sampleSec = (double)durationMs / 1000.0;
    int64_t totalBytes = (int64_t)(dramBwGBs * 1e9 * sampleSec);
    // Split evenly between read and write (power can't distinguish direction)
    metrics.dramReadBytes = totalBytes / 2;
    metrics.dramWriteBytes = totalBytes / 2;
  }

  // Fallback: use kperf PMU counters for DRAM BW (requires root).
  if (metrics.dramReadBytes == 0 && metrics.dramWriteBytes == 0 && g_kperf_active) {
    int64_t kperfRd = 0, kperfWr = 0;
    readKperfDramBW(&kperfRd, &kperfWr);
    metrics.dramReadBytes = kperfRd;
    metrics.dramWriteBytes = kperfWr;
  }

  // Defer readSocTemperature — try to derive CPU/GPU temps from HID per-core data first.
  // This avoids a redundant HID service enumeration on systems where HID provides good data.

  if (g_smcConn) {
    metrics.systemPower = SMCGetFloatValue(g_smcConn, "PSTR");
  }

  // Read fan data
  metrics.fanCount = readFanInfo(metrics.fans, 8);

  // Read all temperature sensors
  loadAllTempSensors();

  // Strategy: Validate HID per-core data against expected core counts.
  // Use HID for a category ONLY if it provides >= expected physical cores.
  // Otherwise fall back to SMC (with category-aware 10°C threshold).
  // This prevents M2 Max's incomplete/garbage HID from replacing good SMC data.

  // Step 1: Read HID per-core sensors and get per-category counts
  temp_sensor_t hidSensors[64];
  int hidEcount = 0, hidPcount = 0, hidScount = 0, hidGPUcount = 0;
  int hidTotal = readHIDCoreTempSensors(hidSensors, 64,
                                         &hidEcount, &hidPcount,
                                         &hidScount, &hidGPUcount);

  // Step 2: Decide per-category: use HID or SMC?
  // Use HID only if count >= expected physical cores (validation)
  int useHidEcore = (hidEcount >= g_expected_ecores && g_expected_ecores > 0);
  int useHidPcore = (hidPcount >= g_expected_pcores && g_expected_pcores > 0);
  int useHidScore = (hidScount >= g_expected_scores && g_expected_scores > 0);
  // GPU: use HID if it has any sensors (no expected count to compare)
  int useHidGPU = (hidGPUcount > 0);

  // Derive CPU/GPU temps from HID data when available.
  // If HID covers both CPU and GPU, skip the expensive readSocTemperature call.
  int gotCpuFromHID = 0, gotGpuFromHID = 0;
  if (useHidEcore || useHidPcore || useHidScore) {
    // Average all CPU core temps from HID
    float cpuSum = 0;
    int cpuCnt = 0;
    for (int i = 0; i < hidTotal; i++) {
      char hk = hidSensors[i].key[1];
      if ((hk == 'e' && useHidEcore) || (hk == 'p' && useHidPcore) ||
          (hk == 's' && useHidScore)) {
        cpuSum += hidSensors[i].value;
        cpuCnt++;
      }
    }
    if (cpuCnt > 0) {
      metrics.cpuTemp = cpuSum / cpuCnt;
      gotCpuFromHID = 1;
    }
  }
  if (useHidGPU) {
    float gpuSum = 0;
    int gpuCnt = 0;
    for (int i = 0; i < hidTotal; i++) {
      if (hidSensors[i].key[1] == 'g') {
        gpuSum += hidSensors[i].value;
        gpuCnt++;
      }
    }
    if (gpuCnt > 0) {
      metrics.gpuTemp = gpuSum / gpuCnt;
      gotGpuFromHID = 1;
    }
  }

  // Only call readSocTemperature if HID didn't provide both CPU and GPU temps
  float fallbackCpuTemp = 0.0f;
  float fallbackGpuTemp = 0.0f;
  if (!gotCpuFromHID || !gotGpuFromHID) {
    metrics.socTemp = readSocTemperature(&fallbackCpuTemp, &fallbackGpuTemp);
    if (!gotCpuFromHID && fallbackCpuTemp > 0) metrics.cpuTemp = fallbackCpuTemp;
    if (!gotGpuFromHID && fallbackGpuTemp > 0) metrics.gpuTemp = fallbackGpuTemp;
  } else {
    metrics.socTemp = (metrics.cpuTemp > metrics.gpuTemp) ? metrics.cpuTemp : metrics.gpuTemp;
  }

  int validSensorCount = 0;

  // Step 3: Add validated HID sensors for categories that passed validation
  if (hidTotal > 0) {
    for (int i = 0; i < hidTotal && validSensorCount < 512; i++) {
      char hk = hidSensors[i].key[1]; // e, p, s, or g
      int include = 0;
      if (hk == 'e' && useHidEcore) include = 1;
      else if (hk == 'p' && useHidPcore) include = 1;
      else if (hk == 's' && useHidScore) include = 1;
      else if (hk == 'g' && useHidGPU) include = 1;
      if (include) {
        metrics.temps[validSensorCount++] = hidSensors[i];
      }
    }
  }

  // Step 4: Add SMC sensors, skipping categories already covered by HID.
  // Tiered caching: slow-changing sensors (Ambient, Board, SSD, VRM, etc.)
  // are only read from SMC every 5th tick to reduce IPC overhead.
  static int smcTempTick = 0;
  int refreshSlowSensors = (smcTempTick % 5 == 0);  // Refresh every 5th tick
  smcTempTick++;

  for (int i = 0; i < g_all_temp_sensor_count && validSensorCount < 512; i++) {
    char k1 = g_all_temp_sensors[i].key[1];

    // Check if this SMC key's category is already covered by HID
    int coveredByHID = 0;
    if ((k1 == 'e') && useHidEcore) coveredByHID = 1;
    else if ((k1 == 'p' || k1 == 'f') && useHidPcore) coveredByHID = 1;
    else if ((k1 == 's') && useHidScore) coveredByHID = 1;
    else if ((k1 == 'g' || k1 == 'R') && useHidGPU) coveredByHID = 1;

    if (coveredByHID) continue;

    // Classify: silicon core sensors change rapidly, environmental sensors don't
    int isCoreKey = (k1 == 'p' || k1 == 'e' || k1 == 'f' || k1 == 's' ||
                     k1 == 'c' || k1 == 'C' || k1 == 'g' || k1 == 'R');
    int isSlowSensor = !isCoreKey;  // Ambient, Board, SSD, VRM, etc.

    float v;
    if (isSlowSensor && !refreshSlowSensors && g_all_temp_sensors[i].value > 0.0f) {
      // Use cached value for slow-changing sensors between refreshes
      v = g_all_temp_sensors[i].value;  // Last known value
    } else if (g_smcConn) {
      v = (float)SMCGetFloatValue(g_smcConn, g_all_temp_sensors[i].key);
      g_all_temp_sensors[i].value = v;  // Update cached value
    } else {
      v = g_all_temp_sensors[i].value;
    }

    float minTemp = 0.0f;
    if (isCoreKey) {
      minTemp = 10.0f;
    }

    if (v > minTemp && v <= 200) {
      metrics.temps[validSensorCount] = g_all_temp_sensors[i];
      metrics.temps[validSensorCount].value = v;
      validSensorCount++;
    }
  }

  // Step 5: Append NVMe SMART temperatures.
  // Uses the same slow-sensor tick as SMC environmental sensors.
  if (refreshSlowSensors) {
    readNVMeSMARTTemps();
  }
  for (int i = 0; i < g_nvme_temp_count && validSensorCount < 512; i++) {
    metrics.temps[validSensorCount++] = g_nvme_temps[i];
  }

  metrics.tempSensorCount = validSensorCount;

  CFRelease(delta);

  } // @autoreleasepool

  return metrics;
}

void cleanupIOReport() {
  if (g_channels != NULL) {
    CFRelease(g_channels);
    g_channels = NULL;
  }
  g_subscription = NULL;
  if (g_smcConn) {
    SMCClose(g_smcConn);
    g_smcConn = 0;
  }
  // Clean up cached HID client
  if (g_hidClient) {
    CFRelease(g_hidClient);
    g_hidClient = NULL;
  }
  if (g_hidMatching) {
    CFRelease(g_hidMatching);
    g_hidMatching = NULL;
  }
  // Clean up kperf
  if (g_kperf_active && g_forceCtrs) {
    g_forceCtrs(0);
    g_kperf_active = 0;
  }
  if (g_kperf_prev) {
    free(g_kperf_prev);
    g_kperf_prev = NULL;
  }
}

int getThermalState() {
  @autoreleasepool {
    NSProcessInfo *info = [NSProcessInfo processInfo];
    return (int)[info thermalState];
  }
}

void debugMonitorChannels(int durationMs) { (void)durationMs; }
