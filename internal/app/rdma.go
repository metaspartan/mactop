// Copyright (c) 2024-2026 Carsen Klock under MIT License
// rdma.go - RDMA over Thunderbolt detection

package app

import (
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RDMAStatus holds RDMA availability information
type RDMAStatus struct {
	Available bool   `json:"available"`
	Status    string `json:"status"`
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
	} else if strings.Contains(result, "disabled") {
		status.Available = false
		status.Status = "RDMA Disabled (use rdma_ctl enable in Recovery Mode)"
	} else {
		status.Status = "RDMA Status: " + strings.TrimSpace(string(output))
	}

	lastRDMAStatus = status
	lastRDMACheck = time.Now()
	return status
}
