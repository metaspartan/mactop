// Copyright (c) 2024-2026 Carsen Klock under MIT License
// overlay.m - Native macOS floating overlay HUD window

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#include <dispatch/dispatch.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

#define OVERLAY_SPARKLINE_HISTORY 60

extern char *GoI18nT(char *id);

static NSString *localize(NSString *key) {
  const char *cKey = [key UTF8String];
  char *cVal = GoI18nT((char *)cKey);
  if (!cVal)
    return key;
  NSString *res = [NSString stringWithUTF8String:cVal];
  free(cVal);
  return res;
}

// ---------- Metrics struct (passed from Go) ----------

typedef struct {
  double cpu_percent;
  double gpu_percent;
  double ane_percent;
  int gpu_freq_mhz;
  uint64_t mem_used_bytes;
  uint64_t mem_total_bytes;
  uint64_t swap_used_bytes;
  uint64_t swap_total_bytes;
  double total_watts;
  double package_watts;
  double cpu_watts;
  double gpu_watts;
  double ane_watts;
  double dram_watts;
  double soc_temp;
  double cpu_temp;
  double gpu_temp;
  char thermal_state[32];
  int thermal_level; // 0=nominal, 1=fair, 2=serious, 3=critical
  char model_name[128];
  int gpu_core_count;
  int e_core_count;
  int p_core_count;
  int s_core_count;
  int ecluster_freq_mhz;
  double ecluster_active;
  int pcluster_freq_mhz;
  double pcluster_active;
  int scluster_freq_mhz;
  double scluster_active;
  double net_in_bytes_per_sec;
  double net_out_bytes_per_sec;
  double disk_read_kb_per_sec;
  double disk_write_kb_per_sec;
  double tflops_fp32;
  char rdma_status[64];
  double dram_bw_combined_gbs;
  int fan_count;
  int fan_rpm[4];
  char fan_name[4][32];
} overlay_metrics_t;

// ---------- Config struct ----------

typedef struct {
  int show_cpu;
  int show_gpu;
  int show_ane;
  int show_memory;
  int show_power;
  int show_temps;
  int show_thermals;
  int show_fans;
  int show_bandwidth;
  int show_network;
  int show_gpu_freq;
  double opacity;
  char collapsed_sections[256]; // comma-separated ordered section names
  char expanded_order[512];     // comma-separated ordered section names
  char label_fps[32];
  char label_frame[32];
  char label_cpu[32];
  char label_gpu[32];
  char label_ane[32];
  char label_memory[32];
  char label_swap[32];
  char label_power[32];
  char label_bandwidth[64];
  char label_gpu_freq[64];
  char label_temps[32];
  char label_thermal[32];
  char label_fans[32];
  char label_network[32];
} overlay_config_t;

// ---------- Section ordering ----------

// Section ID enum for data-driven rendering
typedef enum {
  kSectionFPS = 0,
  kSectionFrame,
  kSectionCPU,
  kSectionGPU,
  kSectionANE,
  kSectionMemory,
  kSectionSwap,
  kSectionPower,
  kSectionBandwidth,
  kSectionGPUFreq,
  kSectionTemps,
  kSectionThermal,
  kSectionFans,
  kSectionNetwork,
  kSectionCount // sentinel
} OverlaySectionID;

// Parsed ordered section lists
static OverlaySectionID g_collapsedSections[kSectionCount];
static int g_collapsedCount = 0;
static OverlaySectionID g_expandedSections[kSectionCount];
static int g_expandedCount = 0;

static OverlaySectionID sectionIDFromName(const char *name) {
  if (strcmp(name, "fps") == 0) return kSectionFPS;
  if (strcmp(name, "frame") == 0) return kSectionFrame;
  if (strcmp(name, "cpu") == 0) return kSectionCPU;
  if (strcmp(name, "gpu") == 0) return kSectionGPU;
  if (strcmp(name, "ane") == 0) return kSectionANE;
  if (strcmp(name, "memory") == 0) return kSectionMemory;
  if (strcmp(name, "swap") == 0) return kSectionSwap;
  if (strcmp(name, "power") == 0) return kSectionPower;
  if (strcmp(name, "bandwidth") == 0) return kSectionBandwidth;
  if (strcmp(name, "gpu_freq") == 0) return kSectionGPUFreq;
  if (strcmp(name, "temps") == 0) return kSectionTemps;
  if (strcmp(name, "thermal") == 0) return kSectionThermal;
  if (strcmp(name, "fans") == 0) return kSectionFans;
  if (strcmp(name, "network") == 0) return kSectionNetwork;
  return kSectionCount; // invalid
}

static void parseSectionList(const char *csv, OverlaySectionID *out, int *count) {
  *count = 0;
  if (!csv || csv[0] == '\0') return;

  char buf[512];
  strncpy(buf, csv, sizeof(buf) - 1);
  buf[sizeof(buf) - 1] = '\0';

  char *token = strtok(buf, ",");
  while (token && *count < kSectionCount) {
    // Trim leading whitespace
    while (*token == ' ') token++;
    OverlaySectionID sid = sectionIDFromName(token);
    if (sid != kSectionCount) {
      out[(*count)++] = sid;
    }
    token = strtok(NULL, ",");
  }
}

// ---------- Global state ----------

static overlay_config_t g_overlay_config = {
    .show_cpu = 1,
    .show_gpu = 1,
    .show_ane = 1,
    .show_memory = 1,
    .show_power = 1,
    .show_temps = 1,
    .show_thermals = 1,
    .show_fans = 1,
    .show_bandwidth = 1,
    .show_network = 1,
    .show_gpu_freq = 1,
    .opacity = 0.88,
    .collapsed_sections = "fps,frame,cpu,gpu,memory",
    .expanded_order = "fps,frame,cpu,gpu,ane,memory,swap,power,bandwidth,gpu_freq,temps,thermal,fans,network",
    .label_fps = "FPS",
    .label_frame = "Frame Interval",
    .label_cpu = "CPU",
    .label_gpu = "GPU",
    .label_ane = "ANE",
    .label_memory = "Memory",
    .label_swap = "Swap",
    .label_power = "Power",
    .label_bandwidth = "DRAM Bandwidth",
    .label_gpu_freq = "GPU Frequency",
    .label_temps = "Temperatures",
    .label_thermal = "Thermal State",
    .label_fans = "Fans",
    .label_network = "Network",
};

static overlay_metrics_t g_overlay_metrics;
static double cpuSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};
static double gpuSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};
static double fpsSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};
static double frameIntSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};
static BOOL g_overlay_expanded = NO;
static BOOL g_showSettings = NO;

// Gear icon hit rect (set during drawRect, tested in mouseDown)
static NSRect g_gearHitRect = {0};

// Settings panel: which collapsed sections are toggled
// We track the full set: user toggles these, and on save we rebuild collapsed_sections
static BOOL g_settingsCollapsed[kSectionCount];
static BOOL g_settingsExpanded[kSectionCount];
static BOOL g_settingsInited = NO;

// Mutable order arrays for the settings panel — users drag to reorder these
static OverlaySectionID g_settingsCollapsedOrder[kSectionCount];
static int g_settingsCollapsedOrderCount = 0;
static OverlaySectionID g_settingsExpandedOrder[kSectionCount];
static int g_settingsExpandedOrderCount = 0;

// Drag-to-reorder state
static BOOL g_dragActive = NO;
static int g_dragSourceIdx = -1;       // Index in the order array being dragged
static BOOL g_dragIsCollapsed = YES;    // Which list is being dragged
static int g_dragInsertIdx = -1;        // Insertion point (between rows)
static CGFloat g_dragMouseY = 0;        // Current mouse Y in view coords
static CGFloat g_dragListOriginY = 0;   // Y position where the list starts
static CGFloat g_dragRowH = 26;         // Row height for drag calculations

static const char *sectionDisplayName(OverlaySectionID sid) {
  switch (sid) {
    case kSectionFPS: return g_overlay_config.label_fps;
    case kSectionFrame: return g_overlay_config.label_frame;
    case kSectionCPU: return g_overlay_config.label_cpu;
    case kSectionGPU: return g_overlay_config.label_gpu;
    case kSectionANE: return g_overlay_config.label_ane;
    case kSectionMemory: return g_overlay_config.label_memory;
    case kSectionSwap: return g_overlay_config.label_swap;
    case kSectionPower: return g_overlay_config.label_power;
    case kSectionBandwidth: return g_overlay_config.label_bandwidth;
    case kSectionGPUFreq: return g_overlay_config.label_gpu_freq;
    case kSectionTemps: return g_overlay_config.label_temps;
    case kSectionThermal: return g_overlay_config.label_thermal;
    case kSectionFans: return g_overlay_config.label_fans;
    case kSectionNetwork: return g_overlay_config.label_network;
    default: return "?";
  }
}

static const char *sectionConfigName(OverlaySectionID sid) {
  switch (sid) {
    case kSectionFPS: return "fps";
    case kSectionFrame: return "frame";
    case kSectionCPU: return "cpu";
    case kSectionGPU: return "gpu";
    case kSectionANE: return "ane";
    case kSectionMemory: return "memory";
    case kSectionSwap: return "swap";
    case kSectionPower: return "power";
    case kSectionBandwidth: return "bandwidth";
    case kSectionGPUFreq: return "gpu_freq";
    case kSectionTemps: return "temps";
    case kSectionThermal: return "thermal";
    case kSectionFans: return "fans";
    case kSectionNetwork: return "network";
    default: return "";
  }
}

// Returns whether a section should be drawn based on --overlay-sections show flags.
static BOOL sectionShouldDraw(OverlaySectionID sid, overlay_config_t cfg) {
  switch (sid) {
    case kSectionCPU: return cfg.show_cpu;
    case kSectionGPU: return cfg.show_gpu;
    case kSectionANE: return cfg.show_ane;
    case kSectionMemory: return cfg.show_memory;
    case kSectionPower: return cfg.show_power;
    case kSectionTemps: return cfg.show_temps;
    case kSectionThermal: return cfg.show_thermals;
    case kSectionFans: return cfg.show_fans;
    case kSectionBandwidth: return cfg.show_bandwidth;
    case kSectionNetwork: return cfg.show_network;
    case kSectionGPUFreq: return cfg.show_gpu_freq;
    default: return YES;
  }
}

