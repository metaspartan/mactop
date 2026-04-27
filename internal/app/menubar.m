// Copyright (c) 2024-2026 Carsen Klock under MIT License
// menubar.m - Native macOS menu bar status item using AppKit

#import <Cocoa/Cocoa.h>
#include <dispatch/dispatch.h>
#import <objc/runtime.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define SPARKLINE_HISTORY_SIZE 60

// Metrics structure passed from Go
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
} menubar_metrics_t;

// Config passed from Go
typedef struct {
  int status_bar_width;
  int status_bar_height;
  int sparkline_width;
  int sparkline_height;
  int show_cpu;
  int show_gpu;
  int show_ane;
  int show_memory;
  int show_power;
  int show_percent;
  int font_size;
  int power_font_size;
  char cpu_color[8];
  char gpu_color[8];
  char ane_color[8];
  char mem_color[8];
  char label_color[8];
} menubar_config_t;

// Go callback for persisting settings
extern void GoSaveMenuBarConfig(int statusBarWidth, int statusBarHeight,
                                int sparklineWidth, int sparklineHeight,
                                int showCPU, int showGPU, int showANE,
                                int showMem, int showPower, int showPercent,
                                int fontSize, int powerFontSize,
                                const char *cpuHex, const char *gpuHex,
                                const char *aneHex, const char *memHex,
                                const char *labelHex);

// Global state
static menubar_config_t g_config = {
    .status_bar_width = 28,
    .status_bar_height = 18,
    .sparkline_width = 420,
    .sparkline_height = 80,
    .show_cpu = 1,
    .show_gpu = 1,
    .show_ane = 1,
    .show_memory = 0,
    .show_power = 1,
    .show_percent = 0,
    .font_size = 10,
    .power_font_size = 11,
    .cpu_color = "",
    .gpu_color = "",
    .ane_color = "",
    .mem_color = "",
    .label_color = "",
};

// Sparkline history buffers
static double cpuHistory[SPARKLINE_HISTORY_SIZE] = {0};
static double gpuHistory[SPARKLINE_HISTORY_SIZE] = {0};
static double memHistory[SPARKLINE_HISTORY_SIZE] = {0};
static double aneHistory[SPARKLINE_HISTORY_SIZE] = {0};

static void pushHistory(double *buf, double val) {
  memmove(buf, buf + 1, (SPARKLINE_HISTORY_SIZE - 1) * sizeof(double));
  buf[SPARKLINE_HISTORY_SIZE - 1] = val;
}

// Forward declarations
static NSFont *metricFont(void);
static NSFont *headerFont(void);
static NSColor *labelDimColor(void);
static NSColor *valueColor(void);
static NSColor *headerColor(void);
static NSImage *drawStatusBarImage(double cpu, double gpu, double ane,
                                   double memPct, double watts);
static NSImage *drawSparklineChart(double *history, int count, NSColor *color,
                                   NSString *label, double currentVal,
                                   NSString *valOverride);
static NSString *formatThroughput(double bps);
static void buildMenu(void);
static void persistConfig(void);
static void refreshAllMenuColors(void);

// ---- Color helpers ----

static NSColor *colorFromHex(const char *hex) {
  if (hex == NULL || hex[0] == '\0')
    return nil;
  NSString *str = [NSString stringWithUTF8String:hex];
  if ([str hasPrefix:@"#"])
    str = [str substringFromIndex:1];
  if (str.length != 6)
    return nil;
  unsigned int rgb = 0;
  [[NSScanner scannerWithString:str] scanHexInt:&rgb];
  return [NSColor colorWithRed:((rgb >> 16) & 0xFF) / 255.0
                         green:((rgb >> 8) & 0xFF) / 255.0
                          blue:(rgb & 0xFF) / 255.0
                         alpha:1.0];
}

