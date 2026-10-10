package app

import (
	"math"
	"testing"
)

// Channel names below are copied from IOReport on real machines:
// M1-M4 (group "PMP"), M5 Max (group "PMP0", no die index on the ANE) and
// M5 Ultra (groups "PMP0"/"PMP1", ANE0/ANE1).

func TestPMPGroup(t *testing.T) {
	cases := map[string]bool{
		"PMP":          true, // M1-M4
		"PMP0":         true, // M5 Max, Ultra die 0
		"PMP1":         true, // Ultra die 1
		"PMP10":        true,
		"PMPX":         false,
		"PMP0 ":        false,
		"Energy Model": false,
		"AMC Stats":    false,
		"":             false,
	}
	for grp, want := range cases {
		if got := pmpIsGroup(grp); got != want {
			t.Errorf("isPMPGroup(%q) = %v, want %v", grp, got, want)
		}
	}
}

func TestAneFloorChannel(t *testing.T) {
	cases := []struct {
		chn, sub string
		want     bool
	}{
		{"ANE-AF-BW", "SOC Floor", true},       // single die
		{"ANE-DCS-BW", "DCS Floor", true},      // single die
		{"ANE-LNK0-AF-BW", "SOC Floor", true},  // M5 Max
		{"ANE-LNK1-AF-BW", "SOC Floor", true},  // M5 Max
		{"ANE0-LNK0-AF-BW", "SOC Floor", true}, // Ultra
		{"ANE1-LNK1-AF-BW", "SOC Floor", true}, // Ultra
		{"ANE0-AF-BW", "SOC Floor", true},      // Ultra (present, 0 residency)
		{"ANE1-DCS-BW", "DCS Floor", true},     // Ultra
		{"ANE-AF-BW", "AF BW", false},          // not a Floor subgroup
		{"ANE0", "SOC Floor", false},           // engine state, not a floor
		{"ANE0-LNK0-AF-BWX", "SOC Floor", false},
		{"GPU-AF-BW", "SOC Floor", false},
		{"THROT-ANE-SUM", "PWRS0", false},
	}
	for _, c := range cases {
		if got := pmpIsAneFloorChannel(c.chn, c.sub); got != c.want {
			t.Errorf("isAneFloorChannelName(%q, %q) = %v, want %v", c.chn, c.sub, got, c.want)
		}
	}
}

func TestAneEngineStateChannel(t *testing.T) {
	cases := []struct {
		chn, sub string
		want     bool
	}{
		{"ANE0", "Fast-Die CE", true}, // single die and M5 Max
		{"ANE0", "SOC Floor", true},
		{"ANE1", "DCS Floor", true}, // Ultra die 1
		{"ANE", "SOC Floor", true},  // M5 Max bare engine
		{"ANE0", "AF BW", false},
		{"ANE0 L0 RD", "AF BW", false},
		{"ANE-AF-BW", "SOC Floor", false},
	}
	for _, c := range cases {
		if got := pmpIsAneEngineStateChannel(c.chn, c.sub); got != c.want {
			t.Errorf("isAneEngineStateChannelName(%q, %q) = %v, want %v", c.chn, c.sub, got, c.want)
		}
	}
}

func TestAneBwKind(t *testing.T) {
	const read, write, combined = 0, 1, 2
	cases := map[string]int{
		"ANE0 RD":       read, // single die
		"ANE0 WR":       write,
		"ANE0 RD+WR":    combined,
		"ANE L0 RD":     read, // M5 Max
		"ANE L1 WR":     write,
		"ANE1 L0 RD":    read, // Ultra
		"ANE1 L1 RD+WR": combined,
	}
	for chn, want := range cases {
		if got := pmpAneBwKind(chn); got != want {
			t.Errorf("aneBwKind(%q) = %d, want %d", chn, got, want)
		}
	}
}