// Returns the number of rows a section contributes to height calculation.
static int sectionRowCount(OverlaySectionID sid, overlay_metrics_t metrics) {
  switch (sid) {
    case kSectionPower:
      return g_overlay_config.show_power ? 5 : 0;
    case kSectionSwap:
      return (metrics.swap_used_bytes > 0 && metrics.swap_total_bytes > 0) ? 1 : 0;
    case kSectionFans:
      return (g_overlay_config.show_fans && metrics.fan_count > 0) ? 1 : 0;
    case kSectionCPU: return g_overlay_config.show_cpu ? 1 : 0;
    case kSectionGPU: return g_overlay_config.show_gpu ? 1 : 0;
    case kSectionANE: return g_overlay_config.show_ane ? 1 : 0;
    case kSectionMemory: return g_overlay_config.show_memory ? 1 : 0;
    case kSectionTemps: return g_overlay_config.show_temps ? 1 : 0;
    case kSectionThermal: return g_overlay_config.show_thermals ? 1 : 0;
    case kSectionBandwidth: return g_overlay_config.show_bandwidth ? 1 : 0;
    case kSectionNetwork: return g_overlay_config.show_network ? 1 : 0;
    case kSectionGPUFreq: return g_overlay_config.show_gpu_freq ? 1 : 0;
    default: return 1; // FPS, Frame
  }
}

// All canonical sections for building the settings order arrays
static OverlaySectionID g_allCanonicalSections[] = {
  kSectionFPS, kSectionFrame, kSectionCPU, kSectionGPU, kSectionANE,
  kSectionMemory, kSectionSwap, kSectionPower, kSectionBandwidth,
  kSectionGPUFreq, kSectionTemps, kSectionThermal, kSectionFans, kSectionNetwork
};
static int g_nCanonicalSections = sizeof(g_allCanonicalSections) / sizeof(g_allCanonicalSections[0]);

// Initialize settings toggles and order arrays from current parsed section lists
static void initSettingsFromConfig(void) {
  memset(g_settingsCollapsed, 0, sizeof(g_settingsCollapsed));
  memset(g_settingsExpanded, 0, sizeof(g_settingsExpanded));
  for (int i = 0; i < g_collapsedCount; i++) {
    g_settingsCollapsed[g_collapsedSections[i]] = YES;
  }
  for (int i = 0; i < g_expandedCount; i++) {
    g_settingsExpanded[g_expandedSections[i]] = YES;
  }

  // Build order arrays: start with sections from the existing config (preserves user order),
  // then append any canonical sections not yet in the list.
  g_settingsCollapsedOrderCount = 0;
  BOOL collapsedSeen[kSectionCount] = {0};
  for (int i = 0; i < g_collapsedCount; i++) {
    g_settingsCollapsedOrder[g_settingsCollapsedOrderCount++] = g_collapsedSections[i];
    collapsedSeen[g_collapsedSections[i]] = YES;
  }
  for (int i = 0; i < g_nCanonicalSections; i++) {
    OverlaySectionID sid = g_allCanonicalSections[i];
    if (!collapsedSeen[sid]) {
      g_settingsCollapsedOrder[g_settingsCollapsedOrderCount++] = sid;
    }
  }

  g_settingsExpandedOrderCount = 0;
  BOOL expandedSeen[kSectionCount] = {0};
  for (int i = 0; i < g_expandedCount; i++) {
    g_settingsExpandedOrder[g_settingsExpandedOrderCount++] = g_expandedSections[i];
    expandedSeen[g_expandedSections[i]] = YES;
  }
  for (int i = 0; i < g_nCanonicalSections; i++) {
    OverlaySectionID sid = g_allCanonicalSections[i];
    if (!expandedSeen[sid]) {
      g_settingsExpandedOrder[g_settingsExpandedOrderCount++] = sid;
    }
  }

  g_settingsInited = YES;
}

// Save current settings toggles back to config.json
// Uses the drag-reordered g_settingsCollapsedOrder / g_settingsExpandedOrder
// arrays so user reordering is preserved.
static void saveSettingsToConfig(void) {
  // --- Rebuild collapsed_sections from the reordered settings array ---
  char collapsed[256] = {0};
  int cLen = 0;
  for (int i = 0; i < g_settingsCollapsedOrderCount; i++) {
    OverlaySectionID sid = g_settingsCollapsedOrder[i];
    if (g_settingsCollapsed[sid]) {
      const char *name = sectionConfigName(sid);
      if (cLen > 0) collapsed[cLen++] = ',';
      int nLen = (int)strlen(name);
      if (cLen + nLen < 255) { memcpy(collapsed + cLen, name, nLen); cLen += nLen; }
    }
  }
  collapsed[cLen] = '\0';

  // --- Rebuild expanded_order from the reordered settings array ---
  char expanded[512] = {0};
  int eLen = 0;
  for (int i = 0; i < g_settingsExpandedOrderCount; i++) {
    OverlaySectionID sid = g_settingsExpandedOrder[i];
    if (g_settingsExpanded[sid]) {
      const char *name = sectionConfigName(sid);
      if (eLen > 0) expanded[eLen++] = ',';
      int nLen = (int)strlen(name);
      if (eLen + nLen < 511) { memcpy(expanded + eLen, name, nLen); eLen += nLen; }
    }
  }
  expanded[eLen] = '\0';

  // Update the live config
  strncpy(g_overlay_config.collapsed_sections, collapsed, 255);
  g_overlay_config.collapsed_sections[255] = '\0';
  strncpy(g_overlay_config.expanded_order, expanded, 511);
  g_overlay_config.expanded_order[511] = '\0';
  parseSectionList(g_overlay_config.collapsed_sections, g_collapsedSections, &g_collapsedCount);
  parseSectionList(g_overlay_config.expanded_order, g_expandedSections, &g_expandedCount);

  // Write to ~/.mactop/config.json
  const char *home = getenv("HOME");
  if (!home) return;

  char configDir[512], configPath[512];
  snprintf(configDir, sizeof(configDir), "%s/.mactop", home);
  snprintf(configPath, sizeof(configPath), "%s/config.json", configDir);
  mkdir(configDir, 0755);

  // Read existing config
  NSData *existingData = [NSData dataWithContentsOfFile:
      [NSString stringWithUTF8String:configPath]];
  NSMutableDictionary *config;
  if (existingData) {
    config = [NSJSONSerialization JSONObjectWithData:existingData options:NSJSONReadingMutableContainers error:nil];
    if (!config || ![config isKindOfClass:[NSDictionary class]]) {
      config = [NSMutableDictionary dictionary];
    }
  } else {
    config = [NSMutableDictionary dictionary];
  }

  // Build overlay section — use the order-preserved C strings we just built
  NSMutableArray *collapsedArr = [NSMutableArray array];
  NSMutableArray *expandedArr = [NSMutableArray array];
  parseSectionList(collapsed, g_collapsedSections, &g_collapsedCount);
  for (int i = 0; i < g_collapsedCount; i++)
    [collapsedArr addObject:[NSString stringWithUTF8String:sectionConfigName(g_collapsedSections[i])]];
  parseSectionList(expanded, g_expandedSections, &g_expandedCount);
  for (int i = 0; i < g_expandedCount; i++)
    [expandedArr addObject:[NSString stringWithUTF8String:sectionConfigName(g_expandedSections[i])]];

  NSMutableDictionary *overlayDict = [NSMutableDictionary dictionary];
  overlayDict[@"collapsed_sections"] = collapsedArr;
  overlayDict[@"expanded_order"] = expandedArr;
  overlayDict[@"opacity"] = @(g_overlay_config.opacity);
  config[@"overlay"] = overlayDict;

  // Write back
  NSData *jsonData = [NSJSONSerialization dataWithJSONObject:config
      options:NSJSONWritingPrettyPrinted | NSJSONWritingSortedKeys error:nil];
  if (jsonData) {
    [jsonData writeToFile:[NSString stringWithUTF8String:configPath] atomically:YES];
  }
}

// Settings panel: hit rects for checkboxes (stored during drawRect, checked in mouseDown)
#define MAX_SETTINGS_ROWS 28
static NSRect g_settingsHitRects[MAX_SETTINGS_ROWS];
static int g_settingsHitSectionID[MAX_SETTINGS_ROWS]; // OverlaySectionID
static BOOL g_settingsHitIsCollapsed[MAX_SETTINGS_ROWS]; // true=collapsed toggle, false=expanded toggle
static int g_settingsHitCount = 0;
static NSRect g_settingsDoneRect = {0};

// Drag handle hit rects (≡ icon on the right of each row)
static NSRect g_dragHandleHitRects[MAX_SETTINGS_ROWS];
static int g_dragHandleHitSectionIdx[MAX_SETTINGS_ROWS]; // Index into g_settingsCollapsed/ExpandedOrder
static BOOL g_dragHandleHitIsCollapsed[MAX_SETTINGS_ROWS];
static int g_dragHandleHitCount = 0;

// Tracks the Y origin of collapsed and expanded lists in the settings panel
static CGFloat g_collapsedListOriginY = 0;
static CGFloat g_expandedListOriginY = 0;

static void pushSparkHistory(double *buf, double val) {
  memmove(buf, buf + 1, (OVERLAY_SPARKLINE_HISTORY - 1) * sizeof(double));
  buf[OVERLAY_SPARKLINE_HISTORY - 1] = val;
}

// ---------- FPS counter via CGDisplayStream ----------
// Counts actual display surface updates (real rendered frames), not VSync ticks.
// CGDisplayStream only fires when the WindowServer composites a new frame,
// so on a static desktop FPS will be low, while a game at 45fps shows ~45.
//
// We use dlsym to load CGDisplayStream functions at runtime because Apple
// marked them as unavailable in the macOS 15 SDK headers even though the
// symbols still exist in the CoreGraphics dylib and work fine at runtime.

#include <mach/mach_time.h>
#include <dlfcn.h>

// Opaque types
typedef void *CGDisplayStreamRef_t;
typedef void *CGDisplayStreamUpdateRef_t;
typedef void *IOSurfaceRef_t;

// Function pointer types matching CGDisplayStream API
typedef CGDisplayStreamRef_t (*CGDisplayStreamCreateWithDispatchQueue_fn)(
    CGDirectDisplayID, size_t, size_t, int32_t, CFDictionaryRef,
    dispatch_queue_t,
    void (^)(int status, uint64_t displayTime, IOSurfaceRef_t surface,
             CGDisplayStreamUpdateRef_t updateRef));
typedef int (*CGDisplayStreamStart_fn)(CGDisplayStreamRef_t);
typedef int (*CGDisplayStreamStop_fn)(CGDisplayStreamRef_t);
typedef size_t (*CGDisplayStreamUpdateGetDropCount_fn)(
    CGDisplayStreamUpdateRef_t);