static NSString *hexFromColor(NSColor *color) {
  NSColor *c = [color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
  if (!c)
    return @"";
  int r = (int)(c.redComponent * 255 + 0.5);
  int g = (int)(c.greenComponent * 255 + 0.5);
  int b = (int)(c.blueComponent * 255 + 0.5);
  return [NSString stringWithFormat:@"#%02X%02X%02X", r, g, b];
}
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

static NSColor *cpuColor(void) {
  NSColor *c = colorFromHex(g_config.cpu_color);
  return c ?: [NSColor systemGreenColor];
}
static NSColor *gpuColor(void) {
  NSColor *c = colorFromHex(g_config.gpu_color);
  return c ?: [NSColor systemOrangeColor];
}
static NSColor *aneColor(void) {
  NSColor *c = colorFromHex(g_config.ane_color);
  return c ?: [NSColor systemCyanColor];
}
static NSColor *memColor(void) {
  NSColor *c = colorFromHex(g_config.mem_color);
  return c ?: [NSColor systemPurpleColor];
}
static NSColor *menuBarLabelColor(void) {
  NSColor *c = colorFromHex(g_config.label_color);
  return c ?: [NSColor labelColor];
}

static NSColor *labelDimColor(void) {
  // Dimmed variant of the configured label color (or system secondaryLabelColor
  // when no override is set, to preserve the default appearance).
  if (g_config.label_color[0] == '\0')
    return [NSColor secondaryLabelColor];
  return [menuBarLabelColor() colorWithAlphaComponent:0.7];
}
static NSColor *valueColor(void) { return menuBarLabelColor(); }
static NSColor *headerColor(void) { return menuBarLabelColor(); }

// ---- Settings Window Controller & Delegate Forward Declarations ----

@class SettingsWindowController;
@class MactopMenuBarDelegate;

// ---- Views ----

@interface MactopLabelView : NSView
@property(strong, nonatomic) NSTextField *label;
- (void)refreshColors;
@end
@implementation MactopLabelView
- (instancetype)initWithText:(NSString *)text
                        font:(NSFont *)font
                       color:(NSColor *)color {
  CGFloat chartW = (CGFloat)g_config.sparkline_width;
  CGFloat width = chartW + 16;
  CGFloat height = 24;
  self = [super initWithFrame:NSMakeRect(0, 0, width, height)];
  if (self) {
    _label = [NSTextField labelWithString:text];
    _label.font = font;
    _label.textColor = color;
    _label.frame = NSMakeRect(8, 0, width - 16, height);
    _label.drawsBackground = NO;
    _label.bordered = NO;
    _label.editable = NO;
    _label.selectable = NO;
    [self addSubview:_label];
    self.autoresizingMask = NSViewNotSizable;
  }
  return self;
}
- (void)refreshColors {
  _label.textColor = headerColor();
}
@end

@interface MactopMetricView : NSView
@property(strong, nonatomic) NSTextField *labelField;
@property(strong, nonatomic) NSTextField *valueField;
- (void)refreshColors;
@end
@implementation MactopMetricView
- (instancetype)initWithLabel:(NSString *)lbl value:(NSString *)val {
  CGFloat chartW = (CGFloat)g_config.sparkline_width;
  CGFloat width = chartW + 16;
  CGFloat height = 24;
  self = [super initWithFrame:NSMakeRect(0, 0, width, height)];
  if (self) {
    CGFloat halfW = (width - 16) / 2.0;
    _labelField =
        [[NSTextField alloc] initWithFrame:NSMakeRect(8, 0, halfW, height)];
    _labelField.drawsBackground = NO;
    _labelField.bordered = NO;
    _labelField.editable = NO;
    _labelField.selectable = NO;
    _labelField.font = metricFont();
    _labelField.textColor = labelDimColor();
    _labelField.alignment = NSTextAlignmentLeft;

    _valueField = [[NSTextField alloc]
        initWithFrame:NSMakeRect(8 + halfW, 0, halfW, height)];
    _valueField.drawsBackground = NO;
    _valueField.bordered = NO;
    _valueField.editable = NO;
    _valueField.selectable = NO;
    _valueField.font = metricFont();
    _valueField.textColor = valueColor();
    _valueField.alignment = NSTextAlignmentRight;

    [self addSubview:_labelField];
    [self addSubview:_valueField];
    self.autoresizingMask = NSViewNotSizable;

    [self setTwoToneLabel:lbl value:val];
  }
  return self;
}
- (void)setTwoToneLabel:(NSString *)lbl value:(NSString *)val {
  _labelField.stringValue = lbl;
  _valueField.stringValue = val;
}
- (void)refreshColors {
  _labelField.textColor = labelDimColor();
  _valueField.textColor = valueColor();
}
@end

@interface MactopBrandingView : NSView
@property(strong, nonatomic) NSTextField *field;
- (void)refreshColors;
@end
@implementation MactopBrandingView
- (instancetype)initWithFrame:(NSRect)frame {
  self = [super initWithFrame:frame];
  if (self) {
    _field = [[NSTextField alloc] initWithFrame:frame];
    _field.drawsBackground = NO;
    _field.bordered = NO;
    _field.editable = NO;
    _field.selectable = NO;
    _field.alignment = NSTextAlignmentCenter;
    [self addSubview:_field];
    self.autoresizingMask = NSViewNotSizable;
    [self refreshColors];
  }
  return self;
}
- (void)refreshColors {
  NSMutableParagraphStyle *style = [[NSMutableParagraphStyle alloc] init];
  style.alignment = NSTextAlignmentCenter;
  NSAttributedString *as = [[NSAttributedString alloc]
      initWithString:@"mactop"
          attributes:@{
            NSFontAttributeName :
                [NSFont systemFontOfSize:14 weight:NSFontWeightHeavy],
            NSForegroundColorAttributeName : menuBarLabelColor(),
            NSParagraphStyleAttributeName : style
          }];
  _field.attributedStringValue = as;
}
@end

@interface MactopImageView : NSView
@property(strong, nonatomic) NSImageView *imageView;
@end
@implementation MactopImageView
- (instancetype)initWithImage:(NSImage *)img {
  CGFloat insetX = 8;
  CGFloat padY = 6; // vertical spacing between charts
  CGFloat chartW = (CGFloat)g_config.sparkline_width;
  CGFloat h = (CGFloat)g_config.sparkline_height;
  CGFloat totalW = chartW + insetX * 2;
  CGFloat totalH = h + padY * 2;
  self = [super initWithFrame:NSMakeRect(0, 0, totalW, totalH)];
  if (self) {
    _imageView =
        [[NSImageView alloc] initWithFrame:NSMakeRect(insetX, padY, chartW, h)];
    _imageView.imageScaling = NSImageScaleNone;
    _imageView.image = img;
    _imageView.autoresizingMask = NSViewNotSizable;
    self.autoresizingMask = NSViewNotSizable;
    [self addSubview:_imageView];
  }
  return self;
}
@end

// ---- Delegate Interface ----

@interface MactopMenuBarDelegate : NSObject <NSApplicationDelegate>
@property(strong, nonatomic) NSStatusItem *statusItem;
@property(strong, nonatomic) NSMenu *statusMenu;
@property(strong, nonatomic) NSMenuItem *modelItem;
@property(strong, nonatomic) NSMenuItem *cpuUsageItem;
@property(strong, nonatomic) NSMenuItem *cpuEClusterItem;
@property(strong, nonatomic) NSMenuItem *cpuPClusterItem;
@property(strong, nonatomic) NSMenuItem *cpuSClusterItem;
@property(strong, nonatomic) NSMenuItem *cpuWattsItem;
@property(strong, nonatomic) NSMenuItem *cpuTempItem;
@property(strong, nonatomic) NSMenuItem *gpuUsageItem;
@property(strong, nonatomic) NSMenuItem *gpuWattsItem;
@property(strong, nonatomic) NSMenuItem *gpuTempItem;
@property(strong, nonatomic) NSMenuItem *gpuTflopsItem;
@property(strong, nonatomic) NSMenuItem *memUsageItem;
@property(strong, nonatomic) NSMenuItem *memSwapItem;
@property(strong, nonatomic) NSMenuItem *dramBwItem;
@property(strong, nonatomic) NSMenuItem *netItem;
@property(strong, nonatomic) NSMenuItem *rdmaItem;
@property(strong, nonatomic) NSMenuItem *diskItem;
@property(strong, nonatomic) NSMenuItem *powerTotalItem;
@property(strong, nonatomic) NSMenuItem *powerPackageItem;
@property(strong, nonatomic) NSMenuItem *powerCpuItem;
@property(strong, nonatomic) NSMenuItem *powerGpuItem;
@property(strong, nonatomic) NSMenuItem *powerAneItem;
@property(strong, nonatomic) NSMenuItem *powerDramItem;
@property(strong, nonatomic) NSMenuItem *thermalItem;
@property(strong, nonatomic) NSMenuItem *cpuSparkItem;
@property(strong, nonatomic) NSMenuItem *gpuSparkItem;
@property(strong, nonatomic) NSMenuItem *aneSparkItem;
@property(strong, nonatomic) NSMenuItem *memSparkItem;
@property(strong, nonatomic) NSMenuItem *fanHeaderItem;
@property(strong, nonatomic) NSMenuItem *fan0Item;
@property(strong, nonatomic) NSMenuItem *fan1Item;
@property(strong, nonatomic) NSMenuItem *fan2Item;
@property(strong, nonatomic) NSMenuItem *fan3Item;
@property(strong, nonatomic) NSMenuItem *fanSepItem;
- (void)performMetricUpdate:(NSValue *)val;
- (void)openSettings:(id)sender;
- (void)statusBarClicked:(id)sender;
@end

// ---- Settings Window Controller ----

@interface SettingsWindowController : NSWindowController <NSWindowDelegate>
@property(strong) NSButton *cpuCheck;
@property(strong) NSButton *gpuCheck;
@property(strong) NSButton *aneCheck;
@property(strong) NSButton *memCheck;
@property(strong) NSButton *powerCheck;
@property(strong) NSButton *percentCheck;
@property(strong) NSSlider *widthSlider;
@property(strong) NSTextField *widthLabel;
@property(strong) NSSlider *heightSlider;
@property(strong) NSTextField *heightLabel;
@property(strong) NSSlider *fontSizeSlider;
@property(strong) NSTextField *fontSizeLabel;
@property(strong) NSSlider *powerFontSlider;
@property(strong) NSTextField *powerFontLabel;
@property(strong) NSColorWell *cpuColorWell;
@property(strong) NSColorWell *gpuColorWell;
@property(strong) NSColorWell *aneColorWell;
@property(strong) NSColorWell *memColorWell;
@property(strong) NSColorWell *labelColorWell;
@end

@implementation SettingsWindowController

- (instancetype)init {
  CGFloat w = 320;
  CGFloat h = 560;
  NSRect frame = NSMakeRect(0, 0, w, h);
  NSWindow *window = [[NSWindow alloc]
      initWithContentRect:frame
                styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                          NSWindowStyleMaskMiniaturizable
                  backing:NSBackingStoreBuffered
                    defer:NO];
  window.title = localize(@"Menu_SettingsTitle");
  [window center];

  // "Liquid Glass" effect
  NSVisualEffectView *effectView =
      [[NSVisualEffectView alloc] initWithFrame:frame];
  effectView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
  effectView.material = NSVisualEffectMaterialHUDWindow;
  effectView.state = NSVisualEffectStateActive;
  window.contentView = effectView;
  window.opaque = NO;
  window.backgroundColor = [NSColor clearColor];

  self = [super initWithWindow:window];
  if (self) {
    [self buildUI:effectView];
  }
  return self;
}

- (void)buildUI:(NSView *)view {
  // Anchor content to top of contentView with consistent padding so the layout
  // adapts when the window height changes.
  CGFloat x = 20, lw = 200, lh = 20;
  CGFloat y = view.frame.size.height - 30 - lh;

  // Toggles
  _cpuCheck = [NSButton checkboxWithTitle:localize(@"Menu_ShowCPUBar")
                                   target:self
                                   action:@selector(toggleChanged:)];
  _cpuCheck.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:_cpuCheck];
  y -= 24;

  _gpuCheck = [NSButton checkboxWithTitle:localize(@"Menu_ShowGPUBar")
                                   target:self
                                   action:@selector(toggleChanged:)];
  _gpuCheck.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:_gpuCheck];
  y -= 24;

  _aneCheck = [NSButton checkboxWithTitle:localize(@"Menu_ShowANEBar")
                                   target:self
                                   action:@selector(toggleChanged:)];
  _aneCheck.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:_aneCheck];
  y -= 24;

  _memCheck = [NSButton checkboxWithTitle:localize(@"Menu_ShowMemoryBar")
                                   target:self
                                   action:@selector(toggleChanged:)];
  _memCheck.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:_memCheck];
  y -= 24;

  _powerCheck = [NSButton checkboxWithTitle:localize(@"Menu_ShowWattageInMenuBar")
                                     target:self
                                     action:@selector(toggleChanged:)];
  _powerCheck.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:_powerCheck];
  y -= 24;

  _percentCheck = [NSButton checkboxWithTitle:localize(@"Menu_ShowPercentages")
                                       target:self
                                       action:@selector(toggleChanged:)];
  _percentCheck.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:_percentCheck];
  y -= 32;

  // --- Adjustments ---

  // Bar Width
  NSTextField *wl = [NSTextField labelWithString:localize(@"Menu_StatusBarWidth")];
  wl.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:wl];
  y -= 22;

  _widthSlider = [[NSSlider alloc] initWithFrame:NSMakeRect(x, y, 180, lh)];
  _widthSlider.minValue = 16;
  _widthSlider.maxValue = 120;
  _widthSlider.target = self;
  _widthSlider.action = @selector(sliderChanged:);
  [view addSubview:_widthSlider];

  _widthLabel = [NSTextField labelWithString:@"28"];
  _widthLabel.frame = NSMakeRect(x + 190, y, 50, lh);
  [view addSubview:_widthLabel];
  y -= 32;

  // Bar Height
  NSTextField *hl = [NSTextField labelWithString:localize(@"Menu_StatusBarHeight")];
  hl.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:hl];
  y -= 22;

  _heightSlider = [[NSSlider alloc] initWithFrame:NSMakeRect(x, y, 180, lh)];
  _heightSlider.minValue = 12;
  _heightSlider.maxValue = 36;
  _heightSlider.target = self;
  _heightSlider.action = @selector(sliderChanged:);
  [view addSubview:_heightSlider];

  _heightLabel = [NSTextField labelWithString:@"18"];
  _heightLabel.frame = NSMakeRect(x + 190, y, 50, lh);
  [view addSubview:_heightLabel];
  y -= 32;

  // Font Size (Bars)
  NSTextField *fsl = [NSTextField labelWithString:localize(@"Menu_BarFontSize")];
  fsl.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:fsl];
  y -= 22;

  _fontSizeSlider = [[NSSlider alloc] initWithFrame:NSMakeRect(x, y, 180, lh)];
  _fontSizeSlider.minValue = 8;
  _fontSizeSlider.maxValue = 16;
  _fontSizeSlider.target = self;
  _fontSizeSlider.action = @selector(sliderChanged:);
  [view addSubview:_fontSizeSlider];

  _fontSizeLabel = [NSTextField labelWithString:@"10"];
  _fontSizeLabel.frame = NSMakeRect(x + 190, y, 50, lh);
  [view addSubview:_fontSizeLabel];
  y -= 32;

  // Font Size (Wattage)
  NSTextField *pfsl = [NSTextField labelWithString:localize(@"Menu_WattageFontSize")];
  pfsl.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:pfsl];
  y -= 22;

  _powerFontSlider = [[NSSlider alloc] initWithFrame:NSMakeRect(x, y, 180, lh)];
  _powerFontSlider.minValue = 8;
  _powerFontSlider.maxValue = 16;
  _powerFontSlider.target = self;
  _powerFontSlider.action = @selector(sliderChanged:);
  [view addSubview:_powerFontSlider];

  _powerFontLabel = [NSTextField labelWithString:@"11"];
  _powerFontLabel.frame = NSMakeRect(x + 190, y, 50, lh);
  [view addSubview:_powerFontLabel];
  y -= 32;

  // --- Colors ---

  NSTextField *cl = [NSTextField labelWithString:localize(@"Menu_Colors")];
  cl.frame = NSMakeRect(x, y, lw, lh);
  [view addSubview:cl];
  y -= 30;

  // Explicit Layout (No Blocks) to fix overlap
  // Row 1: CPU, GPU
  NSTextField *l1 =
      [NSTextField labelWithString:[NSString stringWithFormat:@"%@:", localize(@"Menu_CPU")]];
  l1.frame = NSMakeRect(x, y + 2, 40, 16);
  [view addSubview:l1];

  _cpuColorWell =
      [[NSColorWell alloc] initWithFrame:NSMakeRect(x + 45, y, 50, 24)];
  _cpuColorWell.target = self;
  _cpuColorWell.action = @selector(colorChanged:);
  [view addSubview:_cpuColorWell];

  NSTextField *l2 =
      [NSTextField labelWithString:[NSString stringWithFormat:@"%@:", localize(@"Menu_GPU")]];
  l2.frame = NSMakeRect(x + 120, y + 2, 40, 16);
  [view addSubview:l2];

  _gpuColorWell =
      [[NSColorWell alloc] initWithFrame:NSMakeRect(x + 165, y, 50, 24)];
  _gpuColorWell.target = self;
  _gpuColorWell.action = @selector(colorChanged:);
  [view addSubview:_gpuColorWell];
  y -= 34;

  // Row 2: ANE, Mem
  NSTextField *l3 =
      [NSTextField labelWithString:[NSString stringWithFormat:@"%@:", localize(@"Overlay_ANE")]];
  l3.frame = NSMakeRect(x, y + 2, 40, 16);
  [view addSubview:l3];

  _aneColorWell =
      [[NSColorWell alloc] initWithFrame:NSMakeRect(x + 45, y, 50, 24)];
  _aneColorWell.target = self;
  _aneColorWell.action = @selector(colorChanged:);
  [view addSubview:_aneColorWell];

  NSTextField *l4 =
      [NSTextField labelWithString:[NSString stringWithFormat:@"%@:", localize(@"Menu_RAM")]];
  l4.frame = NSMakeRect(x + 120, y + 2, 40, 16);
  [view addSubview:l4];

  _memColorWell =
      [[NSColorWell alloc] initWithFrame:NSMakeRect(x + 165, y, 50, 24)];
  _memColorWell.target = self;
  _memColorWell.action = @selector(colorChanged:);
  [view addSubview:_memColorWell];
  y -= 34;

  // Row 3: Label (text color for letters, percents, wattage)
  NSTextField *l5 =
      [NSTextField labelWithString:[NSString stringWithFormat:@"%@:", localize(@"Menu_LabelText")]];
  l5.frame = NSMakeRect(x, y + 2, 80, 16);
  [view addSubview:l5];

  _labelColorWell =
      [[NSColorWell alloc] initWithFrame:NSMakeRect(x + 85, y, 50, 24)];
  _labelColorWell.target = self;
  _labelColorWell.action = @selector(colorChanged:);
  [view addSubview:_labelColorWell];
}

