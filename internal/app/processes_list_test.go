package app

import (
	"os"
	"testing"
)

func TestGetProcessListIncludesSelfAcrossCalls(t *testing.T) {
	// Two calls: the second one runs on the reused sysctl buffer.
	for i := 0; i < 2; i++ {
		procs, err := getProcessList(0)
		if err != nil {
			t.Fatalf("call %d: getProcessList: %v", i, err)
		}
		found := false
		for _, p := range procs {
			if p.PID == os.Getpid() {
				found = true
				break
			}
		}
		if !found && len(procs) < 500 {
			t.Fatalf("call %d: own pid %d not in %d processes", i, os.Getpid(), len(procs))
		}
	}
}

func BenchmarkGetProcessList(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := getProcessList(0); err != nil {
			b.Fatal(err)
		}
	}
}