// Loaded function pointers
static CGDisplayStreamCreateWithDispatchQueue_fn fn_CGDisplayStreamCreate =
    NULL;
static CGDisplayStreamStart_fn fn_CGDisplayStreamStart = NULL;
static CGDisplayStreamStop_fn fn_CGDisplayStreamStop = NULL;
static CGDisplayStreamUpdateGetDropCount_fn fn_CGDisplayStreamGetDrops = NULL;

// CGDisplayStream property keys (string constants)
static CFStringRef kMinFrameTime = NULL;
static CFStringRef kShowCursor = NULL;
static CFStringRef kQueueDepth = NULL;
static CFStringRef kSourceRect = NULL;

static bool loadCGDisplayStreamSymbols(void) {
  void *cg = dlopen(
      "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics",
      RTLD_LAZY);
  if (!cg)
    return false;

  fn_CGDisplayStreamCreate =
      (CGDisplayStreamCreateWithDispatchQueue_fn)dlsym(
          cg, "CGDisplayStreamCreateWithDispatchQueue");
  fn_CGDisplayStreamStart =
      (CGDisplayStreamStart_fn)dlsym(cg, "CGDisplayStreamStart");
  fn_CGDisplayStreamStop =
      (CGDisplayStreamStop_fn)dlsym(cg, "CGDisplayStreamStop");
  fn_CGDisplayStreamGetDrops =
      (CGDisplayStreamUpdateGetDropCount_fn)dlsym(
          cg, "CGDisplayStreamUpdateGetDropCount");

  // Load string constant symbols
  CFStringRef *pMinFrameTime =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamMinimumFrameTime");
  CFStringRef *pShowCursor =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamShowCursor");
  CFStringRef *pQueueDepth =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamQueueDepth");
  CFStringRef *pSourceRect =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamSourceRect");

  if (pMinFrameTime) kMinFrameTime = *pMinFrameTime;
  if (pShowCursor) kShowCursor = *pShowCursor;
  if (pQueueDepth) kQueueDepth = *pQueueDepth;
  if (pSourceRect) kSourceRect = *pSourceRect;

  // Don't dlclose — keep symbols alive
  return (fn_CGDisplayStreamCreate && fn_CGDisplayStreamStart &&
          fn_CGDisplayStreamStop && fn_CGDisplayStreamGetDrops && kMinFrameTime &&
          kShowCursor && kQueueDepth);
}

static CGDisplayStreamRef_t g_fpsStream = NULL;
static _Atomic uint32_t g_fpsFrameCount = 0;   // Completed frames this interval
static _Atomic uint32_t g_fpsDropCount = 0;     // Dropped frames this interval
static _Atomic uint32_t g_fpsValue = 0;         // Last computed FPS
static _Atomic uint32_t g_frameIntervalUs = 0;  // Frame interval in microseconds (×1000 for ms)
static dispatch_source_t g_fpsTimer = NULL;
static uint64_t g_fpsLastTimestamp = 0;
static BOOL g_fpsStreamFailed = NO;

static double machTimeToSeconds(uint64_t elapsed) {
  static mach_timebase_info_data_t sTimebase = {0};
  if (sTimebase.denom == 0) {
    mach_timebase_info(&sTimebase);
  }
  double nanos = (double)elapsed * sTimebase.numer / sTimebase.denom;
  return nanos / 1e9;
}

// Frame statuses
enum {
  kFrameStatusComplete = 0,
  kFrameStatusIdle = 1,
  kFrameStatusBlank = 2,
  kFrameStatusStopped = 3,
};

static void startFPSCounter(void) {
  if (!loadCGDisplayStreamSymbols()) {
    // CGDisplayStream not available — FPS feature silently disabled
    g_fpsStreamFailed = YES;
    return;
  }

  CGDirectDisplayID mainDisplay = CGMainDisplayID();

  // minimumFrameTime = 0 means "deliver as fast as possible"
  NSMutableDictionary *streamProps = [NSMutableDictionary dictionary];
  streamProps[(__bridge NSString *)kMinFrameTime] = @(0.0);
  streamProps[(__bridge NSString *)kShowCursor] = @(NO);
  streamProps[(__bridge NSString *)kQueueDepth] = @(1);

  dispatch_queue_t fpsQueue =
      dispatch_queue_create("com.mactop.fps", DISPATCH_QUEUE_SERIAL);

  // Capture a 16x16 region minimum scaling threshold check (bypass <16px hw faults)
  g_fpsStream = fn_CGDisplayStreamCreate(
      mainDisplay, 16, 16, 'BGRA', (__bridge CFDictionaryRef)streamProps,
      fpsQueue,
      ^(int status, uint64_t displayTime __attribute__((unused)),
        IOSurfaceRef_t frameSurface __attribute__((unused)),
        CGDisplayStreamUpdateRef_t updateRef) {
        if (status == kFrameStatusComplete) {
          atomic_fetch_add(&g_fpsFrameCount, 1);
          // Count dropped frames (frames WindowServer rendered but we missed)
          if (updateRef && fn_CGDisplayStreamGetDrops) {
            size_t dropped = fn_CGDisplayStreamGetDrops(updateRef);
            if (dropped > 0) {
              atomic_fetch_add(&g_fpsDropCount, (uint32_t)dropped);
            }
          }
        }
      });

  if (!g_fpsStream) {
    g_fpsStreamFailed = YES;
    return; // Stream creation failed — FPS silently unavailable
  }
  fn_CGDisplayStreamStart(g_fpsStream);

  g_fpsLastTimestamp = mach_absolute_time();

  // Timer fires every ~1s to snapshot FPS using actual elapsed time
  g_fpsTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0,
                                       dispatch_get_main_queue());
  dispatch_source_set_timer(g_fpsTimer,
                            dispatch_time(DISPATCH_TIME_NOW, NSEC_PER_SEC),
                            NSEC_PER_SEC, NSEC_PER_SEC / 10);
  dispatch_source_set_event_handler(g_fpsTimer, ^{
    uint64_t now = mach_absolute_time();
    uint64_t elapsed = now - g_fpsLastTimestamp;
    double seconds = machTimeToSeconds(elapsed);
    g_fpsLastTimestamp = now;

    uint32_t completed = atomic_exchange(&g_fpsFrameCount, 0);
    uint32_t dropped = atomic_exchange(&g_fpsDropCount, 0);
    uint32_t totalFrames = completed + dropped;

    // Calculate FPS and frame interval from actual elapsed time
    uint32_t fps = 0;
    uint32_t intervalUs = 0;
    if (seconds > 0.1 && totalFrames > 0) {
      fps = (uint32_t)(totalFrames / seconds + 0.5);
      // Average frame interval in microseconds for sub-ms precision
      intervalUs = (uint32_t)(seconds * 1e6 / totalFrames + 0.5);
    }
    atomic_store(&g_fpsValue, fps);
    atomic_store(&g_frameIntervalUs, intervalUs);
  });
  dispatch_resume(g_fpsTimer);
}

static void stopFPSCounter(void) {
  if (g_fpsStream && fn_CGDisplayStreamStop) {
    fn_CGDisplayStreamStop(g_fpsStream);
    CFRelease(g_fpsStream);
    g_fpsStream = NULL;
  }
  if (g_fpsTimer) {
    dispatch_source_cancel(g_fpsTimer);
    g_fpsTimer = NULL;
  }
}

// ---------- Forward declarations ----------

void updateOverlayMetrics(overlay_metrics_t *m);

@class OverlayContentView;
@class OverlayWindow;

static OverlayWindow *g_overlayWindow = nil;
static OverlayContentView *g_contentView = nil;

// ---------- Color helpers ----------

// Neon green terminal aesthetic
static NSColor *overlayNeonGreen(void) {
  return [NSColor colorWithRed:0.15 green:1.0 blue:0.30 alpha:1.0];
}
static NSColor *overlayAccentGreen(void) {
  return overlayNeonGreen();
}
static NSColor *overlayAccentOrange(void) {
  return [NSColor colorWithRed:1.0 green:0.65 blue:0.10 alpha:1.0];
}
static NSColor *overlayAccentCyan(void) {
  return [NSColor colorWithRed:0.20 green:0.95 blue:0.95 alpha:1.0];
}
static NSColor *overlayAccentPurple(void) {
  return [NSColor colorWithRed:0.75 green:0.45 blue:1.0 alpha:1.0];
}
static NSColor *overlayAccentRed(void) {
  return [NSColor colorWithRed:1.0 green:0.25 blue:0.20 alpha:1.0];
}
static NSColor *overlayAccentYellow(void) {
  return [NSColor colorWithRed:1.0 green:0.92 blue:0.20 alpha:1.0];
}
static NSColor *overlayAccentBlue(void) {
  return [NSColor colorWithRed:0.30 green:0.60 blue:1.0 alpha:1.0];
}
static NSColor *overlayDimText(void) {
  return [NSColor colorWithRed:0.10 green:0.75 blue:0.22 alpha:1.0];
}
static NSColor *overlayBrightText(void) {
  return overlayNeonGreen();
}

// ---------- Throughput formatter ----------

static NSString *formatOverlayThroughput(double bps) {
  if (bps < 1024.0)
    return [NSString stringWithFormat:@"%.0fB/s", bps];
  if (bps < 1024.0 * 1024.0)
    return [NSString stringWithFormat:@"%.1fKB/s", bps / 1024.0];
  if (bps < 1024.0 * 1024.0 * 1024.0)
    return [NSString stringWithFormat:@"%.1fMB/s", bps / (1024.0 * 1024.0)];
  return [NSString
      stringWithFormat:@"%.2fGB/s", bps / (1024.0 * 1024.0 * 1024.0)];
}

// ---------- Color for percentage ----------

static NSColor *colorForPercent(double pct) {
  if (pct >= 80.0)
    return overlayAccentRed();
  if (pct >= 50.0)
    return overlayAccentYellow();
  return overlayAccentGreen();
}

// ---------- Custom NSWindow subclass ----------

@interface OverlayWindow : NSWindow
@end

@implementation OverlayWindow

- (BOOL)canBecomeKeyWindow {
  return NO;
}
- (BOOL)canBecomeMainWindow {
  return NO;
}

- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
  return NSDragOperationNone;
}

@end

// ---------- Content view ----------

static NSInteger g_opacityFlashCountdown = 0; // Show opacity indicator for N frames

@interface OverlayContentView : NSView
@property(nonatomic) BOOL dragging;
@property(nonatomic) NSPoint dragStart;
@property(nonatomic) NSPoint windowStart;
@end