- (void)showWindow:(id)sender {
  [self syncUI];
  [super showWindow:sender];
  [NSApp activateIgnoringOtherApps:YES];
}

- (void)syncUI {
  _cpuCheck.state =
      g_config.show_cpu ? NSControlStateValueOn : NSControlStateValueOff;
  _gpuCheck.state =
      g_config.show_gpu ? NSControlStateValueOn : NSControlStateValueOff;
  _aneCheck.state =
      g_config.show_ane ? NSControlStateValueOn : NSControlStateValueOff;
  _memCheck.state =
      g_config.show_memory ? NSControlStateValueOn : NSControlStateValueOff;
  _powerCheck.state =
      g_config.show_power ? NSControlStateValueOn : NSControlStateValueOff;
  _percentCheck.state =
      g_config.show_percent ? NSControlStateValueOn : NSControlStateValueOff;

  _widthSlider.integerValue = g_config.status_bar_width;
  _widthLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.status_bar_width];

  _heightSlider.integerValue = g_config.status_bar_height;
  _heightLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.status_bar_height];

  _fontSizeSlider.integerValue = g_config.font_size;
  _fontSizeLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.font_size];

  _powerFontSlider.integerValue = g_config.power_font_size;
  _powerFontLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.power_font_size];

  _cpuColorWell.color = cpuColor();
  _gpuColorWell.color = gpuColor();
  _aneColorWell.color = aneColor();
  _memColorWell.color = memColor();
  _labelColorWell.color = menuBarLabelColor();
}

