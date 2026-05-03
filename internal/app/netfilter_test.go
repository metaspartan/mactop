package app

import "testing"

func TestGetNativeNetworkMetricsExcludesLoopback(t *testing.T) {
	m, err := GetNativeNetworkMetrics()
	if err != nil {
		t.Fatalf("GetNativeNetworkMetrics: %v", err)
	}
	if _, ok := m["lo0"]; ok {
		t.Errorf("expected lo0 to be excluded, but it was present in metrics map")
	}
	t.Logf("interfaces returned: %d", len(m))
	for name, v := range m {
		t.Logf("%-12s recv=%d sent=%d", name, v.BytesRecv, v.BytesSent)
	}
}
