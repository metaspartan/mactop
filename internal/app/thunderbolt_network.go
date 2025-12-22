// Copyright (c) 2024-2026 Carsen Klock under MIT License
// thunderbolt_network.go - Thunderbolt network interface monitoring

package app

import (
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

// ThunderboltNetStats holds per-interface stats for a Thunderbolt network interface
type ThunderboltNetStats struct {
	InterfaceName  string  `json:"interface_name"`
	BytesIn        uint64  `json:"bytes_in"`
	BytesOut       uint64  `json:"bytes_out"`
	BytesInPerSec  float64 `json:"bytes_in_per_sec"`
	BytesOutPerSec float64 `json:"bytes_out_per_sec"`
	PacketsIn      uint64  `json:"packets_in"`
	PacketsOut     uint64  `json:"packets_out"`
}

var (
	tbNetMutex          sync.Mutex
	lastTBNetStats      map[string]net.IOCountersStat
	lastTBNetUpdateTime time.Time
	tbBridgeMembers     map[string]bool // Cached bridge member interfaces
	tbBridgeMembersInit bool
)

// getTBBridgeMembers returns the interface names that are Thunderbolt network interfaces
func getTBBridgeMembers() map[string]bool {
	if tbBridgeMembersInit {
		return tbBridgeMembers
	}

	tbBridgeMembers = make(map[string]bool)

	cmd := exec.Command("ifconfig", "bridge0")
	out, err := cmd.Output()
	if err == nil {
		tbBridgeMembers["bridge0"] = true
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "member:") {
				// Format: "member: en2 flags=..."
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					tbBridgeMembers[parts[1]] = true
				}
			}
		}
	}

	cmd2 := exec.Command("networksetup", "-listallhardwareports")
	out2, err := cmd2.Output()
	if err == nil {
		lines := strings.Split(string(out2), "\n")
		for i, line := range lines {
			if strings.Contains(line, "Thunderbolt") {
				// Next line should have "Device: enX"
				if i+1 < len(lines) {
					deviceLine := lines[i+1]
					if strings.HasPrefix(deviceLine, "Device:") {
						parts := strings.Fields(deviceLine)
						if len(parts) >= 2 {
							tbBridgeMembers[parts[1]] = true
						}
					}
				}
			}
		}
	}

	tbBridgeMembersInit = true
	return tbBridgeMembers
}

// isThunderboltInterface checks if an interface name indicates a Thunderbolt bridge
func isThunderboltInterface(name string) bool {
	members := getTBBridgeMembers()
	if members[name] {
		return true
	}

	return strings.HasPrefix(name, "tb") ||
		strings.Contains(strings.ToLower(name), "thunderbolt")
}

// GetThunderboltNetStats returns network statistics for all Thunderbolt interfaces
func GetThunderboltNetStats() []ThunderboltNetStats {
	tbNetMutex.Lock()
	defer tbNetMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(lastTBNetUpdateTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	// Get per-interface stats (true = per-interface)
	stats, err := net.IOCounters(true)
	if err != nil {
		return nil
	}

	var result []ThunderboltNetStats

	currentStats := make(map[string]net.IOCountersStat)
	for _, stat := range stats {
		if !isThunderboltInterface(stat.Name) {
			continue
		}

		currentStats[stat.Name] = stat

		tbStat := ThunderboltNetStats{
			InterfaceName: stat.Name,
			BytesIn:       stat.BytesRecv,
			BytesOut:      stat.BytesSent,
			PacketsIn:     stat.PacketsRecv,
			PacketsOut:    stat.PacketsSent,
		}

		// Calculate per-second rates if we have previous data
		if prev, ok := lastTBNetStats[stat.Name]; ok && !lastTBNetUpdateTime.IsZero() {
			tbStat.BytesInPerSec = float64(stat.BytesRecv-prev.BytesRecv) / elapsed
			tbStat.BytesOutPerSec = float64(stat.BytesSent-prev.BytesSent) / elapsed
		}

		result = append(result, tbStat)
	}

	lastTBNetStats = currentStats
	lastTBNetUpdateTime = now

	return result
}
