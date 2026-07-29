package app

import (
	"testing"
)

func TestIsExternalBind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		bind string
		want bool
	}{
		{"127.0.0.1", false},
		{"[::1]", false},
		{"::1", false},
		{"localhost", false},
		{"*", true},
		{"0.0.0.0", true},
		{"[::]", true},
		{"192.168.1.10", true},
		{"10.0.0.5", true},
	}
	for _, tt := range tests {
		t.Run(tt.bind, func(t *testing.T) {
			if got := isExternalBind(tt.bind); got != tt.want {
				t.Fatalf("isExternalBind(%q) = %v, want %v", tt.bind, got, tt.want)
			}
		})
	}
}

func TestPortMatchesSearch(t *testing.T) {
	t.Parallel()
	p := PortMetrics{
		Port:     8080,
		Protocol: "TCP",
		Bind:     "127.0.0.1",
		PID:      1234,
		User:     "cj",
		Command:  "node",
	}
	if !portMatchesSearch(p, "8080") {
		t.Fatal("expected port match")
	}
	if !portMatchesSearch(p, "node") {
		t.Fatal("expected command match")
	}
	if !portMatchesSearch(p, "tcp") {
		t.Fatal("expected protocol match")
	}
	if portMatchesSearch(p, "nginx") {
		t.Fatal("expected no match")
	}
}

func TestPortsSummary(t *testing.T) {
	t.Parallel()
	ports := []PortMetrics{
		{Port: 22, Protocol: "TCP", Bind: "*", External: true},
		{Port: 80, Protocol: "TCP", Bind: "127.0.0.1", External: false},
		{Port: 53, Protocol: "UDP", Bind: "0.0.0.0", External: true},
	}
	total, external, tcp, udp := portsSummary(ports)
	if total != 3 || external != 2 || tcp != 2 || udp != 1 {
		t.Fatalf("portsSummary = (%d,%d,%d,%d), want (3,2,2,1)", total, external, tcp, udp)
	}
}

func TestSortPortsByPort(t *testing.T) {
	origCol := portSelectedColumn
	origRev := portSortReverse
	defer func() {
		portSelectedColumn = origCol
		portSortReverse = origRev
	}()

	portSelectedColumn = 0
	portSortReverse = false
	ports := []PortMetrics{
		{Port: 8080, Protocol: "TCP", PID: 2},
		{Port: 80, Protocol: "TCP", PID: 1},
		{Port: 443, Protocol: "TCP", PID: 3},
	}
	sortPorts(ports)
	if ports[0].Port != 80 || ports[1].Port != 443 || ports[2].Port != 8080 {
		t.Fatalf("unexpected sort order: %+v", ports)
	}
}

func TestFilterPortsExternalAndSearch(t *testing.T) {
	origExt := portsExternalOnly
	origSearch := searchText
	defer func() {
		portsExternalOnly = origExt
		searchText = origSearch
	}()

	ports := []PortMetrics{
		{Port: 22, Protocol: "TCP", Bind: "*", Command: "sshd", External: true},
		{Port: 3000, Protocol: "TCP", Bind: "127.0.0.1", Command: "node", External: false},
		{Port: 8080, Protocol: "TCP", Bind: "0.0.0.0", Command: "python", External: true},
	}

	portsExternalOnly = true
	searchText = ""
	got := filterPorts(ports)
	if len(got) != 2 {
		t.Fatalf("external filter len = %d, want 2", len(got))
	}

	searchText = "python"
	got = filterPorts(ports)
	if len(got) != 1 || got[0].Command != "python" {
		t.Fatalf("external+search filter = %+v, want python only", got)
	}

	portsExternalOnly = false
	searchText = "node"
	got = filterPorts(ports)
	if len(got) != 1 || got[0].Command != "node" {
		t.Fatalf("search filter = %+v, want node only", got)
	}
}

func TestLayoutPortsInOrder(t *testing.T) {
	found := false
	for _, layout := range layoutOrder {
		if layout == LayoutPorts {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("LayoutPorts missing from layoutOrder")
	}
}

func TestCollectListeningPortsSmoke(t *testing.T) {
	ports, err := collectListeningPorts()
	if err != nil {
		t.Fatalf("collectListeningPorts error: %v", err)
	}
	// Smoke only: the machine may have zero visible listeners for this user.
	for _, p := range ports {
		if p.Port <= 0 || p.Port > 65535 {
			t.Fatalf("invalid port: %+v", p)
		}
		if p.Protocol != "TCP" && p.Protocol != "UDP" {
			t.Fatalf("invalid protocol: %+v", p)
		}
		if p.PID <= 0 {
			t.Fatalf("invalid pid: %+v", p)
		}
		if p.Bind == "" {
			t.Fatalf("empty bind: %+v", p)
		}
	}
}
