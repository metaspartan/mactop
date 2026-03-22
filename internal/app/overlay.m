// Copyright (c) 2024-2026 Carsen Klock under MIT License
// overlay.m - Native macOS floating overlay HUD window

#import <Cocoa/Cocoa.h>
#include <dispatch/dispatch.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define OVERLAY_SPARKLINE_HISTORY 60

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
} overlay_config_t;

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
};

static overlay_metrics_t g_overlay_metrics;
static double cpuSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};
static double gpuSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};
static double fpsSparkHistory[OVERLAY_SPARKLINE_HISTORY] = {0};

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

  if (pMinFrameTime)
    kMinFrameTime = *pMinFrameTime;
  if (pShowCursor)
    kShowCursor = *pShowCursor;
  if (pQueueDepth)
    kQueueDepth = *pQueueDepth;

  // Don't dlclose — keep symbols alive
  return (fn_CGDisplayStreamCreate && fn_CGDisplayStreamStart &&
          fn_CGDisplayStreamStop && fn_CGDisplayStreamGetDrops && kMinFrameTime &&
          kShowCursor && kQueueDepth);
}

static CGDisplayStreamRef_t g_fpsStream = NULL;
static _Atomic uint32_t g_fpsFrameCount = 0;   // Completed frames this interval
static _Atomic uint32_t g_fpsDropCount = 0;     // Dropped frames this interval
static _Atomic uint32_t g_fpsValue = 0;         // Last computed FPS
static dispatch_source_t g_fpsTimer = NULL;
static uint64_t g_fpsLastTimestamp = 0;

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
    return;
  }

  CGDirectDisplayID mainDisplay = CGMainDisplayID();

  // minimumFrameTime = 0 means "deliver as fast as possible"
  NSDictionary *streamProps = @{
    (__bridge NSString *)kMinFrameTime : @(0.0),
    (__bridge NSString *)kShowCursor : @(NO),
    (__bridge NSString *)kQueueDepth : @(1),
  };

  dispatch_queue_t fpsQueue =
      dispatch_queue_create("com.mactop.fps", DISPATCH_QUEUE_SERIAL);

  // Capture a tiny 1x1 region to minimize GPU/memory cost
  g_fpsStream = fn_CGDisplayStreamCreate(
      mainDisplay, 1, 1, 'BGRA', (__bridge CFDictionaryRef)streamProps,
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

  if (g_fpsStream) {
    fn_CGDisplayStreamStart(g_fpsStream);
  }

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

    // Calculate FPS from actual elapsed time
    uint32_t fps = 0;
    if (seconds > 0.1) {
      fps = (uint32_t)(totalFrames / seconds + 0.5);
    }
    atomic_store(&g_fpsValue, fps);
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
  return [NSColor colorWithRed:0.15 green:1.0 blue:0.30 alpha:1.0];
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
  return [NSColor colorWithRed:0.15 green:1.0 blue:0.30 alpha:1.0];
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
  self.dragStart = [NSEvent mouseLocation];
  self.windowStart = self.window.frame.origin;
  self.dragging = YES;
}

