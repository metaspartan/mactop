package app

import (
	"math"
	"sync"
	"time"
)

const (
	unifiedPeakWindow5m = 5 * time.Minute
	unifiedPeakWindow1h = time.Hour
	unifiedPeakBucket   = 5 * time.Second
	// Five-second maximum buckets preserve useful windowed peaks without keeping
	// every 100ms sample. One extra bucket covers an inclusive hour boundary.
	unifiedPeakSampleCapacity = int(unifiedPeakWindow1h/unifiedPeakBucket) + 1
)

type unifiedPeakSample struct {
	at            int64
	cpu, gpu, ane float64
}

type unifiedWindowPeaks struct {
	cpu5m, cpu1h, cpuAll float64
	gpu5m, gpu1h, gpuAll float64
	ane5m, ane1h, aneAll float64
}

type unifiedPeakTracker struct {
	mu      sync.Mutex
	samples []unifiedPeakSample
	start   int
	count   int
	all     unifiedWindowPeaks
}

func newUnifiedPeakTracker(capacity int) *unifiedPeakTracker {
	return &unifiedPeakTracker{samples: make([]unifiedPeakSample, capacity)}
}

func (t *unifiedPeakTracker) record(at time.Time, cpu, gpu, ane float64) {
	if t == nil || len(t.samples) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	sample := unifiedPeakSample{
		at:  at.Truncate(unifiedPeakBucket).UnixNano(),
		cpu: peakValue(cpu),
		gpu: peakValue(gpu),
		ane: peakValue(ane),
	}
	t.all.cpuAll = max(t.all.cpuAll, sample.cpu)
	t.all.gpuAll = max(t.all.gpuAll, sample.gpu)
	t.all.aneAll = max(t.all.aneAll, sample.ane)
	if t.count > 0 {
		index := (t.start + t.count - 1) % len(t.samples)
		bucket := &t.samples[index]
		if bucket.at == sample.at {
			bucket.cpu = max(bucket.cpu, sample.cpu)
			bucket.gpu = max(bucket.gpu, sample.gpu)
			bucket.ane = max(bucket.ane, sample.ane)
			return
		}
	}
	index := (t.start + t.count) % len(t.samples)
	if t.count == len(t.samples) {
		t.start = (t.start + 1) % len(t.samples)
	} else {
		t.count++
	}
	t.samples[index] = sample
}

func (t *unifiedPeakTracker) peaks(now time.Time) unifiedWindowPeaks {
	if t == nil {
		return unifiedWindowPeaks{}
	}
	fiveMinutesAgo := now.Add(-unifiedPeakWindow5m).UnixNano()
	oneHourAgo := now.Add(-unifiedPeakWindow1h).UnixNano()
	t.mu.Lock()
	defer t.mu.Unlock()
	peaks := t.all
	for i := 0; i < t.count; i++ {
		sample := t.samples[(t.start+i)%len(t.samples)]
		if sample.at < oneHourAgo {
			continue
		}
		peaks.cpu1h = max(peaks.cpu1h, sample.cpu)
		peaks.gpu1h = max(peaks.gpu1h, sample.gpu)
		peaks.ane1h = max(peaks.ane1h, sample.ane)
		if sample.at >= fiveMinutesAgo {
			peaks.cpu5m = max(peaks.cpu5m, sample.cpu)
			peaks.gpu5m = max(peaks.gpu5m, sample.gpu)
			peaks.ane5m = max(peaks.ane5m, sample.ane)
		}
	}
	return peaks
}

func peakValue(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