- (void)toggleChanged:(id)sender {
  g_config.show_cpu = _cpuCheck.state == NSControlStateValueOn;
  g_config.show_gpu = _gpuCheck.state == NSControlStateValueOn;
  g_config.show_ane = _aneCheck.state == NSControlStateValueOn;
  g_config.show_memory = _memCheck.state == NSControlStateValueOn;
  g_config.show_power = _powerCheck.state == NSControlStateValueOn;
  g_config.show_percent = _percentCheck.state == NSControlStateValueOn;
  persistConfig();
}

- (void)sliderChanged:(id)sender {
  g_config.status_bar_width = _widthSlider.intValue;
  _widthLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.status_bar_width];

  g_config.status_bar_height = _heightSlider.intValue;
  _heightLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.status_bar_height];

  g_config.font_size = _fontSizeSlider.intValue;
  _fontSizeLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.font_size];

  g_config.power_font_size = _powerFontSlider.intValue;
  _powerFontLabel.stringValue =
      [NSString stringWithFormat:@"%d", g_config.power_font_size];

  // Force button font update immediately
  MactopMenuBarDelegate *delegate = (MactopMenuBarDelegate *)[NSApp delegate];
  if (delegate && delegate.statusItem) {
    delegate.statusItem.button.font = [NSFont
        monospacedDigitSystemFontOfSize:(CGFloat)g_config.power_font_size
                                 weight:NSFontWeightMedium];
  }

  persistConfig();
}

- (void)colorChanged:(id)sender {
  void (^updateColor)(char *, NSColor *) = ^(char *dest, NSColor *c) {
    NSString *hex = hexFromColor(c);
    strlcpy(dest, [hex UTF8String], 8);
  };

  if (sender == _cpuColorWell)
    updateColor(g_config.cpu_color, _cpuColorWell.color);
  if (sender == _gpuColorWell)
    updateColor(g_config.gpu_color, _gpuColorWell.color);
  if (sender == _aneColorWell)
    updateColor(g_config.ane_color, _aneColorWell.color);
  if (sender == _memColorWell)
    updateColor(g_config.mem_color, _memColorWell.color);
  if (sender == _labelColorWell)
    updateColor(g_config.label_color, _labelColorWell.color);
  refreshAllMenuColors();
  persistConfig();
}

@end

static SettingsWindowController *g_settingsWindow = nil;

static MactopMenuBarDelegate *g_delegate = nil;

// ---- Typography ----
static NSFont *metricFont(void) {
  return [NSFont monospacedDigitSystemFontOfSize:15 weight:NSFontWeightMedium];
}
static NSFont *headerFont(void) {
  return [NSFont systemFontOfSize:15 weight:NSFontWeightHeavy];
}

// Helpers
static NSMenuItem *makeHeaderItem(NSString *title) {
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@""
                                                action:nil
                                         keyEquivalent:@""];
  MactopLabelView *view = [[MactopLabelView alloc] initWithText:title
                                                           font:headerFont()
                                                          color:headerColor()];
  item.view = view;
  return item;
}
static NSMenuItem *makeMetricItem(NSString *label, NSString *value) {
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@""
                                                action:nil
                                         keyEquivalent:@""];
  MactopMetricView *view = [[MactopMetricView alloc] initWithLabel:label
                                                             value:value];
  item.view = view;
  return item;
}
static NSMenuItem *makeSparkItem(NSImage *img) {
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@""
                                                action:nil
                                         keyEquivalent:@""];
  MactopImageView *view = [[MactopImageView alloc] initWithImage:img];
  item.view = view;
  return item;
}
static NSMenuItem *makeBrandingItem(void) {
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@""
                                                action:nil
                                         keyEquivalent:@""];
  CGFloat chartW = (CGFloat)g_config.sparkline_width;
  CGFloat width = chartW + 16;
  CGFloat height = 24;
  MactopBrandingView *container =
      [[MactopBrandingView alloc] initWithFrame:NSMakeRect(0, 0, width, height)];
  item.view = container;
  return item;
}

// ---- Drawing ----

static void drawHBar(NSString *label, double pct, NSColor *color, CGFloat x,
                     CGFloat barY, CGFloat barW, CGFloat barH,
                     int showPercent) {
  CGFloat fill = (pct / 100.0) * barW;
  if (fill < 1.0 && pct > 0)
    fill = 1.0;

  // Use configured font size, Bold
  CGFloat fontSize =
      (CGFloat)(g_config.font_size > 0 ? g_config.font_size : 10);
  NSFont *lf = [NSFont monospacedDigitSystemFontOfSize:fontSize
                                                weight:NSFontWeightBold];
  NSDictionary *la = @{
    NSFontAttributeName : lf,
    NSForegroundColorAttributeName : menuBarLabelColor()
  };
  NSSize ls = [label sizeWithAttributes:la];
  CGFloat labelW = ls.width + 4;

  CGFloat ly = barY + (barH - ls.height) / 2.0 - 1;

  [label drawAtPoint:NSMakePoint(x, ly) withAttributes:la];

  CGFloat bx = x + labelW;

  [[NSColor colorWithWhite:0.5 alpha:0.2] set];
  NSBezierPath *track =
      [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(bx, barY, barW, barH)
                                      xRadius:2
                                      yRadius:2];
  [track fill];

  if (fill > 0) {
    [color set];
    NSBezierPath *bar =
        [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(bx, barY, fill, barH)
                                        xRadius:2
                                        yRadius:2];
    [bar fill];
  }

  if (showPercent) {
    NSFont *pf = [NSFont monospacedDigitSystemFontOfSize:fontSize
                                                  weight:NSFontWeightRegular];
    NSDictionary *pa = @{
      NSFontAttributeName : pf,
      NSForegroundColorAttributeName : menuBarLabelColor()
    };
    NSString *pStr = [NSString stringWithFormat:@"%.0f%%", pct];
    CGFloat px = bx + barW + 4;
    CGFloat py = barY + (barH - [pStr sizeWithAttributes:pa].height) / 2.0 - 1;
    [pStr drawAtPoint:NSMakePoint(px, py) withAttributes:pa];
  }
}

