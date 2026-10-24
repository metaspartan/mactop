package app

import "testing"

func TestQuickSettingCounterResetAndDeviceChanges(t *testing.T) {
	for _, c := range []struct {
		now, before   uint64
		elapsed, want float64
	}{
		{200, 100, 5, 20}, {50, 100, 5, 0}, {200, 100, 0, 0}, {200, 100, -1, 0},
	} {
		if got := counterRate(c.now, c.before, c.elapsed); got != c.want {
			t.Fatalf("counterRate: got %v want %v", got, c.want)
		}
	}
	netBefore := map[string]NativeNetMetric{"kept": {BytesRecv: 100}, "removed": {BytesRecv: 1000000}}
	netNow := map[string]NativeNetMetric{"kept": {BytesRecv: 200}, "added": {BytesRecv: 1000000000}}
	diskBefore := map[string]NativeDiskMetric{"kept": {ReadBytes: 1024, WriteOps: 100}, "removed": {ReadBytes: 1000000}}
	diskNow := map[string]NativeDiskMetric{"kept": {ReadBytes: 3072, WriteOps: 50}, "added": {ReadBytes: 1000000000}}
	got := netDiskRates(netNow, netBefore, diskNow, diskBefore, 2)
	if got.InBytesPerSec != 50 || got.ReadKBytesPerSec != 1 || got.WriteOpsPerSec != 0 {
		t.Fatalf("bad rates after topology/reset: %+v", got)
	}
	first := netDiskRates(netNow, nil, diskNow, nil, 2)
	if first != (NetDiskMetrics{}) {
		t.Fatalf("new baseline counted as traffic: %+v", first)
	}
	// Once the added device has a baseline, its next increments count normally.
	netNext := map[string]NativeNetMetric{"added": {BytesRecv: 1000000200}}
	if netDiskRates(netNext, netNow, nil, nil, 2).InBytesPerSec != 100 {
		t.Fatal("added device did not recover")
	}
}
