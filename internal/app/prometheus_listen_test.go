package app

import "testing"

func TestPrometheusListenAddr(t *testing.T) {
	cases := map[string]string{
		"2112":           ":2112",
		":2112":          ":2112",
		"127.0.0.1:2112": "127.0.0.1:2112",
		"[::1]:2112":     "[::1]:2112",
		"0.0.0.0:2112":   "0.0.0.0:2112",
	}
	for in, want := range cases {
		if got := prometheusListenAddr(in); got != want {
			t.Errorf("prometheusListenAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrometheusURL(t *testing.T) {
	cases := map[string]string{
		"2112":           "http://localhost:2112/metrics",
		":2112":          "http://localhost:2112/metrics",
		"127.0.0.1:2112": "http://127.0.0.1:2112/metrics",
		"[::1]:2112":     "http://[::1]:2112/metrics",
	}
	for in, want := range cases {
		if got := prometheusURL(in); got != want {
			t.Errorf("prometheusURL(%q) = %q, want %q", in, got, want)
		}
	}
}
