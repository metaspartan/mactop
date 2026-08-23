package app

import (
	"testing"
	"time"
)

func TestUnifiedPeakTrackerUsesTimestampWindows(t *testing.T) {
	tracker := newUnifiedPeakTracker(16)
	now := time.Date(2026, time.August, 21, 12, 0, 0, 0, time.UTC)
	tracker.record(now.Add(-time.Hour-time.Second), 99, 99, 99)
	tracker.record(now.Add(-6*time.Minute), 80, 70, 60)
	tracker.record(now.Add(-5*time.Minute), 85, 75, 65)
	tracker.record(now.Add(-2*time.Minute), 70, 60, 50)

	got := tracker.peaks(now)
	if got.cpu5m != 85 || got.cpu1h != 85 || got.cpuAll != 99 || got.gpu5m != 75 || got.gpu1h != 75 || got.gpuAll != 99 {
		t.Fatalf("CPU/GPU peaks = %#v, want five-minute boundary included, one-hour expiry, and lifetime maximum", got)
	}
	if got.ane5m != 65 || got.ane1h != 65 || got.aneAll != 99 {
		t.Fatalf("ANE peaks = %#v, want five-minute boundary included, one-hour expiry, and lifetime maximum", got)
	}
}

func TestUnifiedPeakTrackerAggregatesFiveSecondBuckets(t *testing.T) {
	tracker := newUnifiedPeakTracker(4)
	now := time.Date(2026, time.August, 21, 12, 0, 0, 0, time.UTC)
	tracker.record(now, 20, 30, 40)
	tracker.record(now.Add(4*time.Second), 70, 80, 90)
	if tracker.count != 1 {
		t.Fatalf("bucket count = %d, want one five-second bucket", tracker.count)
	}
	got := tracker.peaks(now.Add(time.Second))
	if got.cpu5m != 70 || got.gpu5m != 80 || got.ane5m != 90 {
		t.Fatalf("bucket peaks = %#v, want per-five-second maxima", got)
	}
}

func TestUnifiedWindowedPeakTitles(t *testing.T) {
	peaks := unifiedWindowPeaks{cpu5m: 100, cpu1h: 80, cpuAll: 70, gpu5m: 60, gpu1h: 50, gpuAll: 40, ane5m: 30, ane1h: 20, aneAll: 10}
	if got, want := unifiedComputeWindowedPeakTitle(peaks), "Compute (C 5m/1h/all: 100/80/70%, G: 60/50/40%, A: 30/20/10%)"; got != want {
		t.Fatalf("compute title = %q, want %q", got, want)
	}
}