// The AF BW gate must admit every channel aneBwKind can classify, or a
// combined-only chip ("ANE0 RW") is filtered out and reports 0 GB/s forever.
// Neither "RD" nor "WR" occurs in "RW", so this is a real trap.
func TestIsAneBwDirectionChannel(t *testing.T) {
	cases := map[string]bool{
		"ANE0 RD":    true,
		"ANE0 WR":    true,
		"ANE0 RD+WR": true,
		"ANE0 RW":    true,
		"ANE1 L0 RW": true,
		"ANE L1 RD":  true,
		"ANE0 CH":    false,
		"AF BW":      false,
		"":           false,
	}
	for chn, want := range cases {
		if got := pmpIsAneBwDirectionChannel(chn); got != want {
			t.Errorf("isAneBwDirectionChannel(%q) = %v, want %v", chn, got, want)
		}
	}
}

// The invariant that matters: a channel the gate rejects is never classified,
// and the combined classification is reachable through the gate.
func TestAneBwGateCoversEveryClassifiedChannel(t *testing.T) {
	const combined = 2
	names := []string{
		"ANE0 RD", "ANE0 WR", "ANE0 RD+WR", "ANE0 RW",
		"ANE L0 RD", "ANE L1 WR", "ANE1 L0 RD", "ANE1 L1 RD+WR",
	}
	for _, chn := range names {
		if !pmpIsAneBwDirectionChannel(chn) {
			t.Errorf("gate rejects %q, so its classification never runs", chn)
		}
	}
	if !pmpIsAneBwDirectionChannel("ANE0 RW") ||
		pmpAneBwKind("ANE0 RW") != combined {
		t.Error("ANE0 RW must pass the gate and classify as combined")
	}
}

func TestPmpCPUPowerChannel(t *testing.T) {
	cases := []struct {
		sub, chn string
		want     bool
	}{
		{"Energy", "PACC", true}, // M5 Max
		{"Energy", "PACC SRAM", true},
		{"Energy", "PACC0", true}, // Ultra
		{"Energy", "MACC3", true},
		{"Energy", "MACC0 SRAM", true},
		{"Energy", "AGX", false}, // GPU
		{"Energy", "AGX AFR", false},
		{"Energy", "PACC0 FOO", false},
		{"Power", "PACC0", false},
		{"Energy", "ACC", false},
	}
	for _, c := range cases {
		if got := pmpIsCPUPowerChannel(c.sub, c.chn); got != c.want {
			t.Errorf("isPmpCpuPowerChannel(%q, %q) = %v, want %v", c.sub, c.chn, got, c.want)
		}
	}
}

