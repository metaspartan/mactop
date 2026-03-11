// Copyright (c) 2024-2026 Carsen Klock under MIT License
// ioreport.m - Objective-C implementation for IOReport power/thermal metrics

#include "smc.h"
#import <CoreFoundation/CoreFoundation.h>
#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>
#import <IOKit/IOKitLib.h>
#include <mach/mach_host.h>
#include <mach/mach_init.h>
#include <mach/processor_info.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

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
static temp_sensor_t g_all_temp_sensors[128];
static int g_all_temp_sensor_count = 0;

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
    uint32_t freqMHz = freq / 1000000;
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

        CFDataRef eData = (CFDataRef)CFDictionaryGetValue(
            properties, CFSTR("voltage-states1-sram"));
        if (eData != NULL) {
          parseFreqData(eData, g_ecpu_freqs, &g_ecpu_freq_count);
        }

        CFDataRef pData = (CFDataRef)CFDictionaryGetValue(
            properties, CFSTR("voltage-states5-sram"));
        if (pData != NULL) {
          parseFreqData(pData, g_pcpu_freqs, &g_pcpu_freq_count);
        } else {
          // Try alternate for P-Cluster if 5 is missing (unlikely on M1/M2, but
          // safe)
          pData = (CFDataRef)CFDictionaryGetValue(
              properties, CFSTR("voltage-states-sram")); // fallback?
          if (pData != NULL) {
            parseFreqData(pData, g_pcpu_freqs, &g_pcpu_freq_count);
          }
        }

        // S-Cluster (Super cores, M5+): try voltage-states3-sram
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
          char keyName[128];
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

  if (energyChan == NULL) {
    return -1;
  }

  if (gpuChan != NULL) {
    IOReportMergeChannels(energyChan, gpuChan, NULL);
    CFRelease(gpuChan);
  }

  CFDictionaryRef cpuChan =
      IOReportCopyChannelsInGroup(cpuGroup, NULL, 0, 0, 0);
  if (cpuChan != NULL) {
    IOReportMergeChannels(energyChan, cpuChan, NULL);
    CFRelease(cpuChan);
  }

  CFDictionaryRef amcChan =
      IOReportCopyChannelsInGroup(amcGroup, NULL, 0, 0, 0);
  if (amcChan != NULL) {
    IOReportMergeChannels(energyChan, amcChan, NULL);
    CFRelease(amcChan);
  }

  // PMP group provides DRAM bandwidth data on A-series chips (A18 Pro, etc.)
  // where AMC Stats channels are present but produce no delta data.
  CFDictionaryRef pmpChan =
      IOReportCopyChannelsInGroup(CFSTR("PMP"), NULL, 0, 0, 0);
  if (pmpChan != NULL) {
    IOReportMergeChannels(energyChan, pmpChan, NULL);
    CFRelease(pmpChan);
  }

  CFIndex size = CFDictionaryGetCount(energyChan);
  g_channels =
      CFDictionaryCreateMutableCopy(kCFAllocatorDefault, size, energyChan);
  CFRelease(energyChan);

  if (g_channels == NULL) {
    return -2;
  }

  CFMutableDictionaryRef subsystem = NULL;
  g_subscription =
      IOReportCreateSubscription(NULL, g_channels, &subsystem, 0, NULL);

  if (g_subscription == NULL) {
    CFRelease(g_channels);
    g_channels = NULL;
    return -3;
  }

  loadGpuFrequencies();
  loadCpuFrequencies();

  g_smcConn = SMCOpen();
  loadSMCTempKeys();

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
      char channel[128] = {0};
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
  temp_sensor_t temps[128];
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
    return "SSD"; // Ts* = SSD proximity
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

    // CPU Keys: Tp* or Te*
    if ((key[0] == 'T' && (key[1] == 'p' || key[1] == 'e'))) {
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
  for (int i = 0; i < totalKeys && g_all_temp_sensor_count < 128; i++) {
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

    // Read current value to ensure it's a valid sensor
    float val = (float)SMCGetFloatValue(g_smcConn, key);
    if (val <= 0 || val > 200)
      continue;

    temp_sensor_t *sensor = &g_all_temp_sensors[g_all_temp_sensor_count];
    strcpy(sensor->key, key);
    snprintf(sensor->name, sizeof(sensor->name), "%s %c%c", tempSensorName(key),
             key[2], key[3]);
    sensor->value = val;
    g_all_temp_sensor_count++;
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
  if (fanCount <= 0 || fanCount > maxFans)
    fanCount = (fanCount > maxFans) ? maxFans : fanCount;

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

  // Fallback to HID if SMC failed
  if (cpuCount == 0 || gpuCount == 0) {
    // ... (HID logic) ...
    const void *keys[2] = {CFSTR("PrimaryUsagePage"), CFSTR("PrimaryUsage")};
    int page = kHIDPage_AppleVendor;
    int usage = kHIDUsage_AppleVendor_TemperatureSensor;
    CFNumberRef pageNum =
        CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &page);
    CFNumberRef usageNum =
        CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &usage);
    const void *values[2] = {pageNum, usageNum};

    CFDictionaryRef matching = CFDictionaryCreate(
        kCFAllocatorDefault, keys, values, 2, &kCFTypeDictionaryKeyCallBacks,
        &kCFTypeDictionaryValueCallBacks);
    CFRelease(pageNum);
    CFRelease(usageNum);

    IOHIDEventSystemClientRef client =
        IOHIDEventSystemClientCreate(kCFAllocatorDefault);
    if (client != NULL) {
      IOHIDEventSystemClientSetMatching(client, matching);
      CFRelease(matching);

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

          char product[128] = {0};
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
      CFRelease(client);
    } else {
      CFRelease(matching);
    }
  }

  if (cpuCount > 0)
    *outCpuTemp = cpuSum / cpuCount;
  if (gpuCount > 0)
    *outGpuTemp = gpuSum / gpuCount;

  // Return max of both as "SoC Temp" for backward compatibility if needed
  return (*outCpuTemp > *outGpuTemp) ? *outCpuTemp : *outGpuTemp;
}

