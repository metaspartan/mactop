// Copyright (c) 2024-2026 Carsen Klock under MIT License
// thunderbolt_network.go - Thunderbolt network interface monitoring

package app

import (
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
)

// isThunderboltInterface checks if an interface name indicates a Thunderbolt bridge
func isThunderboltInterface(name string) bool {
	// Common Thunderbolt interface patterns on macOS
	return strings.HasPrefix(name, "bridge") ||
		strings.Contains(strings.ToLower(name), "thunderbolt") ||
		strings.HasPrefix(name, "tb") // tb0, tb1, etc. for RDMA interfaces
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
