// Copyright (c) 2024-2026 Carsen Klock under MIT License
// rdma.go - RDMA over Thunderbolt detection

package app

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RDMADevice holds information about a single RDMA device
type RDMADevice struct {
	Name      string `json:"name" yaml:"name" xml:"Name"`
	NodeGUID  string `json:"node_guid" yaml:"node_guid" xml:"NodeGUID"`
	Transport string `json:"transport" yaml:"transport" xml:"Transport"`
	PortState string `json:"port_state" yaml:"port_state" xml:"PortState"`
	ActiveMTU int    `json:"active_mtu" yaml:"active_mtu" xml:"ActiveMTU"`
	LinkLayer string `json:"link_layer" yaml:"link_layer" xml:"LinkLayer"`
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty" xml:"Interface,omitempty"` // Mapped network interface (e.g., en2)
}

// RDMAStatus holds RDMA availability information
type RDMAStatus struct {
	Available bool         `json:"available" yaml:"available" xml:"Available"`
	Status    string       `json:"status" yaml:"status" xml:"Status"`
	Devices   []RDMADevice `json:"devices,omitempty" yaml:"devices,omitempty" xml:"Devices,omitempty"`
}

var (
	rdmaMutex      sync.Mutex
	lastRDMAStatus RDMAStatus
	lastRDMACheck  time.Time
	rdmaCacheTTL   = 10 * time.Second // Cache RDMA status for 10 seconds
)

// CheckRDMAAvailable checks if RDMA over Thunderbolt is enabled
// Uses `rdma_ctl status` command available on macOS 26.2+
func CheckRDMAAvailable() RDMAStatus {
	rdmaMutex.Lock()
	defer rdmaMutex.Unlock()

	// Return cached result if recent enough
	if time.Since(lastRDMACheck) < rdmaCacheTTL {
		return lastRDMAStatus
	}

	status := RDMAStatus{
		Available: false,
		Status:    "Unknown",
	}

	// Try to run rdma_ctl status
	cmd := exec.Command("rdma_ctl", "status")
	output, err := cmd.Output()
	if err != nil {
		// Command not found or failed - RDMA not available
		status.Status = "RDMA not available (rdma_ctl not found or macOS < 26.2)"
		lastRDMAStatus = status
		lastRDMACheck = time.Now()
		return status
	}

	result := strings.TrimSpace(string(output))
	result = strings.ToLower(result)

	if strings.Contains(result, "enabled") {
		status.Available = true
		status.Status = "RDMA Enabled"
		// Enumerate RDMA devices when enabled
		status.Devices = GetRDMADevices()
	} else if strings.Contains(result, "disabled") {
		status.Available = false
		status.Status = "RDMA Disabled (use rdma_ctl enable in Recovery Mode)"
	} else {
		status.Available = strings.Contains(result, "enabled")
		status.Status = "RDMA: " + strings.TrimSpace(string(output))
	}

	lastRDMAStatus = status
	lastRDMACheck = time.Now()
	return status
}

// GetRDMADevices enumerates RDMA devices by parsing ibv_devinfo output
func GetRDMADevices() []RDMADevice {
	var devices []RDMADevice

	cmd := exec.Command("ibv_devinfo")
	output, err := cmd.Output()
	if err != nil {
		return devices
	}

	lines := strings.Split(string(output), "\n")
	var currentDevice *RDMADevice

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// New device starts with "hca_id:"
		if strings.HasPrefix(line, "hca_id:") {
			if currentDevice != nil {
				finalizeRDMADevice(currentDevice)
				devices = append(devices, *currentDevice)
			}
			currentDevice = &RDMADevice{
				Name: strings.TrimSpace(strings.TrimPrefix(line, "hca_id:")),
			}
			continue
		}

		if currentDevice != nil {
			parseRDMADeviceLine(line, currentDevice)
		}
	}

	// Don't forget the last device
	if currentDevice != nil {
		finalizeRDMADevice(currentDevice)
		devices = append(devices, *currentDevice)
	}

	return devices
}

// finalizeRDMADevice derives the network interface from the device name
func finalizeRDMADevice(device *RDMADevice) {
	if devi, ok := strings.CutPrefix(device.Name, "rdma_"); ok {
		device.Interface = devi
	}
}

// parseRDMADeviceLine parses a single line of ibv_devinfo output into the device
func parseRDMADeviceLine(line string, device *RDMADevice) {
	switch {
	case strings.HasPrefix(line, "transport:"):
		device.Transport = parseRDMAFieldWithParens(line)
	case strings.HasPrefix(line, "node_guid:"):
		device.NodeGUID = parseRDMAField(line)
	case strings.HasPrefix(line, "state:"):
		device.PortState = parseRDMAFieldWithParens(line)
	case strings.HasPrefix(line, "active_mtu:"):
		device.ActiveMTU = parseRDMAMTU(line)
	case strings.HasPrefix(line, "link_layer:"):
		device.LinkLayer = parseRDMAField(line)
	}
}

// parseRDMAField extracts the value after the colon
func parseRDMAField(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// parseRDMAFieldWithParens extracts the value and removes parenthetical suffix
func parseRDMAFieldWithParens(line string) string {
	value := parseRDMAField(line)
	if idx := strings.Index(value, "("); idx > 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

// parseRDMAMTU extracts the MTU integer value
func parseRDMAMTU(line string) int {
	mtuStr := parseRDMAField(line)
	if idx := strings.Index(mtuStr, " "); idx > 0 {
		mtuStr = mtuStr[:idx]
	}
	if mtu, err := strconv.Atoi(mtuStr); err == nil {
		return mtu
	}
	return 0
}
