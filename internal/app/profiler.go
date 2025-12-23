package app

import (
	"encoding/json"
	"os/exec"
	"sync"
)

type GlobalProfilerData struct {
	ThunderboltItems []ThunderboltBus `json:"SPThunderboltDataType"`
	StorageItems     []StorageItem    `json:"SPStorageDataType"`
	USBItems         []USBBus         `json:"SPUSBDataType"`
	DisplayItems     []DisplayItem    `json:"SPDisplaysDataType"`
}

type DisplayItem struct {
	Name   string `json:"_name"`
	Cores  string `json:"sppci_cores"`
	Model  string `json:"sppci_model"`
	Vendor string `json:"spdisplays_vendor"`
}

var (
	globalProfilerCache *GlobalProfilerData
	profilerMutex       sync.Mutex
)

// GetGlobalProfilerData returns the combined system_profiler data.
// It fetches once per execution (singleton) or can be refreshed if needed.
// For now, we fetch once at startup to minimize latency.
func GetGlobalProfilerData() (*GlobalProfilerData, error) {
	profilerMutex.Lock()
	defer profilerMutex.Unlock()

	if globalProfilerCache != nil {
		return globalProfilerCache, nil
	}

	// Combined call for maximum efficiency
	cmd := exec.Command("system_profiler", "-json", "-detailLevel", "basic",
		"SPThunderboltDataType",
		"SPStorageDataType",
		"SPUSBDataType",
		"SPDisplaysDataType",
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var data GlobalProfilerData
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}

	globalProfilerCache = &data
	return globalProfilerCache, nil
}