func TestAmccDcsBwChannel(t *testing.T) {
	cases := []struct {
		sub, chn string
		want     bool
	}{
		{"DCS BW", "AMCC RD+WR", true}, // M4 Pro/Max: preferred combined source
		{"DCS BW", "AMCC RD", true},
		{"DCS BW", "AMCC WR", true},
		{"DCS BW", "AMCC0 RD+WR", true}, // M3 Ultra die 0
		{"DCS BW", "AMCC1 RD", true},    // M3 Ultra die 1
		{"DCS BW", "AMCC1 WR", true},
		{"DCS BW", "PACC0 RD", false}, // agent histogram, not the aggregate
		{"DCS BW", "EACC0 WR", false},
		{"DCS BW", "AGX RD+WR", false},
		{"DCS BW", "AMCC", false}, // needs the trailing space + direction
		{"DCS BW", "AMCC0", false},
		{"AFC BW", "AMCC0 RD+WR", false}, // fabric, not DRAM
		{"AFI BW", "AMCC1 RD", false},
		{"AFR BW", "AMCC1 WR", false},
		{"DCS Floor", "AMCC RD", false},
		{"DRAM BW", "AMCC RD", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := pmpIsAmccDcsBwChannel(c.sub, c.chn); got != c.want {
			t.Errorf("isAmccDcsBwChannel(%q, %q) = %v, want %v", c.sub, c.chn, got, c.want)
		}
	}
}

func TestAmccDcsBwDirection(t *testing.T) {
	const rd, wr, comb = 1, 2, 3
	cases := map[string]int{
		"AMCC RD+WR":   comb, // M4 Pro/Max
		"AMCC RD":      rd,
		"AMCC WR":      wr,
		"AMCC0 RD+WR":  comb, // M3 Ultra die 0
		"AMCC0 RD":     rd,
		"AMCC1 WR":     wr,
		"AMCC1 RD+WR":  comb,
		"AMCC":         0,
		"AMCC0":        0,
		"PACC0 RD":     0,
		"AMCC0 RD+WRX": 0,
	}
	for chn, want := range cases {
		if got := pmpAmccDcsBwDirection(chn); got != want {
			t.Errorf("amccDcsBwDirection(%q) = %d, want %d", chn, got, want)
		}
	}
}

// M3 Ultra publishes one AMCC histogram per die. Distinct dies sum. A
// channel listed twice by a merged subscription keeps the max, so it is
// not counted twice. RD+WR stays a separate kind from RD and WR.
func TestM3UltraAmccDiesSum(t *testing.T) {
	const rd, wr, comb = 1, 2, 3
	sums := pmpKeyedSums([]pmpRecord{
		{"PMP0", "AMCC0 RD+WR", 300, comb},
		{"PMP1", "AMCC1 RD+WR", 260, comb},
		{"PMP0", "AMCC0 RD+WR", 100, comb},
		{"PMP0", "AMCC0 RD", 180, rd},
		{"PMP1", "AMCC1 RD", 150, rd},
		{"PMP0", "AMCC0 WR", 120, wr},
		{"PMP1", "AMCC1 WR", 110, wr},
	}, 4)
	if sums[comb] != 560 || sums[rd] != 330 || sums[wr] != 230 {
		t.Errorf("ultra dies = %v, want combined 560, read 330, write 230", sums)
	}
}

func TestBinWeightedAverage(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

	// Idle CPU cluster: all residency in the lowest "2W" bin reads as 0 W.
	if got := pmpBinWeightedAverage([]float64{2, 4, 6}, []int64{100, 0, 0}, true); got != 0 {
		t.Errorf("idle cluster = %v W, want 0", got)
	}
	// Busy cluster: 50% in 2W (read as 0), 50% in 6W -> 3 W.
	if got := pmpBinWeightedAverage([]float64{2, 4, 6}, []int64{50, 0, 50}, true); !near(got, 3) {
		t.Errorf("busy cluster = %v W, want 3", got)
	}
	// ANE bandwidth keeps the lowest bin: 50% at 2 GB/s, 50% at 12 GB/s -> 7.
	if got := pmpBinWeightedAverage([]float64{2, 12}, []int64{50, 50}, false); !near(got, 7) {
		t.Errorf("ANE bandwidth = %v GB/s, want 7", got)
	}
	// No residency (ANE powered off) -> 0.
	if got := pmpBinWeightedAverage([]float64{2, 12}, []int64{0, 0}, false); got != 0 {
		t.Errorf("unpowered = %v, want 0", got)
	}
}

func TestPmpKeyedMax(t *testing.T) {
	const read, write = 0, 1

	// Single die: one channel per direction, listed twice by a merged
	// subscription. The duplicate must not double count.
	single := pmpKeyedSums([]pmpRecord{
		{"PMP", "ANE0 RD", 10, read},
		{"PMP", "ANE0 WR", 4, write},
		{"PMP", "ANE0 RD", 8, read},
	}, 2)
	if single[read] != 10 || single[write] != 4 {
		t.Errorf("single die = %v, want [10 4]", single)
	}

	// Ultra: one channel per die and link. Distinct channels sum.
	ultra := pmpKeyedSums([]pmpRecord{
		{"PMP0", "ANE0 L0 RD", 3, read},
		{"PMP0", "ANE0 L1 RD", 3, read},
		{"PMP1", "ANE1 L0 RD", 2, read},
		{"PMP1", "ANE1 L1 RD", 2, read},
	}, 2)
	if ultra[read] != 10 || ultra[write] != 0 {
		t.Errorf("ultra = %v, want [10 0]", ultra)
	}
}
