package app

import (
	"os"
	"testing"
)

func checkProcessList(t *testing.T, step string, procs []ProcessMetrics) {
	t.Helper()
	if len(procs) == 0 {
		t.Fatalf("%s: empty process list", step)
	}
	seen := make(map[int]bool, len(procs))
	for _, p := range procs {
		if p.PID <= 0 || seen[p.PID] {
			t.Fatalf("%s: invalid or duplicate pid %d", step, p.PID)
		}
		seen[p.PID] = true
	}
}

// The KERN_PROC_ALL buffer is reused across calls and its whole capacity is
// handed to sysctl. Exercise that path in both directions: a full list grows
// the buffer, a --pid query then fetches a single entry into the oversized
// buffer, and a full list after it must again be complete and valid.
func TestGetProcessListReusedBuffer(t *testing.T) {
	saved := filterPID
	t.Cleanup(func() { filterPID = saved })

	filterPID = 0
	all, err := getProcessList(0)
	if err != nil {
		t.Fatalf("full list: %v", err)
	}
	checkProcessList(t, "full list", all)

	self := os.Getpid()
	filterPID = self
	one, err := getProcessList(0)
	if err != nil {
		t.Fatalf("--pid %d: %v", self, err)
	}
	if len(one) != 1 || one[0].PID != self {
		t.Fatalf("--pid %d: got %d processes (first pid %v), want exactly self", self, len(one), firstPID(one))
	}

	filterPID = 0
	again, err := getProcessList(0)
	if err != nil {
		t.Fatalf("full list after --pid: %v", err)
	}
	checkProcessList(t, "full list after --pid", again)
	if len(again) < len(all)/2 {
		t.Fatalf("full list after --pid shrank from %d to %d processes", len(all), len(again))
	}
}

func firstPID(procs []ProcessMetrics) any {
	if len(procs) == 0 {
		return nil
	}
	return procs[0].PID
}

func BenchmarkGetProcessList(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := getProcessList(0); err != nil {
			b.Fatal(err)
		}
	}
}