static NSImage *drawStatusBarImage(double cpu, double gpu, double ane,
                                   double memPct, double watts) {
  int barCount = 0;
  if (g_config.show_cpu)
    barCount++;
  if (g_config.show_gpu)
    barCount++;
  if (g_config.show_ane)
    barCount++;
  if (g_config.show_memory)
    barCount++;

  // Font sizes
  CGFloat fontSize =
      (CGFloat)(g_config.font_size > 0 ? g_config.font_size : 10);
  CGFloat powerFontSize =
      (CGFloat)(g_config.power_font_size > 0 ? g_config.power_font_size : 11);

  // Calculate dimensions
  CGFloat h =
      (CGFloat)(g_config.status_bar_height > 0 ? g_config.status_bar_height
                                               : 18);
  // Ensure bounds accommodate largest font
  if (h < fontSize + 4)
    h = fontSize + 4;
  if (h < powerFontSize + 4)
    h = powerFontSize + 4;

  CGFloat gap = 6;
  CGFloat barH = h - 10; // Scale bar thickness with height
  if (barH < 4)
    barH = 4;
  CGFloat barW = (CGFloat)g_config.status_bar_width;
  CGFloat labelW = fontSize + 4;
  CGFloat percentW = g_config.show_percent ? (fontSize * 3.5) : 0.0;
  CGFloat sectionW = labelW + barW + percentW;

  CGFloat textW = 0;
  NSString *wattStr = nil;
  NSDictionary *wattAttrs = nil;

  if (g_config.show_power) {
    wattStr = [NSString stringWithFormat:@" %.1fW ", watts];
    NSFont *pf = [NSFont monospacedDigitSystemFontOfSize:powerFontSize
                                                  weight:NSFontWeightMedium];
    wattAttrs = @{
      NSFontAttributeName : pf,
      NSForegroundColorAttributeName : menuBarLabelColor()
    };
    textW = [wattStr sizeWithAttributes:wattAttrs].width;
  }

  CGFloat totalW =
      barCount * sectionW + (barCount > 0 ? (barCount - 1) * gap : 0) + 4;
  if (g_config.show_power) {
    if (barCount > 0)
      totalW += gap; // Add gap if there are other bars
    totalW += textW;
  }

  NSImage *img = [[NSImage alloc] initWithSize:NSMakeSize(totalW, h)];
  [img lockFocus];

  CGFloat barY = (h - barH) / 2.0;
  CGFloat x = 2;

  if (g_config.show_cpu) {
    drawHBar(@"C", cpu, cpuColor(), x, barY, barW, barH, g_config.show_percent);
    x += sectionW + gap;
  }
  if (g_config.show_gpu) {
    drawHBar(@"G", gpu, gpuColor(), x, barY, barW, barH, g_config.show_percent);
    x += sectionW + gap;
  }
  if (g_config.show_ane) {
    drawHBar(@"A", ane, aneColor(), x, barY, barW, barH, g_config.show_percent);
    x += sectionW + gap;
  }
  if (g_config.show_memory) {
    drawHBar(@"M", memPct, memColor(), x, barY, barW, barH,
             g_config.show_percent);
    x += sectionW + gap;
  }

  // Draw Wattage
  if (g_config.show_power && wattStr) {
    // Vertically center the text
    NSSize textSize = [wattStr sizeWithAttributes:wattAttrs];
    CGFloat ty = (h - textSize.height) / 2.0 - 1; // -1 optical adjustment
    [wattStr drawAtPoint:NSMakePoint(x, ty) withAttributes:wattAttrs];
  }

  [img unlockFocus];
  [img setTemplate:NO];
  return img;
}

// Sparkline helper
static NSImage *drawSparklineChart(double *history, int count, NSColor *color,
                                   NSString *label, double currentVal,
                                   NSString *valOverride) {
  // Same implementation
  CGFloat w = (CGFloat)g_config.sparkline_width;
  CGFloat h = (CGFloat)g_config.sparkline_height;
  NSImage *img = [[NSImage alloc] initWithSize:NSMakeSize(w, h)];
  [img lockFocus];
  CGFloat padL = 4, padR = 4, padT = 22, padB = 4;
  CGFloat chartW = w - padL - padR;
  CGFloat chartH = h - padT - padB;
  double maxVal = 100.0;
  [[NSColor colorWithWhite:1.0 alpha:0.08] set];
  for (int g = 1; g <= 3; g++) {
    CGFloat gy = padB + (chartH * (CGFloat)g / 4.0);
    NSBezierPath *gridLine = [NSBezierPath bezierPath];
    [gridLine moveToPoint:NSMakePoint(padL, gy)];
    [gridLine lineToPoint:NSMakePoint(padL + chartW, gy)];
    [gridLine setLineWidth:0.5];
    [gridLine stroke];
  }
  CGFloat barW = chartW / (CGFloat)count;
  NSBezierPath *areaPath = [NSBezierPath bezierPath];
  [areaPath moveToPoint:NSMakePoint(padL, padB)];
  for (int i = 0; i < count; i++) {
    CGFloat bh = (history[i] / maxVal) * chartH;
    CGFloat bx = padL + (CGFloat)i * barW;
    [areaPath lineToPoint:NSMakePoint(bx, padB + bh)];
    [areaPath lineToPoint:NSMakePoint(bx + barW, padB + bh)];
  }
  [areaPath lineToPoint:NSMakePoint(padL + chartW, padB)];
  [areaPath closePath];
  NSGradient *gradient = [[NSGradient alloc]
      initWithStartingColor:[color colorWithAlphaComponent:0.5]
                endingColor:[color colorWithAlphaComponent:0.1]];
  [gradient drawInBezierPath:areaPath angle:90];
  NSBezierPath *linePath = [NSBezierPath bezierPath];
  [linePath setLineWidth:1.5];
  for (int i = 0; i < count; i++) {
    CGFloat bh = (history[i] / maxVal) * chartH;
    CGFloat bx = padL + (CGFloat)i * barW;
    CGFloat by = padB + bh;
    if (i == 0)
      [linePath moveToPoint:NSMakePoint(bx, by)];
    else
      [linePath lineToPoint:NSMakePoint(bx, by)];
    [linePath lineToPoint:NSMakePoint(bx + barW, by)];
  }
  [color set];
  [linePath stroke];
  NSFont *labelFont = [NSFont systemFontOfSize:14 weight:NSFontWeightBold];
  NSDictionary *labelAttrs = @{
    NSFontAttributeName : labelFont,
    NSForegroundColorAttributeName : color
  };
  [label drawAtPoint:NSMakePoint(padL + 2, h - padT + 2)
      withAttributes:labelAttrs];
  NSString *valStr =
      valOverride ?: [NSString stringWithFormat:@"%.1f%%", currentVal];
  NSFont *valFont = [NSFont monospacedDigitSystemFontOfSize:15
                                                     weight:NSFontWeightBold];
  NSDictionary *valAttrs = @{
    NSFontAttributeName : valFont,
    NSForegroundColorAttributeName : menuBarLabelColor()
  };
  NSSize valSize = [valStr sizeWithAttributes:valAttrs];
  [valStr drawAtPoint:NSMakePoint(w - padR - valSize.width - 2, h - padT + 2)
       withAttributes:valAttrs];
  [img unlockFocus];
  [img setTemplate:NO];
  return img;
}

static NSString *formatThroughput(double bps) {
  if (bps >= 1024 * 1024 * 1024)
    return [NSString stringWithFormat:@"%.1f GB/s", bps / (1024 * 1024 * 1024)];
  if (bps >= 1024 * 1024)
    return [NSString stringWithFormat:@"%.1f MB/s", bps / (1024 * 1024)];
  if (bps >= 1024)
    return [NSString stringWithFormat:@"%.1f KB/s", bps / 1024];
  return [NSString stringWithFormat:@"%.0f B/s", bps];
}

static void persistConfig(void) {
  dispatch_async(
      dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^{
        GoSaveMenuBarConfig(
            g_config.status_bar_width, g_config.status_bar_height,
            g_config.sparkline_width, g_config.sparkline_height,
            g_config.show_cpu, g_config.show_gpu, g_config.show_ane,
            g_config.show_memory, g_config.show_power, g_config.show_percent,
            g_config.font_size, g_config.power_font_size, g_config.cpu_color,
            g_config.gpu_color, g_config.ane_color, g_config.mem_color,
            g_config.label_color);
      });
}

// refreshAllMenuColors walks the dropdown menu and re-applies textColors to
// any item.view that supports it. Required because NSTextField.textColor is
// cached at view creation, so existing items don't pick up label color
// changes from the Settings window without an explicit refresh.
static void refreshAllMenuColors(void) {
  if (!g_delegate || !g_delegate.statusMenu)
    return;
  for (NSMenuItem *it in g_delegate.statusMenu.itemArray) {
    NSView *v = it.view;
    if ([v respondsToSelector:@selector(refreshColors)]) {
      [(id)v refreshColors];
    }
  }
}