PowerMetrics samplePowerMetrics(int durationMs) {
  PowerMetrics metrics = {0};

  if (g_subscription == NULL || g_channels == NULL) {
    if (initIOReport() != 0) {
      return metrics;
    }
  }

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


    if (cfStringMatch(groupRef, "Energy Model")) {
      CFStringRef unitRef = IOReportChannelGetUnitLabel(item);
      int64_t val = IOReportSimpleGetIntegerValue(item, 0);
      double watts = energyToWatts(val, unitRef, (double)durationMs);

      if (cfStringContains(channelRef, "CPU Energy")) {
        metrics.cpuPower += watts;
      } else if (cfStringMatch(channelRef, "GPU Energy")) {
        metrics.gpuPower += watts;
      } else if (cfStringStartsWith(channelRef, "ANE")) {
        metrics.anePower += watts;
      } else if (cfStringStartsWith(channelRef, "DRAM")) {
        metrics.dramPower += watts;
      } else if (cfStringStartsWith(channelRef, "GPU SRAM")) {
        metrics.gpuSramPower += watts;
      }
    } else if (cfStringMatch(groupRef, "GPU Stats")) {
      CFStringRef subgroupRef = IOReportChannelGetSubGroup(item);
      if (subgroupRef != NULL &&
          cfStringMatch(subgroupRef, "GPU Performance States")) {
        if (cfStringMatch(channelRef, "GPUPH")) {
          int32_t stateCount = IOReportStateGetCount(item);
          int64_t totalTime = 0;
          int64_t activeTime = 0;
          double weightedFreq = 0;
          int activeStateIdx = 0;

          for (int32_t s = 0; s < stateCount; s++) {
            int64_t residency = IOReportStateGetResidency(item, s);
            CFStringRef stateName = IOReportStateGetNameForIndex(item, s);
            totalTime += residency;

            if (stateName != NULL && !cfStringMatch(stateName, "OFF") &&
                !cfStringMatch(stateName, "IDLE") &&
                !cfStringMatch(stateName, "DOWN")) {
              activeTime += residency;
              if (g_gpu_freq_count > 0 && activeStateIdx < g_gpu_freq_count) {
                weightedFreq += (double)g_gpu_freqs[activeStateIdx] * residency;
              }
              activeStateIdx++;
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
    } else if (cfStringMatch(groupRef, "CPU Stats")) {
      CFStringRef subgroupRef = IOReportChannelGetSubGroup(item);
      if (subgroupRef != NULL &&
          cfStringMatch(subgroupRef, "CPU Complex Performance States")) {

        // E-Cluster (usually CPU0 or ECPU)
        int isECluster = cfStringContains(channelRef, "ECPU") ||
                         cfStringContains(channelRef, "CPU0");
        int isPCluster = cfStringContains(channelRef, "PCPU") ||
                         cfStringContains(channelRef, "CPU1");
        int isSCluster = cfStringContains(channelRef, "SCPU");

        if (isECluster || isPCluster || isSCluster) {
          int32_t stateCount = IOReportStateGetCount(item);
          int64_t totalTime = 0;
          int64_t activeTime = 0;
          double weightedFreq = 0;

          for (int32_t s = 0; s < stateCount; s++) {
            int64_t residency = IOReportStateGetResidency(item, s);
            CFStringRef stateName = IOReportStateGetNameForIndex(item, s);
            totalTime += residency;

            if (stateName != NULL && !cfStringMatch(stateName, "OFF") &&
                !cfStringMatch(stateName, "IDLE")) {

              activeTime += residency;

              char nameBuf[64] = {0};
              CFStringGetCString(stateName, nameBuf, sizeof(nameBuf),
                                 kCFStringEncodingUTF8);

              int freq = 0;

              // Heuristic for "V#..." format
              if (nameBuf[0] == 'V') {
                int vIdx = -1;
                // Parse index after 'V'
                if (sscanf(nameBuf, "V%d", &vIdx) == 1 && vIdx >= 0) {
                  if (isECluster && vIdx < g_ecpu_freq_count) {
                    freq = g_ecpu_freqs[vIdx];
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

          if (totalTime > 0) {
            double activePercent =
                (double)activeTime / (double)totalTime * 100.0;
            int avgFreq = 0;
            if (activeTime > 0) {
              avgFreq = (int)(weightedFreq / activeTime);
            }

            if (isECluster) {
              metrics.eClusterActive = activePercent;
              metrics.eClusterFreqMHz = avgFreq;
            } else if (isPCluster) {
              metrics.pClusterActive = activePercent;
              metrics.pClusterFreqMHz = avgFreq;
            } else if (isSCluster) {
              metrics.sClusterActive = activePercent;
              metrics.sClusterFreqMHz = avgFreq;
            }
          }
        }
      }
    } else if (cfStringMatch(groupRef, "AMC Stats")) {
      // Sum memory bandwidth from non-DCS channels to avoid double counting.
      // DCS (DRAM Command Scheduler) channels are a subset of the total.
      // Works on M-series chips (M1, M2, M3, M4, M5, etc.).
      char channelName[256] = {0};
      CFStringGetCString(channelRef, channelName, sizeof(channelName),
                         kCFStringEncodingUTF8);
      if (strstr(channelName, "DCS") == NULL) {
        int64_t val = IOReportSimpleGetIntegerValue(item, 0);
        if (strstr(channelName, "RD") != NULL) {
          metrics.dramReadBytes += val;
        } else if (strstr(channelName, "WR") != NULL) {
          metrics.dramWriteBytes += val;
        }
      }
    } else if (cfStringMatch(groupRef, "PMP")) {
      // PMP group provides DRAM bandwidth on A-series chips (A18 Pro, etc.)
      // where AMC Stats channels exist but produce no delta data.
      // Channels are in subgroup "DRAM BW" with names like "F1 RD", "F1 WR",
      // "F2 RD", etc. and unit "B" (bytes). Sum all frequency bins.
      CFStringRef subgroupRef = IOReportChannelGetSubGroup(item);
      if (subgroupRef != NULL && cfStringMatch(subgroupRef, "DRAM BW")) {
        char channelName[256] = {0};
        CFStringGetCString(channelRef, channelName, sizeof(channelName),
                           kCFStringEncodingUTF8);
        int64_t val = IOReportSimpleGetIntegerValue(item, 0);
        if (val > 0) {
          if (strstr(channelName, "RD") != NULL) {
            pmpDramReadBytes += val;
          } else if (strstr(channelName, "WR") != NULL) {
            pmpDramWriteBytes += val;
          }
        }
      }
    }
  }

  // Fallback: use PMP DRAM BW data when AMC Stats produces no bandwidth data.
  // This occurs on A-series chips (A18 Pro, etc.) where AMC Stats channels
  // exist in the IOReport registry but produce no delta sample data.
  if (metrics.dramReadBytes == 0 && metrics.dramWriteBytes == 0) {
    metrics.dramReadBytes = pmpDramReadBytes;
    metrics.dramWriteBytes = pmpDramWriteBytes;
  }

  metrics.socTemp = readSocTemperature(&metrics.cpuTemp, &metrics.gpuTemp);

  if (g_smcConn) {
    metrics.systemPower = SMCGetFloatValue(g_smcConn, "PSTR");
  }

  // Read fan data
  metrics.fanCount = readFanInfo(metrics.fans, 8);

  // Read all temperature sensors
  loadAllTempSensors();
  metrics.tempSensorCount = g_all_temp_sensor_count;
  for (int i = 0; i < g_all_temp_sensor_count && i < 128; i++) {
    metrics.temps[i] = g_all_temp_sensors[i];
    // Refresh sensor value
    if (g_smcConn) {
      float v = (float)SMCGetFloatValue(g_smcConn, g_all_temp_sensors[i].key);
      if (v > 0)
        metrics.temps[i].value = v;
    }
  }

  CFRelease(delta);

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
}

int getThermalState() {
  NSProcessInfo *info = [NSProcessInfo processInfo];
  return (int)[info thermalState];
}

void debugMonitorChannels(int durationMs) { (void)durationMs; }