- (void)mouseDragged:(NSEvent *)event {
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
                              CGFloat w, CGFloat h, NSColor *color) {
  if (count < 2)
    return;

  double maxVal = 100.0;
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
    modelName = @"Apple Silicon";
  NSSize dotSize = [dot sizeWithAttributes:dotAttrs];
  [modelName
      drawAtPoint:NSMakePoint(padX + titleSize.width + 5 + dotSize.width + 5,
                               y + 0.5)
      withAttributes:subHeaderAttrs];
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
    [coreSummary appendFormat:@" • %d GPU Cores", m.gpu_core_count];
  }
  [coreSummary drawAtPoint:NSMakePoint(padX, y)
               withAttributes:smallAttrs];
  y += 22;

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
                            sparkH, color);
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

  // FPS (first metric — always rendered, full-width sparkline)
  uint32_t fps = atomic_load(&g_fpsValue);
  {
    NSString *fpsLabel = @"FPS";
    [fpsLabel drawAtPoint:NSMakePoint(padX, y + 4) withAttributes:labelAttrs];

    // FPS value right-aligned
    NSString *fpsVal = [NSString stringWithFormat:@"%u", fps];
    NSDictionary *fpsAttrs = @{
      NSFontAttributeName : valueFont,
      NSForegroundColorAttributeName : overlayAccentCyan()
    };
    NSSize fpsSize = [fpsVal sizeWithAttributes:fpsAttrs];
    [fpsVal drawAtPoint:NSMakePoint(padX + contentW - fpsSize.width, y + 3)
        withAttributes:fpsAttrs];

    // Full-width sparkline between label and value
    CGFloat fpsLabelW = 50; // space for "FPS" label
    CGFloat fpsValW = fpsSize.width + 8; // space for value + gap
    CGFloat fpsSparkW = contentW - fpsLabelW - fpsValW;
    drawMiniSparkline(fpsSparkHistory, OVERLAY_SPARKLINE_HISTORY,
                      padX + fpsLabelW, y + 2, fpsSparkW,
                      sparkH, overlayAccentCyan());
    y += rowH;
  }

  // CPU
  if (cfg.show_cpu) {
    drawMetricBar(@"CPU", m.cpu_percent, overlayAccentGreen(), cpuSparkHistory,
                  YES);
  }

  // GPU
  if (cfg.show_gpu) {
    drawMetricBar(@"GPU", m.gpu_percent, overlayAccentOrange(), gpuSparkHistory,
                  YES);
  }

  // ANE
  if (cfg.show_ane) {
    drawMetricBar(@"ANE", m.ane_percent, overlayAccentCyan(), NULL, NO);
  }

  // Memory
  if (cfg.show_memory) {
    double memGB = (double)m.mem_used_bytes / (1024.0 * 1024.0 * 1024.0);
    double totalGB = (double)m.mem_total_bytes / (1024.0 * 1024.0 * 1024.0);
    double memPct = totalGB > 0 ? (memGB / totalGB) * 100.0 : 0;
    NSString *memStr =
        [NSString stringWithFormat:@"%.1f/%.0fGB", memGB, totalGB];
    [(@"Memory") drawAtPoint:NSMakePoint(padX, y + 4)
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

    // Swap — only show when swap is actually being used
    if (m.swap_used_bytes > 0 && m.swap_total_bytes > 0) {
      double swapGB =
          (double)m.swap_used_bytes / (1024.0 * 1024.0 * 1024.0);
      double swapTotalGB =
          (double)m.swap_total_bytes / (1024.0 * 1024.0 * 1024.0);
      double swapPct = swapTotalGB > 0 ? (swapGB / swapTotalGB) * 100.0 : 0;
      NSString *swapStr =
          [NSString stringWithFormat:@"%.1f/%.0fGB", swapGB, swapTotalGB];
      [(@"Swap") drawAtPoint:NSMakePoint(padX, y + 4)
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
  }

  // Separator
  [[NSColor colorWithWhite:1.0 alpha:0.06] set];
  [NSBezierPath fillRect:NSMakeRect(padX, y, contentW, 1)];
  y += 5;

  // Power
  if (cfg.show_power) {
    NSString *powerStr =
        [NSString stringWithFormat:@"%.1fW", m.package_watts];
    drawMetricKV(@"Power", powerStr, overlayAccentYellow());

    // Individual power breakdown — always show all to prevent jumping
    drawMetricKV(@"  CPU", [NSString stringWithFormat:@"%.1fW", m.cpu_watts], overlayDimText());
    drawMetricKV(@"  GPU", [NSString stringWithFormat:@"%.1fW", m.gpu_watts], overlayDimText());
    drawMetricKV(@"  ANE", [NSString stringWithFormat:@"%.1fW", m.ane_watts], overlayDimText());
    drawMetricKV(@"  DRAM", [NSString stringWithFormat:@"%.1fW", m.dram_watts], overlayDimText());
  }

  // DRAM Bandwidth
  if (cfg.show_bandwidth) {
    NSString *bwStr =
        [NSString stringWithFormat:@"%.1f GB/s", m.dram_bw_combined_gbs];
    drawMetricKV(@"DRAM BW", bwStr, overlayAccentBlue());
  }

  // GPU Freq + TFLOPs
  if (cfg.show_gpu_freq) {
    NSString *freqStr;
    if (m.tflops_fp32 > 0) {
      freqStr = [NSString
          stringWithFormat:@"%dMHz • %.1f TFLOPS", m.gpu_freq_mhz,
                           m.tflops_fp32];
    } else {
      freqStr = [NSString stringWithFormat:@"%d MHz", m.gpu_freq_mhz];
    }
    drawMetricKV(@"GPU Freq", freqStr, overlayAccentOrange());
  }

  // Separator
  [[NSColor colorWithWhite:1.0 alpha:0.06] set];
  [NSBezierPath fillRect:NSMakeRect(padX, y, contentW, 1)];
  y += 5;

  // Temps
  if (cfg.show_temps) {
    NSString *tempStr;
    if (m.gpu_temp > 0) {
      tempStr = [NSString
          stringWithFormat:@"CPU %.0f°C  GPU %.0f°C", m.cpu_temp, m.gpu_temp];
    } else {
      tempStr = [NSString stringWithFormat:@"%.0f°C", m.cpu_temp];
    }
    NSColor *tempColor = overlayBrightText();
    if (m.cpu_temp >= 90 || m.gpu_temp >= 90)
      tempColor = overlayAccentRed();
    else if (m.cpu_temp >= 70 || m.gpu_temp >= 70)
      tempColor = overlayAccentYellow();
    drawMetricKV(@"Temps", tempStr, tempColor);
  }

  // Thermal state
  if (cfg.show_thermals) {
    NSString *thermalStr =
        [NSString stringWithUTF8String:m.thermal_state];
    if (thermalStr.length == 0)
      thermalStr = @"Unknown";
    NSColor *thermalColor = overlayAccentGreen();
    if ([thermalStr containsString:@"Critical"])
      thermalColor = overlayAccentRed();
    else if ([thermalStr containsString:@"Serious"])
      thermalColor = overlayAccentRed();
    else if ([thermalStr containsString:@"Fair"])
      thermalColor = overlayAccentYellow();
    drawMetricKV(@"Thermal", thermalStr, thermalColor);
  }

  // Fans
  if (cfg.show_fans && m.fan_count > 0) {
    NSMutableString *fanStr = [NSMutableString string];
    for (int i = 0; i < m.fan_count && i < 4; i++) {
      if (i > 0)
        [fanStr appendString:@"  "];
      [fanStr appendFormat:@"%dRPM", m.fan_rpm[i]];
    }
    drawMetricKV(@"Fans", fanStr, overlayDimText());
  }

  // Network
  if (cfg.show_network) {
    NSString *netStr = [NSString
        stringWithFormat:@"↓%@ ↑%@",
                         formatOverlayThroughput(m.net_in_bytes_per_sec),
                         formatOverlayThroughput(m.net_out_bytes_per_sec)];
    drawMetricKV(@"Network", netStr, overlayDimText());
  }

  // Opacity indicator (flashes briefly when scroll-wheel adjusts opacity)
  if (g_opacityFlashCountdown > 0) {
    g_opacityFlashCountdown--;
    CGFloat opacityPct = g_overlay_config.opacity * 100.0;
    NSString *opacityStr =
        [NSString stringWithFormat:@"Opacity: %.0f%%  (scroll to adjust)", opacityPct];
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

    // Dynamically resize window based on content
    CGFloat rowH = 28;
    CGFloat topPad = 16;  // Must match y = 16 in drawRect
    CGFloat botPad = 16;  // Symmetrical bottom padding
    CGFloat baseH = topPad + 60 + 10; // Header block (title + core + sep)
    int rows = 0;

    rows++; // FPS row (always present — shows 0 when unavailable)

    if (g_overlay_config.show_cpu) rows++;
    if (g_overlay_config.show_gpu) rows++;
    if (g_overlay_config.show_ane) rows++;
    if (g_overlay_config.show_memory) {
      rows++;
      if (localMetrics.swap_used_bytes > 0 && localMetrics.swap_total_bytes > 0)
        rows++;
    }
    baseH += 10; // separator

    if (g_overlay_config.show_power) {
      rows += 5; // Total + CPU + GPU + ANE + DRAM (always show all)
    }
    if (g_overlay_config.show_bandwidth) rows++;
    if (g_overlay_config.show_gpu_freq) rows++;
    baseH += 10; // separator

    if (g_overlay_config.show_temps) rows++;
    if (g_overlay_config.show_thermals) rows++;
    if (g_overlay_config.show_fans && localMetrics.fan_count > 0) rows++;
    if (g_overlay_config.show_network) rows++;

    CGFloat newH = baseH + rows * rowH + botPad;

    NSRect frame = g_overlayWindow.frame;
    if ((int)frame.size.height != (int)newH) {
      // Keep top-left pinned
      CGFloat dy = newH - frame.size.height;
      frame.origin.y -= dy;
      frame.size.height = newH;
      [g_overlayWindow setFrame:frame display:NO];

      // Resize subviews
      NSView *bgView = g_overlayWindow.contentView;
      bgView.frame = NSMakeRect(0, 0, frame.size.width, newH);
      g_contentView.frame = NSMakeRect(0, 0, frame.size.width, newH);
    }

    [g_contentView setNeedsDisplay:YES];
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
