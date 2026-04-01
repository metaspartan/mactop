// Copyright (c) 2024-2026 Carsen Klock under MIT License
// displayfps.m - Standalone display FPS counter via CGDisplayStream
// Extracted from overlay.m to make FPS/frame interval metrics available
// in headless mode and other non-overlay contexts.

#include <mach/mach_time.h>
#include <dlfcn.h>
#include <dispatch/dispatch.h>
#include <stdatomic.h>
#include <stdint.h>
#include <math.h>
#include <sys/sysctl.h>
#include <unistd.h>

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>

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
static CGDisplayStreamCreateWithDispatchQueue_fn fn_DFPSCreate = NULL;
static CGDisplayStreamStart_fn fn_DFPSStart = NULL;
static CGDisplayStreamStop_fn fn_DFPSStop = NULL;
static CGDisplayStreamUpdateGetDropCount_fn fn_DFPSGetDrops = NULL;

// CGDisplayStream property keys
static CFStringRef kDFPSMinFrameTime = NULL;
static CFStringRef kDFPSShowCursor = NULL;
static CFStringRef kDFPSQueueDepth = NULL;
static CFStringRef kDFPSSourceRect = NULL;

static bool dfps_loadSymbols(void) {
  void *cg = dlopen(
      "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics",
      RTLD_LAZY);
  if (!cg)
    return false;

  fn_DFPSCreate = (CGDisplayStreamCreateWithDispatchQueue_fn)dlsym(
      cg, "CGDisplayStreamCreateWithDispatchQueue");
  fn_DFPSStart = (CGDisplayStreamStart_fn)dlsym(cg, "CGDisplayStreamStart");
  fn_DFPSStop = (CGDisplayStreamStop_fn)dlsym(cg, "CGDisplayStreamStop");
  fn_DFPSGetDrops = (CGDisplayStreamUpdateGetDropCount_fn)dlsym(
      cg, "CGDisplayStreamUpdateGetDropCount");

  CFStringRef *pMin =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamMinimumFrameTime");
  CFStringRef *pCur =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamShowCursor");
  CFStringRef *pDepth =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamQueueDepth");
  CFStringRef *pSrc =
      (CFStringRef *)dlsym(cg, "kCGDisplayStreamSourceRect");

  if (pMin) kDFPSMinFrameTime = *pMin;
  if (pCur) kDFPSShowCursor = *pCur;
  if (pDepth) kDFPSQueueDepth = *pDepth;
  if (pSrc) kDFPSSourceRect = *pSrc;

  // kDFPSSourceRect might be legitimately NULL on very old macOS, so don't fail if missing
  return (fn_DFPSCreate && fn_DFPSStart && fn_DFPSStop && fn_DFPSGetDrops &&
          kDFPSMinFrameTime && kDFPSShowCursor && kDFPSQueueDepth);
}

// Frame statuses
enum {
  kDFPSStatusComplete = 0,
  kDFPSStatusIdle = 1,
  kDFPSStatusBlank = 2,
  kDFPSStatusStopped = 3,
};

// Atomic counters
static CGDisplayStreamRef_t g_dfpsStream = NULL;
static _Atomic uint32_t g_dfpsFrameCount = 0;
static _Atomic uint32_t g_dfpsDropCount = 0;
static _Atomic uint32_t g_dfpsFPS = 0;
static _Atomic uint32_t g_dfpsFrameIntervalUs = 0;
static dispatch_source_t g_dfpsTimer = NULL;
static uint64_t g_dfpsLastTimestamp = 0;
static _Atomic int g_dfpsRunning = 0;

static double dfps_machTimeToSeconds(uint64_t elapsed) {
  static mach_timebase_info_data_t sTimebase = {0};
  if (sTimebase.denom == 0) {
    mach_timebase_info(&sTimebase);
  }
  return (double)elapsed * sTimebase.numer / sTimebase.denom / 1e9;
}

