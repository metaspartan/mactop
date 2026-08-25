package app

import (
	"testing"
	"time"
)

func TestProcessRefreshDueUsesTwoSecondWallClockInterval(t *testing.T) {
	start := time.Date(2026, time.August, 23, 14, 0, 0, 0, time.UTC)
	if !processRefreshDue(time.Time{}, start) {
		t.Fatal("first process refresh must be due")
	}
	if processRefreshDue(start, start.Add(processRefreshInterval-time.Nanosecond)) {
		t.Fatal("process refresh must wait for the full interval")
	}
	if !processRefreshDue(start, start.Add(processRefreshInterval)) {
		t.Fatal("process refresh must be due at the interval boundary")
	}
}