@implementation OverlayContentView

- (BOOL)isFlipped {
  return YES;
}

// Allow dragging the window by dragging anywhere on the overlay
- (void)mouseDown:(NSEvent *)event {
  NSPoint pt = [self convertPoint:[event locationInWindow] fromView:nil];

  // Gear icon click — toggle settings panel
  if (NSPointInRect(pt, g_gearHitRect)) {
    if (!g_settingsInited) initSettingsFromConfig();
    g_showSettings = !g_showSettings;
    updateOverlayMetrics(&g_overlay_metrics);
    return;
  }

  // Settings panel interactions
  if (g_showSettings) {
    // Done button
    if (NSPointInRect(pt, g_settingsDoneRect)) {
      saveSettingsToConfig();
      g_showSettings = NO;
      updateOverlayMetrics(&g_overlay_metrics);
      return;
    }
    // Check drag handles first (takes priority over checkbox toggles)
    for (int i = 0; i < g_dragHandleHitCount; i++) {
      if (NSPointInRect(pt, g_dragHandleHitRects[i])) {
        g_dragActive = YES;
        g_dragSourceIdx = g_dragHandleHitSectionIdx[i];
        g_dragIsCollapsed = g_dragHandleHitIsCollapsed[i];
        g_dragInsertIdx = g_dragSourceIdx;
        g_dragMouseY = pt.y;
        g_dragListOriginY = g_dragIsCollapsed ? g_collapsedListOriginY : g_expandedListOriginY;
        [self setNeedsDisplay:YES];
        return;
      }
    }
    // Checkbox toggles
    for (int i = 0; i < g_settingsHitCount; i++) {
      if (NSPointInRect(pt, g_settingsHitRects[i])) {
        OverlaySectionID sid = (OverlaySectionID)g_settingsHitSectionID[i];
        if (g_settingsHitIsCollapsed[i]) {
          g_settingsCollapsed[sid] = !g_settingsCollapsed[sid];
        } else {
          g_settingsExpanded[sid] = !g_settingsExpanded[sid];
        }
        [self setNeedsDisplay:YES];
        return;
      }
    }
    return; // Consume click in settings mode (no dragging)
  }

  // Toggle expand/collapse chevron
  NSRect toggleRect = NSMakeRect(self.bounds.size.width / 2.0 - 40, self.bounds.size.height - 35, 80, 35);
  if (NSPointInRect(pt, toggleRect)) {
    g_overlay_expanded = !g_overlay_expanded;
    updateOverlayMetrics(&g_overlay_metrics);
    return;
  }

  self.dragStart = [NSEvent mouseLocation];
  self.windowStart = self.window.frame.origin;
  self.dragging = YES;
}

- (void)mouseDragged:(NSEvent *)event {
  if (g_dragActive) {
    NSPoint pt = [self convertPoint:[event locationInWindow] fromView:nil];
    g_dragMouseY = pt.y;

    // Calculate insertion index based on mouse Y position
    int count = g_dragIsCollapsed ? g_settingsCollapsedOrderCount : g_settingsExpandedOrderCount;
    CGFloat listY = g_dragIsCollapsed ? g_collapsedListOriginY : g_expandedListOriginY;
    CGFloat relY = pt.y - listY;
    int insertIdx = (int)(relY / g_dragRowH + 0.5);
    if (insertIdx < 0) insertIdx = 0;
    if (insertIdx > count) insertIdx = count;
    g_dragInsertIdx = insertIdx;

    [self setNeedsDisplay:YES];
    return;
  }

  if (!self.dragging)
    return;
  NSPoint current = [NSEvent mouseLocation];
  CGFloat dx = current.x - self.dragStart.x;
  CGFloat dy = current.y - self.dragStart.y;
  NSPoint newOrigin =
      NSMakePoint(self.windowStart.x + dx, self.windowStart.y + dy);
  [self.window setFrameOrigin:newOrigin];
}

- (void)mouseUp:(NSEvent *)event {
  if (g_dragActive) {
    // Perform the reorder
    OverlaySectionID *order;
    int *count;
    if (g_dragIsCollapsed) {
      order = g_settingsCollapsedOrder;
      count = &g_settingsCollapsedOrderCount;
    } else {
      order = g_settingsExpandedOrder;
      count = &g_settingsExpandedOrderCount;
    }

    int src = g_dragSourceIdx;
    int dst = g_dragInsertIdx;
    if (src >= 0 && src < *count && dst >= 0 && dst <= *count && src != dst) {
      // Remove the source item
      OverlaySectionID moving = order[src];
      // Adjust destination if it's after the source
      if (dst > src) dst--;
      // Shift elements
      for (int i = src; i < *count - 1; i++) {
        order[i] = order[i + 1];
      }
      // Insert at destination
      for (int i = *count - 1; i > dst; i--) {
        order[i] = order[i - 1];
      }
      order[dst] = moving;
    }

    g_dragActive = NO;
    g_dragSourceIdx = -1;
    g_dragInsertIdx = -1;
    [self setNeedsDisplay:YES];
    return;
  }
  self.dragging = NO;
}

// Scroll wheel adjusts opacity
- (void)scrollWheel:(NSEvent *)event {
  CGFloat delta = event.scrollingDeltaY * 0.01;
  CGFloat newOpacity = self.window.alphaValue + delta;
  if (newOpacity < 0.15) newOpacity = 0.15;
  if (newOpacity > 1.0) newOpacity = 1.0;
  self.window.alphaValue = newOpacity;
  g_overlay_config.opacity = newOpacity;
  g_opacityFlashCountdown = 30; // Show indicator for ~30 frames
  [self setNeedsDisplay:YES];
}

// ---------- Drawing ----------

static void drawMiniSparkline(double *data, int count, CGFloat x, CGFloat y,
                              CGFloat w, CGFloat h, NSColor *color,
                              double maxVal) {
  if (count < 2)
    return;

  if (maxVal <= 0) maxVal = 100.0;
  NSBezierPath *fill = [NSBezierPath bezierPath];
  [fill moveToPoint:NSMakePoint(x, y + h)];

  for (int i = 0; i < count; i++) {
    CGFloat px = x + ((CGFloat)i / (CGFloat)(count - 1)) * w;
    CGFloat val = data[i];
    if (val < 0) val = 0;
    if (val > maxVal) val = maxVal;
    CGFloat py = y + h - (val / maxVal) * h;
    [fill lineToPoint:NSMakePoint(px, py)];
  }

  [fill lineToPoint:NSMakePoint(x + w, y + h)];
  [fill closePath];

  [[color colorWithAlphaComponent:0.25] set];
  [fill fill];

  // Draw line on top
  NSBezierPath *line = [NSBezierPath bezierPath];
  for (int i = 0; i < count; i++) {
    CGFloat px = x + ((CGFloat)i / (CGFloat)(count - 1)) * w;
    CGFloat val = data[i];
    if (val < 0) val = 0;
    if (val > maxVal) val = maxVal;
    CGFloat py = y + h - (val / maxVal) * h;
    if (i == 0)
      [line moveToPoint:NSMakePoint(px, py)];
    else
      [line lineToPoint:NSMakePoint(px, py)];
  }
  [line setLineWidth:1.5];
  [[color colorWithAlphaComponent:0.9] set];
  [line stroke];
}

static void drawMiniBar(CGFloat x, CGFloat y, CGFloat w, CGFloat h,
                        double pct, NSColor *color) {
  CGFloat radius = h / 2.0;

  // Track
  [[NSColor colorWithWhite:1.0 alpha:0.08] set];
  NSBezierPath *track =
      [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(x, y, w, h)
                                      xRadius:radius
                                      yRadius:radius];
  [track fill];

  // Fill
  CGFloat fillW = (pct / 100.0) * w;
  if (fillW < 1.0 && pct > 0)
    fillW = 1.0;
  if (fillW > 0) {
    [color set];
    NSBezierPath *bar = [NSBezierPath
        bezierPathWithRoundedRect:NSMakeRect(x, y, fillW, h)
                          xRadius:radius
                          yRadius:radius];
    [bar fill];
  }
}