void setMenuBarConfig(menubar_config_t *cfg) {
  if (cfg) {
    g_config = *cfg;
    dispatch_async(dispatch_get_main_queue(), ^{
      refreshAllMenuColors();
      if (g_settingsWindow) {
        [g_settingsWindow syncUI];
      }
    });
  }
}

static void buildMenu(void) {
  @autoreleasepool {
    NSStatusBar *statusBar = [NSStatusBar systemStatusBar];
    g_delegate.statusItem =
        [statusBar statusItemWithLength:NSVariableStatusItemLength];
    NSStatusBarButton *button = g_delegate.statusItem.button;
    button.title = @" mactop ";
    button.toolTip = localize(@"Menu_Tooltip");
    CGFloat pfSize =
        (CGFloat)(g_config.power_font_size > 0 ? g_config.power_font_size : 11);
    button.font = [NSFont monospacedDigitSystemFontOfSize:pfSize
                                                   weight:NSFontWeightMedium];
    button.imagePosition = NSImageLeading;

    NSMenu *menu = [[NSMenu alloc] init];
    menu.autoenablesItems = NO;
    menu.minimumWidth = (CGFloat)g_config.sparkline_width + 16.0;

    [menu addItem:makeBrandingItem()];
    [menu addItem:[NSMenuItem separatorItem]];

    g_delegate.modelItem = makeHeaderItem(localize(@"Menu_AppleSilicon"));
    [menu addItem:g_delegate.modelItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_CPU"))];
    g_delegate.cpuUsageItem = makeMetricItem(localize(@"Menu_Usage"), @"\u2014");
    [menu addItem:g_delegate.cpuUsageItem];
    g_delegate.cpuEClusterItem = makeMetricItem(localize(@"Menu_ECluster"), @"\u2014");
    [menu addItem:g_delegate.cpuEClusterItem];
    g_delegate.cpuPClusterItem = makeMetricItem(localize(@"Menu_PCluster"), @"\u2014");
    [menu addItem:g_delegate.cpuPClusterItem];
    g_delegate.cpuSClusterItem = makeMetricItem(localize(@"Menu_SCluster"), @"\u2014");
    [menu addItem:g_delegate.cpuSClusterItem];
    g_delegate.cpuSClusterItem.hidden =
        YES; // Hidden until S-cluster data arrives
    g_delegate.cpuWattsItem = makeMetricItem(localize(@"Menu_Power"), @"\u2014");
    [menu addItem:g_delegate.cpuWattsItem];
    g_delegate.cpuTempItem = makeMetricItem(localize(@"Menu_Temp"), @"\u2014");
    [menu addItem:g_delegate.cpuTempItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_GPU"))];
    g_delegate.gpuUsageItem = makeMetricItem(localize(@"Menu_Usage"), @"\u2014");
    [menu addItem:g_delegate.gpuUsageItem];
    g_delegate.gpuWattsItem = makeMetricItem(localize(@"Menu_Power"), @"\u2014");
    [menu addItem:g_delegate.gpuWattsItem];
    g_delegate.gpuTflopsItem = makeMetricItem(localize(@"Menu_TFLOPs"), @"\u2014");
    [menu addItem:g_delegate.gpuTflopsItem];
    g_delegate.gpuTempItem = makeMetricItem(localize(@"Menu_Temp"), @"\u2014");
    [menu addItem:g_delegate.gpuTempItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_MEMORY"))];
    g_delegate.memUsageItem = makeMetricItem(localize(@"Menu_RAM"), @"\u2014");
    [menu addItem:g_delegate.memUsageItem];
    g_delegate.memSwapItem = makeMetricItem(localize(@"Menu_Swap"), @"\u2014");
    [menu addItem:g_delegate.memSwapItem];
    g_delegate.dramBwItem = makeMetricItem(localize(@"Menu_DRAMBW"), @"\u2014");
    [menu addItem:g_delegate.dramBwItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_NETWORK"))];
    g_delegate.netItem = makeMetricItem(localize(@"Menu_Network"), @"\u2014");
    [menu addItem:g_delegate.netItem];
    g_delegate.rdmaItem = makeMetricItem(localize(@"Menu_RDMA"), @"\u2014");
    [menu addItem:g_delegate.rdmaItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_DISK"))];
    g_delegate.diskItem = makeMetricItem(localize(@"Menu_Disk"), @"\u2014");
    [menu addItem:g_delegate.diskItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_POWER"))];
    g_delegate.powerTotalItem = makeMetricItem(localize(@"Menu_Total"), @"\u2014");
    [menu addItem:g_delegate.powerTotalItem];
    g_delegate.powerPackageItem = makeMetricItem(localize(@"Menu_System"), @"\u2014");
    [menu addItem:g_delegate.powerPackageItem];
    g_delegate.powerCpuItem = makeMetricItem([NSString stringWithFormat:@"%@:", localize(@"Menu_CPU")], @"\u2014");
    [menu addItem:g_delegate.powerCpuItem];
    g_delegate.powerGpuItem = makeMetricItem([NSString stringWithFormat:@"%@:", localize(@"Menu_GPU")], @"\u2014");
    [menu addItem:g_delegate.powerGpuItem];
    g_delegate.powerAneItem = makeMetricItem([NSString stringWithFormat:@"%@:", localize(@"Overlay_ANE")], @"\u2014");
    [menu addItem:g_delegate.powerAneItem];
    g_delegate.powerDramItem = makeMetricItem([NSString stringWithFormat:@"%@:", localize(@"Overlay_DRAM")], @"\u2014");
    [menu addItem:g_delegate.powerDramItem];
    g_delegate.thermalItem = makeMetricItem(localize(@"Menu_Thermal"), @"\u2014");
    [menu addItem:g_delegate.thermalItem];
    [menu addItem:[NSMenuItem separatorItem]];

    [menu addItem:makeHeaderItem(localize(@"Menu_HISTORY"))];
    NSImage *emptySparkCPU = drawSparklineChart(
        cpuHistory, SPARKLINE_HISTORY_SIZE, cpuColor(), localize(@"Menu_CPU"), 0,
        nil);
    g_delegate.cpuSparkItem = makeSparkItem(emptySparkCPU);
    [menu addItem:g_delegate.cpuSparkItem];
    NSImage *emptySparkGPU = drawSparklineChart(
        gpuHistory, SPARKLINE_HISTORY_SIZE, gpuColor(), localize(@"Menu_GPU"), 0,
        nil);
    g_delegate.gpuSparkItem = makeSparkItem(emptySparkGPU);
    [menu addItem:g_delegate.gpuSparkItem];
    NSImage *emptySparkANE = drawSparklineChart(
        aneHistory, SPARKLINE_HISTORY_SIZE, aneColor(), localize(@"Overlay_ANE"),
        0, nil);
    g_delegate.aneSparkItem = makeSparkItem(emptySparkANE);
    [menu addItem:g_delegate.aneSparkItem];
    NSImage *emptySparkMEM =
        drawSparklineChart(memHistory, SPARKLINE_HISTORY_SIZE, memColor(),
                           localize(@"Menu_RAM"), 0, @"0.0 / 0 GB");
    g_delegate.memSparkItem = makeSparkItem(emptySparkMEM);
    [menu addItem:g_delegate.memSparkItem];
    [menu addItem:[NSMenuItem separatorItem]];

    g_delegate.fanHeaderItem = makeHeaderItem(localize(@"Menu_FANS"));
    [menu addItem:g_delegate.fanHeaderItem];
    g_delegate.fan0Item =
        makeMetricItem([NSString stringWithFormat:@"%@:", [NSString stringWithFormat:localize(@"Menu_FanItem"), 0]],
                       @"\u2014");
    [menu addItem:g_delegate.fan0Item];
    g_delegate.fan1Item =
        makeMetricItem([NSString stringWithFormat:@"%@:", [NSString stringWithFormat:localize(@"Menu_FanItem"), 1]],
                       @"\u2014");
    [menu addItem:g_delegate.fan1Item];
    g_delegate.fan2Item =
        makeMetricItem([NSString stringWithFormat:@"%@:", [NSString stringWithFormat:localize(@"Menu_FanItem"), 2]],
                       @"\u2014");
    [menu addItem:g_delegate.fan2Item];
    g_delegate.fan3Item =
        makeMetricItem([NSString stringWithFormat:@"%@:", [NSString stringWithFormat:localize(@"Menu_FanItem"), 3]],
                       @"\u2014");
    [menu addItem:g_delegate.fan3Item];
    g_delegate.fanSepItem = [NSMenuItem separatorItem];
    [menu addItem:g_delegate.fanSepItem];

    NSMenuItem *settingsItem =
        [[NSMenuItem alloc] initWithTitle:localize(@"Menu_Settings")
                                   action:@selector(openSettings:)
                            keyEquivalent:@","];
    settingsItem.target = g_delegate;
    [menu addItem:settingsItem];
    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *ghItem =
        [[NSMenuItem alloc] initWithTitle:localize(@"Menu_OpenGitHub")
                                   action:@selector(openGitHub:)
                            keyEquivalent:@""];
    ghItem.target = g_delegate;
    [menu addItem:ghItem];
    NSMenuItem *tuiItem =
        [[NSMenuItem alloc] initWithTitle:localize(@"Menu_OpenTUI")
                                   action:@selector(openTUI:)
                            keyEquivalent:@"t"];
    tuiItem.target = g_delegate;
    [menu addItem:tuiItem];
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:localize(@"Menu_Quit")
                                                      action:@selector(quitApp:)
                                               keyEquivalent:@"q"];
    quitItem.target = g_delegate;
    [menu addItem:quitItem];
    g_delegate.statusMenu = menu;

    // Do NOT set statusItem.menu — that couples menu width to button width.
    // Instead, handle clicks manually to present the menu independently.
    g_delegate.statusItem.button.action = @selector(statusBarClicked:);
    g_delegate.statusItem.button.target = g_delegate;
    [g_delegate.statusItem.button
        sendActionOn:NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp];
  }
}

int startMenuBarInBackground(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    g_delegate = [[MactopMenuBarDelegate alloc] init];
    [NSApp setDelegate:g_delegate];
    buildMenu();
    [NSApp finishLaunching];
    return 0;
  }
}
int initMenuBar(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    g_delegate = [[MactopMenuBarDelegate alloc] init];
    [NSApp setDelegate:g_delegate];
    buildMenu();
    return 0;
  }
}
void updateMenuBarMetrics(menubar_metrics_t *m) {
  @autoreleasepool {
    if (g_delegate == nil || m == NULL)
      return;
    menubar_metrics_t copy = *m;
    NSValue *val = [NSValue valueWithBytes:&copy
                                  objCType:@encode(menubar_metrics_t)];
    dispatch_async(dispatch_get_main_queue(), ^{
      // The inner pool is optional now but kept for safety/consistency if more is added later.
      @autoreleasepool {
        [g_delegate performMetricUpdate:val];
      }
    });
  }
}
void pumpMenuBarEvents(void) {
  @autoreleasepool {
    NSEvent *event;
    while ((event = [NSApp nextEventMatchingMask:NSEventMaskAny
                                       untilDate:nil
                                          inMode:NSDefaultRunLoopMode
                                         dequeue:YES])) {
      [NSApp sendEvent:event];
    }
  }
}
void runMenuBarLoop(void) { [NSApp run]; }
void cleanupMenuBar(void) {
  if (g_delegate != nil) {
    if (g_delegate.statusItem != nil) {
      [[NSStatusBar systemStatusBar] removeStatusItem:g_delegate.statusItem];
      g_delegate.statusItem = nil;
    }
    g_delegate = nil;
  }
}

@implementation MactopMenuBarDelegate
- (void)quitApp:(id)sender {
  [NSApp terminate:nil];
}
- (void)openTUI:(id)sender {
  NSString *processPath =
      [[NSProcessInfo processInfo] arguments].firstObject ?: @"mactop";
  NSString *script =
      [NSString stringWithFormat:@"tell application \"Terminal\"\n  activate\n "
                                 @" do script \"%@\"\nend tell",
                                 processPath];
  NSAppleScript *appleScript = [[NSAppleScript alloc] initWithSource:script];
  [appleScript executeAndReturnError:nil];
}
- (void)openGitHub:(id)sender {
  [[NSWorkspace sharedWorkspace]
      openURL:[NSURL URLWithString:@"https://github.com/metaspartan/mactop"]];
}
- (void)openSettings:(id)sender {
  if (g_settingsWindow == nil) {
    g_settingsWindow = [[SettingsWindowController alloc] init];
  }
  [g_settingsWindow showWindow:sender];
}
- (void)statusBarClicked:(id)sender {
  NSStatusBarButton *button = self.statusItem.button;
  // Anchor first item at the top of the button so menu drops below menu bar
  NSMenuItem *first = self.statusMenu.itemArray.firstObject;
  [self.statusMenu
      popUpMenuPositioningItem:first
                    atLocation:NSMakePoint(0, button.bounds.size.height + 6)
                        inView:button];
}
- (void)performMetricUpdate:(NSValue *)val {
  menubar_metrics_t metrics;
  [val getValue:&metrics];
  [self doUpdate:&metrics];
}
- (void)doUpdate:(menubar_metrics_t *)mptr {
  @autoreleasepool {
    menubar_metrics_t metrics = *mptr;
    pushHistory(cpuHistory, metrics.cpu_percent);
    pushHistory(gpuHistory, metrics.gpu_percent);
    pushHistory(aneHistory, metrics.ane_percent);
    double memPct = 0;
    if (metrics.mem_total_bytes > 0) {
      memPct = (double)metrics.mem_used_bytes /
               (double)metrics.mem_total_bytes * 100.0;
    }
    pushHistory(memHistory, memPct);

    self.statusItem.button.image =
        drawStatusBarImage(metrics.cpu_percent, metrics.gpu_percent,
                           metrics.ane_percent, memPct, metrics.total_watts);

    // Clear title as it is now drawn in image
    self.statusItem.button.title = @"";

    // Update menu views...
    MactopLabelView *mv = (MactopLabelView *)self.modelItem.view;
    // Build dynamic core summary — only show core types with non-zero counts
    NSMutableString *coreStr = [NSMutableString string];
    if (metrics.e_core_count > 0) {
      [coreStr appendFormat:@"%dE + ", metrics.e_core_count];
    }
    [coreStr appendFormat:@"%dP", metrics.p_core_count];
    if (metrics.s_core_count > 0) {
      [coreStr appendFormat:@" + %dS", metrics.s_core_count];
    }
    [coreStr appendFormat:@" + %dGPU", metrics.gpu_core_count];
    mv.label.stringValue = [NSString
        stringWithFormat:@"%s  (%@)", metrics.model_name, coreStr];

    MactopMetricView *v = (MactopMetricView *)self.cpuUsageItem.view;
    [v setTwoToneLabel:localize(@"Menu_Usage")
                 value:[NSString
                           stringWithFormat:@"%.1f%%", metrics.cpu_percent]];
    // E-Cluster: hide when e_core_count is 0 (M5+ has no E-cores)
    if (metrics.e_core_count > 0) {
      self.cpuEClusterItem.hidden = NO;
      v = (MactopMetricView *)self.cpuEClusterItem.view;
      [v setTwoToneLabel:localize(@"Menu_ECluster")
                   value:[NSString stringWithFormat:@"%d MHz (%.1f%%)",
                                                    metrics.ecluster_freq_mhz,
                                                    metrics.ecluster_active]];
    } else {
      self.cpuEClusterItem.hidden = YES;
    }
    v = (MactopMetricView *)self.cpuPClusterItem.view;
    [v setTwoToneLabel:localize(@"Menu_PCluster")
                 value:[NSString stringWithFormat:@"%d MHz (%.1f%%)",
                                                  metrics.pcluster_freq_mhz,
                                                  metrics.pcluster_active]];

    // S-Cluster: only show when data is present (M5+)
    if (metrics.scluster_freq_mhz > 0 || metrics.scluster_active > 0) {
      self.cpuSClusterItem.hidden = NO;
      v = (MactopMetricView *)self.cpuSClusterItem.view;
      [v setTwoToneLabel:localize(@"Menu_SCluster")
                   value:[NSString stringWithFormat:@"%d MHz (%.1f%%)",
                                                    metrics.scluster_freq_mhz,
                                                    metrics.scluster_active]];
    }
    v = (MactopMetricView *)self.cpuWattsItem.view;
    [v setTwoToneLabel:localize(@"Menu_Power")
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.cpu_watts]];
    v = (MactopMetricView *)self.cpuTempItem.view;
    [v setTwoToneLabel:localize(@"Menu_Temp")
                 value:[NSString stringWithFormat:@"%.1f°C", metrics.cpu_temp]];

    v = (MactopMetricView *)self.gpuUsageItem.view;
    [v setTwoToneLabel:localize(@"Menu_Usage")
                 value:[NSString stringWithFormat:@"%.1f%% (%d MHz)",
                                                  metrics.gpu_percent,
                                                  metrics.gpu_freq_mhz]];
    v = (MactopMetricView *)self.gpuWattsItem.view;
    [v setTwoToneLabel:localize(@"Menu_Power")
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.gpu_watts]];
    double activeTF = (metrics.gpu_percent / 100.0) * metrics.tflops_fp32;
    v = (MactopMetricView *)self.gpuTflopsItem.view;
    [v setTwoToneLabel:localize(@"Menu_TFLOPs")
                 value:[NSString stringWithFormat:@"%.2f / %.2f FP32", activeTF,
                                                  metrics.tflops_fp32]];
    v = (MactopMetricView *)self.gpuTempItem.view;
    [v setTwoToneLabel:localize(@"Menu_Temp")
                 value:[NSString stringWithFormat:@"%.1f°C", metrics.gpu_temp]];

    double memUsedGB =
        (double)metrics.mem_used_bytes / (1024.0 * 1024.0 * 1024.0);
    double memTotalGB =
        (double)metrics.mem_total_bytes / (1024.0 * 1024.0 * 1024.0);
    v = (MactopMetricView *)self.memUsageItem.view;
    [v setTwoToneLabel:localize(@"Menu_RAM")
                 value:[NSString stringWithFormat:@"%.1f / %.0f GB (%.1f%%)",
                                                  memUsedGB, memTotalGB,
                                                  memPct]];
    double swapUsedGB =
        (double)metrics.swap_used_bytes / (1024.0 * 1024.0 * 1024.0);
    double swapTotalGB =
        (double)metrics.swap_total_bytes / (1024.0 * 1024.0 * 1024.0);
    v = (MactopMetricView *)self.memSwapItem.view;
    [v setTwoToneLabel:localize(@"Menu_Swap")
                 value:[NSString stringWithFormat:@"%.1f / %.1f GB", swapUsedGB,
                                                  swapTotalGB]];
    v = (MactopMetricView *)self.dramBwItem.view;
    [v setTwoToneLabel:localize(@"Menu_DRAMBW")
                 value:[NSString
                           stringWithFormat:@"%.1f GB/s",
                                            metrics.dram_bw_combined_gbs]];

    v = (MactopMetricView *)self.netItem.view;
    [v setTwoToneLabel:localize(@"Menu_Network")
                 value:[NSString
                           stringWithFormat:@"↓ %@  ↑ %@",
                                            formatThroughput(
                                                metrics.net_in_bytes_per_sec),
                                            formatThroughput(
                                                metrics
                                                    .net_out_bytes_per_sec)]];
    v = (MactopMetricView *)self.rdmaItem.view;
    [v setTwoToneLabel:localize(@"Menu_RDMA")
                 value:[NSString stringWithUTF8String:metrics.rdma_status]];

    v = (MactopMetricView *)self.diskItem.view;
    [v setTwoToneLabel:localize(@"Menu_Disk")
                 value:[NSString stringWithFormat:localize(@"Menu_DiskRate"),
                                                  metrics.disk_read_kb_per_sec,
                                                  metrics.disk_write_kb_per_sec]];

    v = (MactopMetricView *)self.powerTotalItem.view;
    [v setTwoToneLabel:localize(@"Menu_Total")
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.total_watts]];
    v = (MactopMetricView *)self.powerPackageItem.view;
    [v setTwoToneLabel:localize(@"Menu_System")
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.package_watts]];
    v = (MactopMetricView *)self.powerCpuItem.view;
    [v setTwoToneLabel:[NSString stringWithFormat:@"%@:", localize(@"Menu_CPU")]
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.cpu_watts]];
    v = (MactopMetricView *)self.powerGpuItem.view;
    [v setTwoToneLabel:[NSString stringWithFormat:@"%@:", localize(@"Menu_GPU")]
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.gpu_watts]];
    v = (MactopMetricView *)self.powerAneItem.view;
    [v setTwoToneLabel:[NSString stringWithFormat:@"%@:", localize(@"Overlay_ANE")]
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.ane_watts]];
    v = (MactopMetricView *)self.powerDramItem.view;
    [v setTwoToneLabel:[NSString stringWithFormat:@"%@:", localize(@"Overlay_Memory")]
                 value:[NSString
                           stringWithFormat:@"%.2f W", metrics.dram_watts]];
    v = (MactopMetricView *)self.thermalItem.view;
    [v setTwoToneLabel:localize(@"Menu_Thermal")
                 value:[NSString stringWithUTF8String:metrics.thermal_state]];

    // Sparklines
    MactopImageView *iv = (MactopImageView *)self.cpuSparkItem.view;
    iv.imageView.image =
        drawSparklineChart(cpuHistory, SPARKLINE_HISTORY_SIZE, cpuColor(),
                           localize(@"Menu_CPU"), metrics.cpu_percent, nil);
    iv = (MactopImageView *)self.gpuSparkItem.view;
    iv.imageView.image =
        drawSparklineChart(gpuHistory, SPARKLINE_HISTORY_SIZE, gpuColor(),
                           localize(@"Menu_GPU"), metrics.gpu_percent, nil);
    iv = (MactopImageView *)self.aneSparkItem.view;
    iv.imageView.image =
        drawSparklineChart(aneHistory, SPARKLINE_HISTORY_SIZE, aneColor(),
                           localize(@"Overlay_ANE"), metrics.ane_percent, nil);
    NSString *memValStr =
        [NSString stringWithFormat:@"%.1f / %.0f GB", memUsedGB, memTotalGB];
    iv = (MactopImageView *)self.memSparkItem.view;
    iv.imageView.image =
        drawSparklineChart(memHistory, SPARKLINE_HISTORY_SIZE, memColor(),
                           localize(@"Menu_RAM"), memPct, memValStr);

    // Fan data — show/hide individual items based on count
    NSMenuItem *fanItems[4] = {self.fan0Item, self.fan1Item, self.fan2Item, self.fan3Item};
    for (int i = 0; i < 4; i++) {
      if (i < metrics.fan_count) {
        fanItems[i].hidden = NO;
        v = (MactopMetricView *)fanItems[i].view;
        NSString *name = [NSString stringWithUTF8String:metrics.fan_name[i]];
        if (name.length == 0)
          name = [NSString stringWithFormat:localize(@"Menu_FanItem"), i];
        [v setTwoToneLabel:[NSString stringWithFormat:@"%@:", name]
                     value:[NSString stringWithFormat:@"%d RPM", metrics.fan_rpm[i]]];
      } else {
        fanItems[i].hidden = YES;
      }
    }
    // Hide entire FANS section if no fans
    self.fanHeaderItem.hidden = (metrics.fan_count == 0);
    self.fanSepItem.hidden = (metrics.fan_count == 0);
  } // @autoreleasepool
}
@end