// startDisplayFPSCounter starts the CGDisplayStream-based FPS counter.
// Returns 0 on success, -1 if CGDisplayStream is unavailable.
int startDisplayFPSCounter(void) {
  if (atomic_exchange(&g_dfpsRunning, 1) == 1) {
    return 0; // Already running
  }

  if (!dfps_loadSymbols()) {
    return -1;
  }

  CGDirectDisplayID mainDisplay = CGMainDisplayID();

  NSMutableDictionary *props = [NSMutableDictionary dictionary];
  props[(__bridge NSString *)kDFPSMinFrameTime] = @(0.0);
  props[(__bridge NSString *)kDFPSShowCursor] = @(NO);
  props[(__bridge NSString *)kDFPSQueueDepth] = @(1);

  dispatch_queue_t q =
      dispatch_queue_create("com.mactop.displayfps", DISPATCH_QUEUE_SERIAL);

  // Capture a 16x16 region minimum scaling threshold check (bypass <16px hw faults)
  g_dfpsStream = fn_DFPSCreate(
      mainDisplay, 16, 16, 'BGRA', (__bridge CFDictionaryRef)props, q,
      ^(int status, uint64_t displayTime __attribute__((unused)),
        IOSurfaceRef_t frameSurface __attribute__((unused)),
        CGDisplayStreamUpdateRef_t updateRef) {
        if (status == kDFPSStatusComplete) {
          atomic_fetch_add(&g_dfpsFrameCount, 1);
          if (updateRef && fn_DFPSGetDrops) {
            size_t dropped = fn_DFPSGetDrops(updateRef);
            if (dropped > 0) {
              atomic_fetch_add(&g_dfpsDropCount, (uint32_t)dropped);
            }
          }
        }
      });

  if (!g_dfpsStream) {
    return -1; // Stream creation failed (no display, permission denied, etc.)
  }
  fn_DFPSStart(g_dfpsStream);

  g_dfpsLastTimestamp = mach_absolute_time();

  // Timer fires every ~1s to snapshot FPS using actual elapsed time
  g_dfpsTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0,
                                       dispatch_get_global_queue(QOS_CLASS_UTILITY, 0));
  dispatch_source_set_timer(g_dfpsTimer,
                            dispatch_time(DISPATCH_TIME_NOW, NSEC_PER_SEC),
                            NSEC_PER_SEC, NSEC_PER_SEC / 10);
  dispatch_source_set_event_handler(g_dfpsTimer, ^{
    uint64_t now = mach_absolute_time();
    uint64_t elapsed = now - g_dfpsLastTimestamp;
    double seconds = dfps_machTimeToSeconds(elapsed);
    g_dfpsLastTimestamp = now;

    uint32_t completed = atomic_exchange(&g_dfpsFrameCount, 0);
    uint32_t dropped = atomic_exchange(&g_dfpsDropCount, 0);
    uint32_t totalFrames = completed + dropped;

    uint32_t fps = 0;
    uint32_t intervalUs = 0;
    if (seconds > 0.1 && totalFrames > 0) {
      fps = (uint32_t)(totalFrames / seconds + 0.5);
      intervalUs = (uint32_t)(seconds * 1e6 / totalFrames + 0.5);
    }
    atomic_store(&g_dfpsFPS, fps);
    atomic_store(&g_dfpsFrameIntervalUs, intervalUs);
  });
  dispatch_resume(g_dfpsTimer);

  atomic_store(&g_dfpsRunning, 1);
  return 0;
}

// stopDisplayFPSCounter tears down the CGDisplayStream and timer.
void stopDisplayFPSCounter(void) {
  if (!atomic_load(&g_dfpsRunning))
    return;

  if (g_dfpsStream && fn_DFPSStop) {
    fn_DFPSStop(g_dfpsStream);
    CFRelease(g_dfpsStream);
    g_dfpsStream = NULL;
  }
  if (g_dfpsTimer) {
    dispatch_source_cancel(g_dfpsTimer);
    g_dfpsTimer = NULL;
  }

  atomic_store(&g_dfpsRunning, 0);
}

// getDisplayFPS returns the current display FPS value.
uint32_t getDisplayFPS(void) { return atomic_load(&g_dfpsFPS); }