- (void)drawRect:(NSRect)dirtyRect {
  [super drawRect:dirtyRect];

  overlay_metrics_t m = g_overlay_metrics;
  overlay_config_t cfg = g_overlay_config;

  CGFloat W = self.bounds.size.width;
  CGFloat padX = 14;
  CGFloat contentW = W - padX * 2;
  __block CGFloat y = 16;

  NSFont *headerFont =
      [NSFont systemFontOfSize:18 weight:NSFontWeightBold];
  NSFont *subHeaderFont =
      [NSFont systemFontOfSize:14 weight:NSFontWeightMedium];
  NSFont *labelFont = [NSFont monospacedDigitSystemFontOfSize:16
                                                        weight:NSFontWeightMedium];
  NSFont *valueFont = [NSFont monospacedDigitSystemFontOfSize:16
                                                        weight:NSFontWeightBold];
  NSFont *smallFont = [NSFont monospacedDigitSystemFontOfSize:13
                                                        weight:NSFontWeightMedium];

  NSDictionary *headerAttrs = @{
    NSFontAttributeName : headerFont,
    NSForegroundColorAttributeName : overlayBrightText()
  };
  NSDictionary *subHeaderAttrs = @{
    NSFontAttributeName : subHeaderFont,
    NSForegroundColorAttributeName : overlayDimText()
  };
  NSDictionary *labelAttrs = @{
    NSFontAttributeName : labelFont,
    NSForegroundColorAttributeName : overlayNeonGreen()
  };
  NSDictionary *smallAttrs = @{
    NSFontAttributeName : smallFont,
    NSForegroundColorAttributeName : overlayDimText()
  };

  // ---- mactop header ----
  NSString *title = @"mactop";
  NSDictionary *titleAttrs = @{
    NSFontAttributeName : [NSFont systemFontOfSize:17 weight:NSFontWeightHeavy],
    NSForegroundColorAttributeName : overlayNeonGreen()
  };
  NSSize titleSize = [title sizeWithAttributes:titleAttrs];
  [title drawAtPoint:NSMakePoint(padX, y) withAttributes:titleAttrs];

  // Dot separator
  NSString *dot = @"•";
  NSDictionary *dotAttrs = @{
    NSFontAttributeName : [NSFont systemFontOfSize:14 weight:NSFontWeightRegular],
    NSForegroundColorAttributeName :
        [NSColor colorWithWhite:0.5 alpha:1.0]
  };
  [dot drawAtPoint:NSMakePoint(padX + titleSize.width + 5, y + 1.5)
      withAttributes:dotAttrs];

  // Model name
  NSString *modelName =
      [NSString stringWithUTF8String:m.model_name];
  if (modelName.length == 0)
    modelName = localize(@"Menu_AppleSilicon");
  NSSize dotSize = [dot sizeWithAttributes:dotAttrs];
  [modelName
      drawAtPoint:NSMakePoint(padX + titleSize.width + 5 + dotSize.width + 5,
                               y + 0.5)
      withAttributes:subHeaderAttrs];

  // Gear icon (right-aligned in header)
  {
    NSString *gear = @"⚙";
    NSDictionary *gearAttrs = @{
      NSFontAttributeName : [NSFont systemFontOfSize:20 weight:NSFontWeightRegular],
      NSForegroundColorAttributeName : g_showSettings
          ? overlayNeonGreen()
          : [NSColor colorWithWhite:0.55 alpha:1.0]
    };
    NSSize gearSize = [gear sizeWithAttributes:gearAttrs];
    CGFloat gearX = padX + contentW - gearSize.width;
    CGFloat gearY = y - 3;
    [gear drawAtPoint:NSMakePoint(gearX, gearY) withAttributes:gearAttrs];
    g_gearHitRect = NSMakeRect(gearX - 6, gearY - 4, gearSize.width + 12, gearSize.height + 8);
  }
  y += 26;

  // Core summary line
  NSMutableString *coreSummary = [NSMutableString string];
  if (m.e_core_count > 0)
    [coreSummary appendFormat:@"%dE", m.e_core_count];
  if (m.p_core_count > 0) {
    if (coreSummary.length > 0)
      [coreSummary appendString:@"/"];
    [coreSummary appendFormat:@"%dP", m.p_core_count];
  }
  if (m.s_core_count > 0) {
    if (coreSummary.length > 0)
      [coreSummary appendString:@"/"];
    [coreSummary appendFormat:@"%dS", m.s_core_count];
  }
  if (m.gpu_core_count > 0) {
    [coreSummary appendFormat:@" • %@", [NSString stringWithFormat:localize(@"Overlay_GPUCores"), m.gpu_core_count]];
  }
  [coreSummary drawAtPoint:NSMakePoint(padX, y)
               withAttributes:smallAttrs];
  y += 22;

  // ---- Settings Panel ----
  if (g_showSettings) {
    g_settingsHitCount = 0;
    g_dragHandleHitCount = 0;

    // Separator
    [[NSColor colorWithWhite:1.0 alpha:0.08] set];
    [NSBezierPath fillRect:NSMakeRect(padX, y, contentW, 1)];
    y += 8;

    NSFont *sectionTitleFont = [NSFont systemFontOfSize:14 weight:NSFontWeightBold];
    NSFont *checkLabelFont = [NSFont systemFontOfSize:13 weight:NSFontWeightMedium];
    CGFloat checkRowH = 26;
    CGFloat checkSize = 14;
    CGFloat gripW = 20; // Width of drag handle area

    int nCollapsed = g_settingsCollapsedOrderCount;
    int nExpanded = g_settingsExpandedOrderCount;

    // --- Collapsed Mode ---
    {
      NSString *header = localize(@"Overlay_CollapsedMode");
      NSDictionary *hdrAttrs = @{
        NSFontAttributeName : sectionTitleFont,
        NSForegroundColorAttributeName : overlayNeonGreen()
      };
      [header drawAtPoint:NSMakePoint(padX, y) withAttributes:hdrAttrs];
      y += 22;

      g_collapsedListOriginY = y;

      for (int i = 0; i < nCollapsed; i++) {
        OverlaySectionID sid = g_settingsCollapsedOrder[i];
        BOOL isOn = g_settingsCollapsed[sid];
        BOOL isDragSource = (g_dragActive && g_dragIsCollapsed && g_dragSourceIdx == i);

        // Draw insertion indicator if dragging
        if (g_dragActive && g_dragIsCollapsed && g_dragInsertIdx == i && g_dragInsertIdx != g_dragSourceIdx) {
          [overlayNeonGreen() setFill];
          [NSBezierPath fillRect:NSMakeRect(padX, y - 1.5, contentW, 3)];
        }

        // Dim the source row during drag
        CGFloat rowAlpha = isDragSource ? 0.3 : 1.0;

        // Checkbox
        NSRect checkRect = NSMakeRect(padX, y + (checkRowH - checkSize) / 2.0, checkSize, checkSize);
        NSBezierPath *box = [NSBezierPath bezierPathWithRoundedRect:checkRect xRadius:3 yRadius:3];
        if (isOn) {
          [[overlayNeonGreen() colorWithAlphaComponent:rowAlpha] setFill];
          [box fill];
          NSBezierPath *check = [NSBezierPath bezierPath];
          [check moveToPoint:NSMakePoint(checkRect.origin.x + 3, checkRect.origin.y + checkSize / 2.0)];
          [check lineToPoint:NSMakePoint(checkRect.origin.x + checkSize * 0.4, checkRect.origin.y + checkSize - 3)];
          [check lineToPoint:NSMakePoint(checkRect.origin.x + checkSize - 2, checkRect.origin.y + 3)];
          [[NSColor colorWithRed:0.05 green:0.05 blue:0.05 alpha:rowAlpha] setStroke];
          [check setLineWidth:2.0];
          [check setLineCapStyle:NSLineCapStyleRound];
          [check setLineJoinStyle:NSLineJoinStyleRound];
          [check stroke];
        } else {
          [[NSColor colorWithWhite:0.3 alpha:rowAlpha] setStroke];
          [box setLineWidth:1.5];
          [box stroke];
        }

        // Label
        NSString *label = [NSString stringWithUTF8String:sectionDisplayName(sid)];
        NSDictionary *lblAttrs = @{
          NSFontAttributeName : checkLabelFont,
          NSForegroundColorAttributeName : [isOn ? overlayBrightText() : [NSColor colorWithWhite:0.45 alpha:1.0] colorWithAlphaComponent:rowAlpha]
        };
        [label drawAtPoint:NSMakePoint(padX + checkSize + 8, y + 4) withAttributes:lblAttrs];

        // Drag grip handle (≡ three horizontal lines) on right side
        {
          CGFloat gripX = padX + contentW - gripW;
          CGFloat gripCenterY = y + checkRowH / 2.0;
          CGFloat lineW = 12;
          CGFloat lineGap = 3.5;
          NSColor *gripColor = isDragSource
            ? overlayNeonGreen()
            : [NSColor colorWithWhite:0.4 alpha:rowAlpha];
          [gripColor setStroke];
          for (int line = -1; line <= 1; line++) {
            NSBezierPath *gripLine = [NSBezierPath bezierPath];
            CGFloat ly = gripCenterY + line * lineGap;
            [gripLine moveToPoint:NSMakePoint(gripX + (gripW - lineW) / 2.0, ly)];
            [gripLine lineToPoint:NSMakePoint(gripX + (gripW + lineW) / 2.0, ly)];
            [gripLine setLineWidth:1.5];
            [gripLine setLineCapStyle:NSLineCapStyleRound];
            [gripLine stroke];
          }

          // Store drag handle hit rect
          if (g_dragHandleHitCount < MAX_SETTINGS_ROWS) {
            g_dragHandleHitRects[g_dragHandleHitCount] = NSMakeRect(gripX - 4, y, gripW + 8, checkRowH);
            g_dragHandleHitSectionIdx[g_dragHandleHitCount] = i;
            g_dragHandleHitIsCollapsed[g_dragHandleHitCount] = YES;
            g_dragHandleHitCount++;
          }
        }

        // Store checkbox hit rect (excluding grip area)
        if (g_settingsHitCount < MAX_SETTINGS_ROWS) {
          g_settingsHitRects[g_settingsHitCount] = NSMakeRect(padX, y, contentW - gripW - 8, checkRowH);
          g_settingsHitSectionID[g_settingsHitCount] = sid;
          g_settingsHitIsCollapsed[g_settingsHitCount] = YES;
          g_settingsHitCount++;
        }

        y += checkRowH;
      }

      // Insertion indicator at end of collapsed list
      if (g_dragActive && g_dragIsCollapsed && g_dragInsertIdx == nCollapsed && g_dragInsertIdx != g_dragSourceIdx) {
        [overlayNeonGreen() setFill];
        [NSBezierPath fillRect:NSMakeRect(padX, y - 1.5, contentW, 3)];
      }
    }

    y += 6;
    [[NSColor colorWithWhite:1.0 alpha:0.06] set];
    [NSBezierPath fillRect:NSMakeRect(padX, y, contentW, 1)];
    y += 8;

    // --- Expanded Mode ---
    {
      NSString *header = localize(@"Overlay_ExpandedMode");
      NSDictionary *hdrAttrs = @{
        NSFontAttributeName : sectionTitleFont,
        NSForegroundColorAttributeName : overlayNeonGreen()
      };
      [header drawAtPoint:NSMakePoint(padX, y) withAttributes:hdrAttrs];
      y += 22;

      g_expandedListOriginY = y;

      for (int i = 0; i < nExpanded; i++) {
        OverlaySectionID sid = g_settingsExpandedOrder[i];
        BOOL isOn = g_settingsExpanded[sid];
        BOOL isDragSource = (g_dragActive && !g_dragIsCollapsed && g_dragSourceIdx == i);

        // Draw insertion indicator if dragging
        if (g_dragActive && !g_dragIsCollapsed && g_dragInsertIdx == i && g_dragInsertIdx != g_dragSourceIdx) {
          [overlayNeonGreen() setFill];
          [NSBezierPath fillRect:NSMakeRect(padX, y - 1.5, contentW, 3)];
        }

        CGFloat rowAlpha = isDragSource ? 0.3 : 1.0;

        NSRect checkRect = NSMakeRect(padX, y + (checkRowH - checkSize) / 2.0, checkSize, checkSize);
        NSBezierPath *box = [NSBezierPath bezierPathWithRoundedRect:checkRect xRadius:3 yRadius:3];
        if (isOn) {
          [[overlayAccentCyan() colorWithAlphaComponent:rowAlpha] setFill];
          [box fill];
          NSBezierPath *check = [NSBezierPath bezierPath];
          [check moveToPoint:NSMakePoint(checkRect.origin.x + 3, checkRect.origin.y + checkSize / 2.0)];
          [check lineToPoint:NSMakePoint(checkRect.origin.x + checkSize * 0.4, checkRect.origin.y + checkSize - 3)];
          [check lineToPoint:NSMakePoint(checkRect.origin.x + checkSize - 2, checkRect.origin.y + 3)];
          [[NSColor colorWithRed:0.05 green:0.05 blue:0.05 alpha:rowAlpha] setStroke];
          [check setLineWidth:2.0];
          [check setLineCapStyle:NSLineCapStyleRound];
          [check setLineJoinStyle:NSLineJoinStyleRound];
          [check stroke];
        } else {
          [[NSColor colorWithWhite:0.3 alpha:rowAlpha] setStroke];
          [box setLineWidth:1.5];
          [box stroke];
        }

        NSString *label = [NSString stringWithUTF8String:sectionDisplayName(sid)];
        NSDictionary *lblAttrs = @{
          NSFontAttributeName : checkLabelFont,
          NSForegroundColorAttributeName : [isOn ? overlayBrightText() : [NSColor colorWithWhite:0.45 alpha:1.0] colorWithAlphaComponent:rowAlpha]
        };
        [label drawAtPoint:NSMakePoint(padX + checkSize + 8, y + 4) withAttributes:lblAttrs];

        // Drag grip handle
        {
          CGFloat gripX = padX + contentW - gripW;
          CGFloat gripCenterY = y + checkRowH / 2.0;
          CGFloat lineW = 12;
          CGFloat lineGap = 3.5;
          NSColor *gripColor = isDragSource
            ? overlayAccentCyan()
            : [NSColor colorWithWhite:0.4 alpha:rowAlpha];
          [gripColor setStroke];
          for (int line = -1; line <= 1; line++) {
            NSBezierPath *gripLine = [NSBezierPath bezierPath];
            CGFloat ly = gripCenterY + line * lineGap;
            [gripLine moveToPoint:NSMakePoint(gripX + (gripW - lineW) / 2.0, ly)];
            [gripLine lineToPoint:NSMakePoint(gripX + (gripW + lineW) / 2.0, ly)];
            [gripLine setLineWidth:1.5];
            [gripLine setLineCapStyle:NSLineCapStyleRound];
            [gripLine stroke];
          }

          if (g_dragHandleHitCount < MAX_SETTINGS_ROWS) {
            g_dragHandleHitRects[g_dragHandleHitCount] = NSMakeRect(gripX - 4, y, gripW + 8, checkRowH);
            g_dragHandleHitSectionIdx[g_dragHandleHitCount] = i;
            g_dragHandleHitIsCollapsed[g_dragHandleHitCount] = NO;
            g_dragHandleHitCount++;
          }
        }

        if (g_settingsHitCount < MAX_SETTINGS_ROWS) {
          g_settingsHitRects[g_settingsHitCount] = NSMakeRect(padX, y, contentW - gripW - 8, checkRowH);
          g_settingsHitSectionID[g_settingsHitCount] = sid;
          g_settingsHitIsCollapsed[g_settingsHitCount] = NO;
          g_settingsHitCount++;
        }

        y += checkRowH;
      }

      // Insertion indicator at end of expanded list
      if (g_dragActive && !g_dragIsCollapsed && g_dragInsertIdx == nExpanded && g_dragInsertIdx != g_dragSourceIdx) {
        [overlayNeonGreen() setFill];
        [NSBezierPath fillRect:NSMakeRect(padX, y - 1.5, contentW, 3)];
      }
    }

    y += 10;

    // Done button
    {
      CGFloat btnW = 120;
      CGFloat btnH = 32;
      CGFloat btnX = (W - btnW) / 2.0;
      NSRect btnRect = NSMakeRect(btnX, y, btnW, btnH);
      g_settingsDoneRect = btnRect;

      NSBezierPath *btnPath = [NSBezierPath bezierPathWithRoundedRect:btnRect xRadius:8 yRadius:8];
      [overlayNeonGreen() setFill];
      [btnPath fill];

      NSString *doneText = localize(@"Overlay_Done");
      NSDictionary *doneAttrs = @{
        NSFontAttributeName : [NSFont systemFontOfSize:14 weight:NSFontWeightBold],
        NSForegroundColorAttributeName : [NSColor colorWithRed:0.05 green:0.05 blue:0.05 alpha:1.0]
      };
      NSSize doneSize = [doneText sizeWithAttributes:doneAttrs];
      [doneText drawAtPoint:NSMakePoint(btnX + (btnW - doneSize.width) / 2.0, y + (btnH - doneSize.height) / 2.0)
          withAttributes:doneAttrs];
      y += btnH + 10;
    }

    // No toggle arrow in settings mode — done button handles dismiss
    return;
  }

  // Separator
  [[NSColor colorWithWhite:1.0 alpha:0.08] set];
  [NSBezierPath fillRect:NSMakeRect(padX, y, contentW, 1)];
  y += 6;

  // ---- Metric rows ----
  CGFloat rowH = 28;
  CGFloat barX = padX + 90;
  CGFloat barW = contentW - 90 - 65; // Leave room for value text
  CGFloat barH = 7;
  CGFloat sparkW = 60;
  CGFloat sparkH = 20;

  // Helper block for labeled metric row with bar
  void (^drawMetricBar)(NSString *, double, NSColor *, double *, BOOL) =
      ^(NSString *label, double pct, NSColor *color, double *sparkData,
        BOOL showSpark) {
        // Label
        [label drawAtPoint:NSMakePoint(padX, y + 4) withAttributes:labelAttrs];

        // Bar
        drawMiniBar(barX, y + 10, barW - (showSpark ? sparkW + 8 : 0), barH,
                    pct, color);

        // Sparkline
        if (showSpark && sparkData) {
          drawMiniSparkline(sparkData, OVERLAY_SPARKLINE_HISTORY,
                            padX + contentW - sparkW - 48, y + 2, sparkW,
                            sparkH, color, 100.0);
        }

        // Value
        NSString *val = [NSString stringWithFormat:@"%.0f%%", pct];
        NSDictionary *valAttrs = @{
          NSFontAttributeName : valueFont,
          NSForegroundColorAttributeName : colorForPercent(pct)
        };
        NSSize valSize = [val sizeWithAttributes:valAttrs];
        [val drawAtPoint:NSMakePoint(padX + contentW - valSize.width, y + 3)
            withAttributes:valAttrs];

        y += rowH;
      };

  // Helper block for labeled key-value row
  void (^drawMetricKV)(NSString *, NSString *, NSColor *) =
      ^(NSString *label, NSString *value, NSColor *color) {
        [label drawAtPoint:NSMakePoint(padX, y + 4)
            withAttributes:labelAttrs];
        NSDictionary *valAttrs = @{
          NSFontAttributeName : valueFont,
          NSForegroundColorAttributeName : color
        };
        NSSize valSize = [value sizeWithAttributes:valAttrs];
        [value drawAtPoint:NSMakePoint(padX + contentW - valSize.width, y + 3)
            withAttributes:valAttrs];
        y += rowH;
      };

  // Section draw blocks — each renders one section at the current y position
  void (^drawSectionFPS)(void) = ^{
    NSString *fpsLabel = [NSString stringWithUTF8String:sectionDisplayName(kSectionFPS)];
    [fpsLabel drawAtPoint:NSMakePoint(padX, y + 4) withAttributes:labelAttrs];

    if (g_fpsStreamFailed) {
      NSString *warn = localize(@"Overlay_RequiresScreenRecordingPermission");
      NSDictionary *warnAttrs = @{
        NSFontAttributeName : [NSFont systemFontOfSize:13 weight:NSFontWeightMedium],
        NSForegroundColorAttributeName : overlayAccentRed()
      };
      NSSize warnSize = [warn sizeWithAttributes:warnAttrs];
      [warn drawAtPoint:NSMakePoint(padX + contentW - warnSize.width, y + 5)
          withAttributes:warnAttrs];
      y += rowH;
      return;
    }

    uint32_t fps = atomic_load(&g_fpsValue);
    NSString *fpsVal = [NSString stringWithFormat:@"%u", fps];
    NSDictionary *fpsAttrs = @{
      NSFontAttributeName : valueFont,
      NSForegroundColorAttributeName : overlayAccentCyan()
    };
    NSSize fpsSize = [fpsVal sizeWithAttributes:fpsAttrs];
    [fpsVal drawAtPoint:NSMakePoint(padX + contentW - fpsSize.width, y + 3)
        withAttributes:fpsAttrs];

    double fpsMax = 60.0;
    for (int i = 0; i < OVERLAY_SPARKLINE_HISTORY; i++) {
      if (fpsSparkHistory[i] > fpsMax) fpsMax = fpsSparkHistory[i];
    }
    fpsMax = ceil(fpsMax / 30.0) * 30.0;
    if (fpsMax < 60.0) fpsMax = 60.0;
    
    NSSize fpsLabelSize = [fpsLabel sizeWithAttributes:labelAttrs];
    CGFloat fpsLabelW = MAX(50.0, fpsLabelSize.width + 10.0);
    
    CGFloat fpsValW = fpsSize.width + 8;
    CGFloat fpsSparkW = contentW - fpsLabelW - fpsValW;
    drawMiniSparkline(fpsSparkHistory, OVERLAY_SPARKLINE_HISTORY,
                      padX + fpsLabelW, y + 2, fpsSparkW,
                      sparkH, overlayAccentCyan(), fpsMax);
    y += rowH;
  };

  void (^drawSectionFrame)(void) = ^{
    NSString *fiLabel = [NSString stringWithUTF8String:sectionDisplayName(kSectionFrame)];
    [fiLabel drawAtPoint:NSMakePoint(padX, y + 4) withAttributes:labelAttrs];

    if (g_fpsStreamFailed) {
      NSString *warn = localize(@"Overlay_RequiresPermission");
      NSDictionary *warnAttrs = @{
        NSFontAttributeName : [NSFont systemFontOfSize:13 weight:NSFontWeightMedium],
        NSForegroundColorAttributeName : overlayAccentRed()
      };
      NSSize warnSize = [warn sizeWithAttributes:warnAttrs];
      [warn drawAtPoint:NSMakePoint(padX + contentW - warnSize.width, y + 5)
          withAttributes:warnAttrs];
      y += rowH;
      return;
    }

    uint32_t frameIntUs = atomic_load(&g_frameIntervalUs);
    double frameMs = frameIntUs / 1000.0;

    NSString *fiVal;
    if (frameIntUs > 0) {
      fiVal = [NSString stringWithFormat:@"%.1fms", frameMs];
    } else {
      fiVal = @"—";
    }
    NSColor *fiColor;
    if (frameIntUs == 0) {
      fiColor = overlayDimText();
    } else if (frameMs > 20.0) {
      fiColor = overlayAccentRed();
    } else if (frameMs > 11.0) {
      fiColor = overlayAccentYellow();
    } else {
      fiColor = overlayAccentGreen();
    }
    NSDictionary *fiAttrs = @{
      NSFontAttributeName : valueFont,
      NSForegroundColorAttributeName : fiColor
    };
    NSSize fiSize = [fiVal sizeWithAttributes:fiAttrs];
    [fiVal drawAtPoint:NSMakePoint(padX + contentW - fiSize.width, y + 3)
        withAttributes:fiAttrs];

    double fiMax = 16.7;
    for (int i = 0; i < OVERLAY_SPARKLINE_HISTORY; i++) {
      if (frameIntSparkHistory[i] > fiMax) fiMax = frameIntSparkHistory[i];
    }
    fiMax = ceil(fiMax / 5.0) * 5.0;
    if (fiMax < 10.0) fiMax = 10.0;
    
    NSSize fiLabelSize = [fiLabel sizeWithAttributes:labelAttrs];
    CGFloat fiLabelW = MAX(65.0, fiLabelSize.width + 10.0);
    
    CGFloat fiValW = fiSize.width + 8;
    CGFloat fiSparkW = contentW - fiLabelW - fiValW;
    drawMiniSparkline(frameIntSparkHistory, OVERLAY_SPARKLINE_HISTORY,
                      padX + fiLabelW, y + 2, fiSparkW,
                      sparkH, overlayAccentOrange(), fiMax);
    y += rowH;
  };

  void (^drawSectionCPU)(void) = ^{
    drawMetricBar([NSString stringWithUTF8String:sectionDisplayName(kSectionCPU)], m.cpu_percent, overlayAccentGreen(), cpuSparkHistory, YES);
  };

  void (^drawSectionGPU)(void) = ^{
    drawMetricBar([NSString stringWithUTF8String:sectionDisplayName(kSectionGPU)], m.gpu_percent, overlayAccentOrange(), gpuSparkHistory, YES);
  };

  void (^drawSectionANE)(void) = ^{
    drawMetricBar([NSString stringWithUTF8String:sectionDisplayName(kSectionANE)], m.ane_percent, overlayAccentCyan(), NULL, NO);
  };

  void (^drawSectionMemory)(void) = ^{
    double memGB = (double)m.mem_used_bytes / (1024.0 * 1024.0 * 1024.0);
    double totalGB = (double)m.mem_total_bytes / (1024.0 * 1024.0 * 1024.0);
    double memPct = totalGB > 0 ? (memGB / totalGB) * 100.0 : 0;
    NSString *memStr =
        [NSString stringWithFormat:@"%.1f/%.0fGB", memGB, totalGB];
    [([NSString stringWithUTF8String:sectionDisplayName(kSectionMemory)]) drawAtPoint:NSMakePoint(padX, y + 4)
                 withAttributes:labelAttrs];
    drawMiniBar(barX, y + 10, barW - sparkW - 8, barH, memPct,
                overlayAccentPurple());
    NSDictionary *valAttrs = @{
      NSFontAttributeName : valueFont,
      NSForegroundColorAttributeName : colorForPercent(memPct)
    };
    NSSize valSize = [memStr sizeWithAttributes:valAttrs];
    [memStr drawAtPoint:NSMakePoint(padX + contentW - valSize.width, y + 3)
        withAttributes:valAttrs];
    y += rowH;
  };

  void (^drawSectionSwap)(void) = ^{
    if (m.swap_used_bytes > 0 && m.swap_total_bytes > 0) {
      double swapGB =
          (double)m.swap_used_bytes / (1024.0 * 1024.0 * 1024.0);
      double swapTotalGB =
          (double)m.swap_total_bytes / (1024.0 * 1024.0 * 1024.0);
      double swapPct = swapTotalGB > 0 ? (swapGB / swapTotalGB) * 100.0 : 0;
      NSString *swapStr =
          [NSString stringWithFormat:@"%.1f/%.0fGB", swapGB, swapTotalGB];
      [([NSString stringWithUTF8String:sectionDisplayName(kSectionSwap)]) drawAtPoint:NSMakePoint(padX, y + 4)
                 withAttributes:labelAttrs];
    drawMiniBar(barX, y + 10, barW - sparkW - 8, barH, swapPct,
                overlayAccentOrange());
      NSDictionary *swapValAttrs = @{
        NSFontAttributeName : valueFont,
        NSForegroundColorAttributeName : colorForPercent(swapPct)
      };
      NSSize swapValSize = [swapStr sizeWithAttributes:swapValAttrs];
      [swapStr drawAtPoint:NSMakePoint(padX + contentW - swapValSize.width, y + 3)
          withAttributes:swapValAttrs];
      y += rowH;
    }
  };

  void (^drawSectionPower)(void) = ^{
    NSString *powerStr =
        [NSString stringWithFormat:@"%.1fW", m.package_watts];
    drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionPower)], powerStr, overlayAccentYellow());
    drawMetricKV([NSString stringWithFormat:@"  %@", localize(@"Menu_CPU")], [NSString stringWithFormat:@"%.1fW", m.cpu_watts], overlayDimText());
    drawMetricKV([NSString stringWithFormat:@"  %@", localize(@"Menu_GPU")], [NSString stringWithFormat:@"%.1fW", m.gpu_watts], overlayDimText());
    drawMetricKV([NSString stringWithFormat:@"  %@", localize(@"Overlay_ANE")], [NSString stringWithFormat:@"%.1fW", m.ane_watts], overlayDimText());
    drawMetricKV([NSString stringWithFormat:@"  %@", localize(@"Overlay_DRAM")], [NSString stringWithFormat:@"%.1fW", m.dram_watts], overlayDimText());
  };

  void (^drawSectionBandwidth)(void) = ^{
    NSString *bwStr =
        [NSString stringWithFormat:@"%.1f GB/s", m.dram_bw_combined_gbs];
    drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionBandwidth)], bwStr, overlayAccentBlue());
  };

  void (^drawSectionGPUFreq)(void) = ^{
    NSString *freqStr;
    if (m.tflops_fp32 > 0) {
      freqStr = [NSString stringWithFormat:localize(@"Overlay_GPUFreqTFLOPs"),
                                          m.gpu_freq_mhz, m.tflops_fp32];
    } else {
      freqStr = [NSString stringWithFormat:localize(@"Overlay_GPUFreqMHz"),
                                          m.gpu_freq_mhz];
    }
    drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionGPUFreq)], freqStr, overlayAccentOrange());
  };

  void (^drawSectionTemps)(void) = ^{
    NSString *tempStr;
    if (m.gpu_temp > 0) {
      tempStr = [NSString stringWithFormat:localize(@"Overlay_TempsDual"),
                                          m.cpu_temp, m.gpu_temp];
    } else {
      tempStr = [NSString stringWithFormat:localize(@"Overlay_TempsSingle"),
                                          m.cpu_temp];
    }
    NSColor *tempColor = overlayBrightText();
    if (m.cpu_temp >= 90 || m.gpu_temp >= 90)
      tempColor = overlayAccentRed();
    else if (m.cpu_temp >= 70 || m.gpu_temp >= 70)
      tempColor = overlayAccentYellow();
    drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionTemps)], tempStr, tempColor);
  };

  void (^drawSectionThermal)(void) = ^{
    NSString *thermalStr =
        [NSString stringWithUTF8String:m.thermal_state];
    if (thermalStr.length == 0)
      thermalStr = localize(@"Metrics_ThermalUnknown");
    NSColor *thermalColor = overlayAccentGreen();
    // Use numeric level for color (language-independent)
    if (m.thermal_level >= 2) // serious or critical
      thermalColor = overlayAccentRed();
    else if (m.thermal_level == 1) // fair
      thermalColor = overlayAccentYellow();
    drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionThermal)], thermalStr, thermalColor);
  };

  void (^drawSectionFans)(void) = ^{
    if (m.fan_count > 0) {
      NSMutableString *fanStr = [NSMutableString string];
      for (int i = 0; i < m.fan_count && i < 4; i++) {
        if (i > 0)
          [fanStr appendString:@"  "];
        [fanStr appendFormat:@"%dRPM", m.fan_rpm[i]];
      }
      drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionFans)], fanStr, overlayDimText());
    }
  };

  void (^drawSectionNetwork)(void) = ^{
    NSString *netStr = [NSString
        stringWithFormat:@"↓%@ ↑%@",
                         formatOverlayThroughput(m.net_in_bytes_per_sec),
                         formatOverlayThroughput(m.net_out_bytes_per_sec)];
    drawMetricKV([NSString stringWithUTF8String:sectionDisplayName(kSectionNetwork)], netStr, overlayDimText());
  };

  // Dispatch table: section ID → draw block
  void (^sectionDrawers[kSectionCount])(void);
  sectionDrawers[kSectionFPS] = drawSectionFPS;
  sectionDrawers[kSectionFrame] = drawSectionFrame;
  sectionDrawers[kSectionCPU] = drawSectionCPU;
  sectionDrawers[kSectionGPU] = drawSectionGPU;
  sectionDrawers[kSectionANE] = drawSectionANE;
  sectionDrawers[kSectionMemory] = drawSectionMemory;
  sectionDrawers[kSectionSwap] = drawSectionSwap;
  sectionDrawers[kSectionPower] = drawSectionPower;
  sectionDrawers[kSectionBandwidth] = drawSectionBandwidth;
  sectionDrawers[kSectionGPUFreq] = drawSectionGPUFreq;
  sectionDrawers[kSectionTemps] = drawSectionTemps;
  sectionDrawers[kSectionThermal] = drawSectionThermal;
  sectionDrawers[kSectionFans] = drawSectionFans;
  sectionDrawers[kSectionNetwork] = drawSectionNetwork;

  // Choose which section list to iterate based on collapsed/expanded
  if (g_overlay_expanded) {
    // Expanded mode: iterate expanded section list
    BOOL needsSeparator = NO;
    for (int i = 0; i < g_expandedCount; i++) {
      OverlaySectionID sid = g_expandedSections[i];

      // Draw separator before "detail" sections (power/bandwidth/gpu_freq and temps/thermal/fans/network)
      if (!needsSeparator && (sid == kSectionPower || sid == kSectionBandwidth || sid == kSectionGPUFreq)) {
        [[NSColor colorWithWhite:1.0 alpha:0.06] set];
        [NSBezierPath fillRect:NSMakeRect(padX, y, contentW, 1)];
        y += 5;
        needsSeparator = YES;
      }

      if (sectionShouldDraw(sid, cfg) && sectionDrawers[sid]) {
        sectionDrawers[sid]();
      }
    }
  } else {
    // Collapsed mode: iterate collapsed section list only
    for (int i = 0; i < g_collapsedCount; i++) {
      OverlaySectionID sid = g_collapsedSections[i];

      if (sectionShouldDraw(sid, cfg) && sectionDrawers[sid]) {
        sectionDrawers[sid]();
      }
    }
  }

  // Draw toggle arrow (liquid glass pill)
  CGFloat arrowW = 40;
  CGFloat arrowH = 16;
  CGFloat arrowX = (W - arrowW) / 2.0;
  CGFloat arrowY = self.bounds.size.height - arrowH - 8;

  NSBezierPath *pill = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(arrowX, arrowY, arrowW, arrowH) xRadius:arrowH/2.0 yRadius:arrowH/2.0];
  [[[NSColor whiteColor] colorWithAlphaComponent:0.08] setFill];
  [pill fill];
  [[[NSColor whiteColor] colorWithAlphaComponent:0.15] setStroke];
  [pill setLineWidth:1.0];
  [pill stroke];

  NSBezierPath *chevron = [NSBezierPath bezierPath];
  CGFloat cx = arrowX + arrowW / 2.0;
  CGFloat cy = arrowY + arrowH / 2.0;
  if (g_overlay_expanded) {
    [chevron moveToPoint:NSMakePoint(cx - 5, cy + 2)];
    [chevron lineToPoint:NSMakePoint(cx, cy - 3)];
    [chevron lineToPoint:NSMakePoint(cx + 5, cy + 2)];
  } else {
    [chevron moveToPoint:NSMakePoint(cx - 5, cy - 2)];
    [chevron lineToPoint:NSMakePoint(cx, cy + 3)];
    [chevron lineToPoint:NSMakePoint(cx + 5, cy - 2)];
  }
  [[[NSColor whiteColor] colorWithAlphaComponent:0.7] setStroke];
  [chevron setLineWidth:2.0];
  [chevron setLineCapStyle:NSLineCapStyleRound];
  [chevron setLineJoinStyle:NSLineJoinStyleRound];
  [chevron stroke];

  // Opacity indicator (flashes briefly when scroll-wheel adjusts opacity)
  if (g_opacityFlashCountdown > 0) {
    g_opacityFlashCountdown--;
    CGFloat opacityPct = g_overlay_config.opacity * 100.0;
    NSString *opacityStr =
        [NSString stringWithFormat:localize(@"Overlay_OpacityHint"), opacityPct];
    NSDictionary *opAttrs = @{
      NSFontAttributeName : [NSFont systemFontOfSize:11 weight:NSFontWeightMedium],
      NSForegroundColorAttributeName :
          [NSColor colorWithWhite:0.6 alpha:0.8]
    };
    NSSize opSize = [opacityStr sizeWithAttributes:opAttrs];
    [opacityStr
        drawAtPoint:NSMakePoint((W - opSize.width) / 2.0, y + 2)
        withAttributes:opAttrs];
  }
}

