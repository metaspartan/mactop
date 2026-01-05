//go:build !headless

package app

import (
	"strconv"
	"testing"
)

// TestNativeProfilerIntegration tests the actual integration with IOKit native calls.
// This test runs in non-headless mode (default) to ensure it picks up the real environment.
func TestNativeProfilerIntegration(t *testing.T) {
	data, err := GetGlobalProfilerData()
	if err != nil {
		t.Fatalf("Failed to get global profiler data: %v", err)
	}

	if data == nil {
		t.Fatal("Global profiler data is nil")
	}

	t.Logf("Found %d Thunderbolt buses", len(data.ThunderboltItems))
	for i, bus := range data.ThunderboltItems {
		t.Logf("Bus %d: %s (%s)", i, bus.Name, bus.SwitchUID)
		if bus.SwitchUID == "" {
			t.Errorf("Bus %d has empty SwitchUID", i)
		}
		if len(bus.ConnectedDevs) > 0 {
			t.Logf("  Found %d devices on bus %s", len(bus.ConnectedDevs), bus.Name)
			for _, dev := range bus.ConnectedDevs {
				t.Logf("    Device: %s (Mode: %s)", dev.Name, dev.Mode)
			}
		}
	}

	if len(data.USBItems) == 0 {
		t.Log("No USB buses found (might be expected depending on environment)")
	} else {
		bus := data.USBItems[0]
		t.Logf("Found USB Bus with %d devices", len(bus.USBDevices))
	}

	if len(data.StorageItems) == 0 {
		t.Error("No storage items found, expected at least boot drive")
	} else {
		t.Logf("Found %d storage items", len(data.StorageItems))
		for _, item := range data.StorageItems {
			t.Logf("  Storage: %s (Internal: %s)", item.Name, item.PhysicalDrive.IsInternal)
		}
	}

	if len(data.DisplayItems) == 0 {
		t.Error("No Display items found (should contain GPU core info)")
	} else {
		gpu := data.DisplayItems[0]
		t.Logf("GPU Found: %s with %s cores", gpu.Name, gpu.Cores)
		cores, err := strconv.Atoi(gpu.Cores)
		if err != nil || cores <= 0 {
			t.Errorf("Invalid GPU core count: %s", gpu.Cores)
		}
	}
}
