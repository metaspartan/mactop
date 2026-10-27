package app

import (
	"encoding/json"
	"math"
	"testing"
)

func TestDramBandwidthFromBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		readBytes         int64
		writeBytes        int64
		actualDurationNs  int64
		requestedDuration int
		wantRead          float64
		wantWrite         float64
		wantCombined      float64
	}{
		{"directional bytes", 3_000_000_000, 1_000_000_000, 1_000_000_000, 1000, 3, 1, 4},
		{"actual interval wins", 6_000_000_000, 0, 2_000_000_000, 1000, 3, 0, 3},
		{"requested interval fallback", 1_000_000_000, 500_000_000, 0, 500, 2, 1, 3},
		{"zero sample", 0, 0, 0, 0, 0, 0, 0},
		{"negative counter", -1, 100, 1_000_000_000, 1000, 0, 0, 0},
		{"implausible combined rate", 1_200_000_000_000, 1_200_000_000_000, 1_000_000_000, 1000, 0, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			read, write, combined := dramBandwidthFromBytes(tc.readBytes, tc.writeBytes, tc.actualDurationNs, tc.requestedDuration)
			if read != tc.wantRead || write != tc.wantWrite || combined != tc.wantCombined {
				t.Fatalf("dramBandwidthFromBytes() = (%v, %v, %v), want (%v, %v, %v)", read, write, combined, tc.wantRead, tc.wantWrite, tc.wantCombined)
			}
		})
	}
}

func TestDramBandwidthPropagatesToMetricsAndHeadless(t *testing.T) {
	soc := SocMetrics{DRAMReadBW: 4.5, DRAMWriteBW: 1.5, DRAMBWCombined: 6}
	metrics := cpuMetricsFromSoc(soc, nil, 0, false)
	if metrics.DRAMReadBW != 4.5 || metrics.DRAMWriteBW != 1.5 || metrics.DRAMBWCombined != 6 {
		t.Fatalf("CPU metrics bandwidth = (%v, %v, %v), want (4.5, 1.5, 6)", metrics.DRAMReadBW, metrics.DRAMWriteBW, metrics.DRAMBWCombined)
	}

	encoded, err := json.Marshal(HeadlessOutput{SocMetrics: soc})
	if err != nil {
		t.Fatalf("marshal headless output: %v", err)
	}
	var decoded HeadlessOutput
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal headless output: %v", err)
	}
	if decoded.SocMetrics.DRAMReadBW != 4.5 || decoded.SocMetrics.DRAMWriteBW != 1.5 || decoded.SocMetrics.DRAMBWCombined != 6 {
		t.Fatalf("headless bandwidth = (%v, %v, %v), want (4.5, 1.5, 6)", decoded.SocMetrics.DRAMReadBW, decoded.SocMetrics.DRAMWriteBW, decoded.SocMetrics.DRAMBWCombined)
	}
}

func TestDramPmpFallbackAndCalibrationSelection(t *testing.T) {
	if !dramPmpFallbackNeeded(true, 0, 0) {
		t.Fatal("missing direct bytes should allow the PMP fallback")
	}
	if dramPmpFallbackNeeded(false, 0, 0) || dramPmpFallbackNeeded(true, 1, 0) {
		t.Fatal("PMP fallback should wait for permission and zero direct bytes")
	}
	if !dramPowerCalibrationNeeded(true, true, 0, 0, 0, 0, 0, true) {
		t.Fatal("power calibration should start after sampled PMP sources stay empty under load")
	}
	for _, tc := range []struct {
		name          string
		fallback      bool
		pmpInSample   bool
		pmpRead       int64
		pmpWrite      int64
		pmpCombined   int64
		selectedRead  int64
		selectedWrite int64
		trafficLikely bool
	}{
		{"new PMP subscription not sampled yet", true, false, 0, 0, 0, 0, 0, true},
		{"PMP returned traffic", true, true, 1, 0, 0, 0, 0, true},
		{"combined PMP traffic", true, true, 0, 0, 1, 0, 0, true},
		{"selected direct traffic", true, true, 0, 0, 0, 1, 0, true},
		{"idle", true, true, 0, 0, 0, 0, 0, false},
		{"fallback disabled", false, true, 0, 0, 0, 0, 0, true},
		{"invalid counter", true, true, -1, 0, 0, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if dramPowerCalibrationNeeded(tc.fallback, tc.pmpInSample, tc.pmpRead, tc.pmpWrite, tc.pmpCombined, tc.selectedRead, tc.selectedWrite, tc.trafficLikely) {
				t.Fatal("calibration should not start for this sample")
			}
		})
	}
}

func TestSelectPmpDramBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                           string
		legacyRead, legacyWrite        int64
		legacyCombined                 int64
		amccRead, amccWrite, amccTotal int64
		wantRead, wantWrite, wantTotal int64
	}{
		{
			name:       "legacy counters when AMCC is absent",
			legacyRead: 100, legacyWrite: 20, legacyCombined: 120,
			wantRead: 100, wantWrite: 20, wantTotal: 120,
		},
		{
			name:       "legacy counters when AMCC is idle",
			legacyRead: 100, legacyWrite: 20, legacyCombined: 120,
			amccRead: 0, amccWrite: 0, amccTotal: 0,
			wantRead: 100, wantWrite: 20, wantTotal: 120,
		},
		{
			name:       "active AMCC data takes precedence",
			legacyRead: 100, legacyWrite: 20, legacyCombined: 120,
			amccRead: 80, amccWrite: 15, amccTotal: 95,
			wantRead: 80, wantWrite: 15, wantTotal: 95,
		},
		{
			name:       "AMCC combined-only data takes precedence",
			legacyRead: 100, legacyWrite: 20, legacyCombined: 120,
			amccTotal: 95,
			wantTotal: 95,
		},
		{
			name: "all sources idle",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			read, write, combined := selectPmpDramBytes(
				tc.legacyRead, tc.legacyWrite, tc.legacyCombined,
				tc.amccRead, tc.amccWrite, tc.amccTotal,
			)
			if read != tc.wantRead || write != tc.wantWrite || combined != tc.wantTotal {
				t.Fatalf("selectPmpDramBytes() = (%d, %d, %d), want (%d, %d, %d)", read, write, combined, tc.wantRead, tc.wantWrite, tc.wantTotal)
			}
		})
	}
}

func TestDramCalibrationParameters(t *testing.T) {
	t.Parallel()

	ratio, idle, ok := deriveDramCalibration(10, 12, 20_000_000_000, 1)
	if !ok || ratio != 10 || idle != 9 {
		t.Fatalf("valid calibration = (%v, %v, %v), want (10, 9, true)", ratio, idle, ok)
	}

	cases := []struct {
		name      string
		idlePower float64
		stress    float64
		bytesRead int64
		sampleSec float64
	}{
		{"zero power", 0, 0, 20_000_000_000, 1},
		{"no power delta", 10, 10, 20_000_000_000, 1},
		{"too little traffic", 10, 12, 1_000_000_000, 1},
		{"negative power", -1, 12, 20_000_000_000, 1},
		{"non-finite stress power", 10, math.NaN(), 20_000_000_000, 1},
		{"invalid interval", 10, 12, 20_000_000_000, 0},
		{"out-of-range ratio", 10, 10.02, 20_000_000_000, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if ratio, idle, ok := deriveDramCalibration(tc.idlePower, tc.stress, tc.bytesRead, tc.sampleSec); ok || ratio != 0 || idle != 0 {
				t.Fatalf("deriveDramCalibration() = (%v, %v, %v), want (0, 0, false)", ratio, idle, ok)
			}
		})
	}
}

func TestEstimateDramBandwidthFromPower(t *testing.T) {
	t.Parallel()

	if got := estimateDramBandwidthFromPower(12, 9, 10); got != 30 {
		t.Fatalf("active estimate = %v GB/s, want 30", got)
	}
	for _, tc := range []struct {
		name    string
		current float64
		idle    float64
		ratio   float64
	}{
		{"below idle", 8, 9, 10},
		{"invalid ratio", 12, 9, 0},
		{"non-finite current", math.Inf(1), 9, 10},
		{"implausible estimate", 15, 9, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := estimateDramBandwidthFromPower(tc.current, tc.idle, tc.ratio); got != 0 {
				t.Fatalf("estimate = %v GB/s, want 0", got)
			}
		})
	}
}