@end

// ---------- C API ----------

int initOverlay(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

    // Calculate initial height based on enabled sections
    CGFloat estimatedHeight = 550; // Base height for header + always-on sections
    // Each section adds roughly 28px with larger text
    if (g_overlay_config.show_fans)
      estimatedHeight += 28;
    if (g_overlay_config.show_network)
      estimatedHeight += 28;
    if (g_overlay_config.show_bandwidth)
      estimatedHeight += 28;
    if (g_overlay_config.show_gpu_freq)
      estimatedHeight += 28;

    CGFloat overlayW = 460;
    CGFloat overlayH = estimatedHeight;

    // Position in top-left with padding
    NSScreen *screen = [NSScreen mainScreen];
    NSRect screenFrame = screen.visibleFrame;
    CGFloat posX = screenFrame.origin.x + 16;
    CGFloat posY = screenFrame.origin.y + screenFrame.size.height - overlayH - 16;

    NSRect frame = NSMakeRect(posX, posY, overlayW, overlayH);

    g_overlayWindow = [[OverlayWindow alloc]
        initWithContentRect:frame
                  styleMask:NSWindowStyleMaskBorderless
                    backing:NSBackingStoreBuffered
                      defer:NO];

    g_overlayWindow.level = NSStatusWindowLevel + 1;
    g_overlayWindow.opaque = NO;
    g_overlayWindow.hasShadow = YES;
    g_overlayWindow.ignoresMouseEvents = NO;
    g_overlayWindow.backgroundColor = [NSColor clearColor];
    g_overlayWindow.alphaValue = g_overlay_config.opacity;

    // Appear on all Spaces, including fullscreen
    g_overlayWindow.collectionBehavior =
        NSWindowCollectionBehaviorCanJoinAllSpaces |
        NSWindowCollectionBehaviorStationary |
        NSWindowCollectionBehaviorFullScreenAuxiliary |
        NSWindowCollectionBehaviorIgnoresCycle;

    // Solid black background with rounded corners
    NSView *bgView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, overlayW, overlayH)];
    bgView.wantsLayer = YES;
    bgView.layer.backgroundColor = [[NSColor colorWithRed:0.05 green:0.05 blue:0.05 alpha:0.92] CGColor];
    bgView.layer.cornerRadius = 14.0;
    bgView.layer.masksToBounds = YES;
    bgView.layer.borderWidth = 1.0;
    bgView.layer.borderColor = [[NSColor colorWithRed:0.15 green:1.0 blue:0.30 alpha:0.3] CGColor];
    bgView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    g_overlayWindow.contentView = bgView;

    // Content view for drawing metrics
    g_contentView = [[OverlayContentView alloc]
        initWithFrame:NSMakeRect(0, 0, overlayW, overlayH)];
    g_contentView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [bgView addSubview:g_contentView];

    [g_overlayWindow orderFrontRegardless];

    // Start FPS counter
    startFPSCounter();

    return 0;
  }
}

