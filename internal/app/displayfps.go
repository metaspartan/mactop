// Copyright (c) 2024-2026 Carsen Klock under MIT License
// displayfps.go - Go wrappers for standalone display FPS counter
package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreGraphics

int startDisplayFPSCounter(void);
void stopDisplayFPSCounter(void);
unsigned int getDisplayFPS(void);
unsigned int getDisplayFrameIntervalUs(void);
void dumpDisplayFPSDiagnostics(void);
*/
import "C"

// DisplayFPSMetrics holds the current display FPS and frame interval
type DisplayFPSMetrics struct {
	FPS             uint32  // Current display FPS
	FrameIntervalMs float64 // Average frame interval in milliseconds
}

// StartDisplayFPSCounter initializes the CGDisplayStream-based FPS counter.
// Returns true if the counter was started successfully, false if CGDisplayStream
// is unavailable (e.g., headless server with no display).
func StartDisplayFPSCounter() bool {
	return C.startDisplayFPSCounter() == 0
}

// StopDisplayFPSCounter tears down the FPS counter and releases resources.
func StopDisplayFPSCounter() {
	C.stopDisplayFPSCounter()
}

// GetDisplayFPSMetrics returns the current display FPS and frame interval.
func GetDisplayFPSMetrics() DisplayFPSMetrics {
	fps := uint32(C.getDisplayFPS())
	intervalUs := uint32(C.getDisplayFrameIntervalUs())
	return DisplayFPSMetrics{
		FPS:             fps,
		FrameIntervalMs: float64(intervalUs) / 1000.0,
	}
}

// DumpDisplayFPSDiagnostics prints comprehensive CGDisplayStream diagnostic info
// including display hardware, screen recording permissions, symbol loading status,
// and stream creation tests at multiple output sizes.
func DumpDisplayFPSDiagnostics() {
	C.dumpDisplayFPSDiagnostics()
}
