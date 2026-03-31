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