void setOverlayConfig(overlay_config_t *cfg) {
  if (cfg) {
    g_overlay_config = *cfg;
    // Parse ordered section lists from the config strings
    parseSectionList(g_overlay_config.collapsed_sections, g_collapsedSections, &g_collapsedCount);
    parseSectionList(g_overlay_config.expanded_order, g_expandedSections, &g_expandedCount);
  }
}

void updateOverlayMetrics(overlay_metrics_t *m) {
  if (!m)
    return;
  // Copy the metrics struct BEFORE dispatching to avoid use-after-free.
  // The pointer m comes from a Go stack variable that may be deallocated
  // before dispatch_async fires on the main queue.
  overlay_metrics_t localMetrics = *m;
  dispatch_async(dispatch_get_main_queue(), ^{
    g_overlay_metrics = localMetrics;
    pushSparkHistory(cpuSparkHistory, localMetrics.cpu_percent);
    pushSparkHistory(gpuSparkHistory, localMetrics.gpu_percent);
    pushSparkHistory(fpsSparkHistory, (double)atomic_load(&g_fpsValue));
    pushSparkHistory(frameIntSparkHistory, atomic_load(&g_frameIntervalUs) / 1000.0);

    // Dynamically resize window based on content
    CGFloat rowH = 28;
    CGFloat topPad = 16;  // Must match y = 16 in drawRect
    CGFloat botPad = 16;  // Symmetrical bottom padding
    CGFloat baseH = topPad + 60 + 10; // Header block (title + core + sep)
    int rows = 0;

    if (g_showSettings) {
      // Settings panel: 2 section headers (22px each) + 28 checkbox rows (26px each)
      // + separator (14px) + done button (42px) + padding
      CGFloat settingsH = baseH + 8 + 22 + (14 * 26) + 14 + 22 + (14 * 26) + 10 + 42 + 10;
      botPad = 0;
      CGFloat newH = settingsH;
      NSRect frame = g_overlayWindow.frame;
      if ((int)frame.size.height != (int)newH) {
        CGFloat dy = newH - frame.size.height;
        frame.origin.y -= dy;
        frame.size.height = newH;
        [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
            context.duration = 0.25;
            context.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
            [[g_overlayWindow animator] setFrame:frame display:YES];
        } completionHandler:^{
            NSView *bgView = g_overlayWindow.contentView;
            bgView.frame = NSMakeRect(0, 0, frame.size.width, newH);
            g_contentView.frame = NSMakeRect(0, 0, frame.size.width, newH);
            [g_contentView setNeedsDisplay:YES];
        }];
      } else {
        [g_contentView setNeedsDisplay:YES];
      }
      return;
    }

    // Count rows based on which section list is active
    if (g_overlay_expanded) {
      BOOL addedDetailSep = NO;
      for (int i = 0; i < g_expandedCount; i++) {
        OverlaySectionID sid = g_expandedSections[i];

        // Add separator space before detail sections
        if (!addedDetailSep && (sid == kSectionPower || sid == kSectionBandwidth || sid == kSectionGPUFreq)) {
          baseH += 10;
          addedDetailSep = YES;
        }

        rows += sectionRowCount(sid, localMetrics);
      }
    } else {
      for (int i = 0; i < g_collapsedCount; i++) {
        OverlaySectionID sid = g_collapsedSections[i];
        rows += sectionRowCount(sid, localMetrics);
      }
    }

    botPad = 28; // Space at the bottom for the toggle pill
    CGFloat newH = baseH + rows * rowH + botPad;

    NSRect frame = g_overlayWindow.frame;
    if ((int)frame.size.height != (int)newH) {
      // Keep top-left pinned
      CGFloat dy = newH - frame.size.height;
      frame.origin.y -= dy;
      frame.size.height = newH;
      
      [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
          context.duration = 0.25;
          context.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
          // Use animator to animate resize
          [[g_overlayWindow animator] setFrame:frame display:YES];
      } completionHandler:^{
          // Ensure layer bounds are correctly set. Subviews usually track window, but safe fallback:
          NSView *bgView = g_overlayWindow.contentView;
          bgView.frame = NSMakeRect(0, 0, frame.size.width, newH);
          g_contentView.frame = NSMakeRect(0, 0, frame.size.width, newH);
          [g_contentView setNeedsDisplay:YES];
      }];
    } else {
      [g_contentView setNeedsDisplay:YES];
    }
  });
}

void runOverlayLoop(void) { [NSApp run]; }

void cleanupOverlay(void) {
  stopFPSCounter();
  if (g_overlayWindow) {
    [g_overlayWindow close];
    g_overlayWindow = nil;
  }
  g_contentView = nil;
}