// getDisplayFrameIntervalUs returns the average frame interval in microseconds.
uint32_t getDisplayFrameIntervalUs(void) {
  return atomic_load(&g_dfpsFrameIntervalUs);
}

// ---------- Diagnostic dump ----------

// CGPreflightScreenCaptureAccess was added in macOS 10.15
typedef bool (*CGPreflightScreenCaptureAccess_fn)(void);

// dumpDisplayFPSDiagnostics prints comprehensive display and CGDisplayStream
// diagnostic info to stdout so remote users can paste the output for debugging.
void dumpDisplayFPSDiagnostics(void) {
  printf("=== mactop Display FPS Diagnostics ===\n\n");

  // --- macOS version ---
  NSProcessInfo *pi = [NSProcessInfo processInfo];
  NSOperatingSystemVersion ver = [pi operatingSystemVersion];
  printf("macOS Version:    %ld.%ld.%ld\n",
         (long)ver.majorVersion, (long)ver.minorVersion, (long)ver.patchVersion);

  // --- Hardware model ---
  size_t hwLen = 0;
  sysctlbyname("hw.model", NULL, &hwLen, NULL, 0);
  if (hwLen > 0) {
    char *model = malloc(hwLen);
    if (model && sysctlbyname("hw.model", model, &hwLen, NULL, 0) == 0) {
      printf("Hardware Model:   %s\n", model);
    }
    free(model);
  }

  // --- CPU brand ---
  size_t cpuLen = 0;
  sysctlbyname("machdep.cpu.brand_string", NULL, &cpuLen, NULL, 0);
  if (cpuLen > 0) {
    char *cpu = malloc(cpuLen);
    if (cpu && sysctlbyname("machdep.cpu.brand_string", cpu, &cpuLen, NULL, 0) == 0) {
      printf("CPU:              %s\n", cpu);
    }
    free(cpu);
  }
  printf("\n");

  // --- Screen recording permission ---
  void *cg = dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", RTLD_LAZY);
  if (cg) {
    CGPreflightScreenCaptureAccess_fn preflightFn =
        (CGPreflightScreenCaptureAccess_fn)dlsym(cg, "CGPreflightScreenCaptureAccess");
    if (preflightFn) {
      bool hasAccess = preflightFn();
      printf("Screen Recording: %s\n", hasAccess ? "GRANTED" : "NOT GRANTED (may block CGDisplayStream)");
    } else {
      printf("Screen Recording: (CGPreflightScreenCaptureAccess not available)\n");
    }
  }
  printf("\n");

  // --- Active displays ---
  CGDirectDisplayID displays[16];
  uint32_t displayCount = 0;
  CGGetActiveDisplayList(16, displays, &displayCount);
  printf("Active Displays:  %u\n\n", displayCount);

  for (uint32_t d = 0; d < displayCount; d++) {
    CGDirectDisplayID did = displays[d];
    size_t pw = CGDisplayPixelsWide(did);
    size_t ph = CGDisplayPixelsHigh(did);
    bool isMain = CGDisplayIsMain(did);
    bool isBuiltin = CGDisplayIsBuiltin(did);

    printf("  Display %u (ID: 0x%08x)%s%s\n", d, did,
           isMain ? " [MAIN]" : "",
           isBuiltin ? " [BUILTIN]" : " [EXTERNAL]");
    printf("    Resolution:   %zux%zu px\n", pw, ph);

    // Refresh rate from display mode
    CGDisplayModeRef mode = CGDisplayCopyDisplayMode(did);
    if (mode) {
      double refreshRate = CGDisplayModeGetRefreshRate(mode);
      size_t modeW = CGDisplayModeGetPixelWidth(mode);
      size_t modeH = CGDisplayModeGetPixelHeight(mode);
      printf("    Pixel Size:   %zux%zu (backing)\n", modeW, modeH);
      if (refreshRate > 0) {
        printf("    Refresh Rate: %.1f Hz\n", refreshRate);
      } else {
        printf("    Refresh Rate: Variable/ProMotion (reported as 0)\n");
      }
      CGDisplayModeRelease(mode);
    }
    printf("\n");
  }

  // --- Symbol loading ---
  printf("--- CGDisplayStream Symbol Loading ---\n");
  bool symsOK = dfps_loadSymbols();
  printf("  CGDisplayStreamCreateWithDispatchQueue: %s\n", fn_DFPSCreate ? "OK" : "MISSING");
  printf("  CGDisplayStreamStart:                   %s\n", fn_DFPSStart ? "OK" : "MISSING");
  printf("  CGDisplayStreamStop:                    %s\n", fn_DFPSStop ? "OK" : "MISSING");
  printf("  CGDisplayStreamUpdateGetDropCount:      %s\n", fn_DFPSGetDrops ? "OK" : "MISSING");
  printf("  kCGDisplayStreamMinimumFrameTime:       %s\n", kDFPSMinFrameTime ? "OK" : "MISSING");
  printf("  kCGDisplayStreamShowCursor:             %s\n", kDFPSShowCursor ? "OK" : "MISSING");
  printf("  kCGDisplayStreamQueueDepth:             %s\n", kDFPSQueueDepth ? "OK" : "MISSING");
  printf("  kCGDisplayStreamSourceRect:             %s\n", kDFPSSourceRect ? "OK" : "N/A (optional)");
  printf("  Overall:                                %s\n\n", symsOK ? "PASS" : "FAIL");

  if (!symsOK) {
    printf("Cannot proceed with stream tests — required symbols not found.\n");
    return;
  }

  // --- Stream creation test at multiple output sizes ---
  printf("--- Stream Creation Tests (main display 0x%08x) ---\n", (unsigned)displays[0]);
  CGDirectDisplayID mainDisplay = displays[0];
  dispatch_queue_t testQ = dispatch_queue_create("com.mactop.fpsdiag", DISPATCH_QUEUE_SERIAL);

  // Track whether we got any frame callbacks
  static _Atomic uint32_t g_diagFrameCount = 0;

  int testSizes[] = {1, 2, 4, 8, 16, 32};
  int nSizes = sizeof(testSizes) / sizeof(testSizes[0]);

  for (int t = 0; t < nSizes; t++) {
    int sz = testSizes[t];

    NSDictionary *props = @{
      (__bridge NSString *)kDFPSMinFrameTime : @(0.0),
      (__bridge NSString *)kDFPSShowCursor : @(NO),
      (__bridge NSString *)kDFPSQueueDepth : @(1),
    };

    atomic_store(&g_diagFrameCount, 0);

    CGDisplayStreamRef_t stream = fn_DFPSCreate(
        mainDisplay, sz, sz, 'BGRA', (__bridge CFDictionaryRef)props, testQ,
        ^(int status, uint64_t dt __attribute__((unused)),
          IOSurfaceRef_t sf __attribute__((unused)),
          CGDisplayStreamUpdateRef_t ur __attribute__((unused))) {
          if (status == kDFPSStatusComplete) {
            atomic_fetch_add(&g_diagFrameCount, 1);
          }
        });

    if (!stream) {
      printf("  %3dx%-3d  → ❌ CREATION FAILED (fn_Create returned NULL)\n", sz, sz);
      continue;
    }

    int startRet = fn_DFPSStart(stream);
    if (startRet != 0) {
      printf("  %3dx%-3d  → ❌ START FAILED (code %d)\n", sz, sz, startRet);
      CFRelease(stream);
      continue;
    }

    // Wait 2 seconds for frame callbacks to arrive
    usleep(2000000);

    uint32_t frames = atomic_load(&g_diagFrameCount);
    if (frames > 0) {
      printf("  %3dx%-3d  → ✅ %u frames in 2s (~%u FPS)\n", sz, sz, frames, frames / 2);
    } else {
      printf("  %3dx%-3d  → ⚠️  0 frames in 2s (hardware scaler may reject this size)\n", sz, sz);
    }

    fn_DFPSStop(stream);
    CFRelease(stream);
  }

  printf("\n=== End Diagnostics ===\n");
}
