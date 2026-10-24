package app

import "testing"

func TestHistorySoCMemFollowsHistorySoC(t *testing.T) {
	idx := map[string]int{}
	for i, name := range layoutOrder {
		idx[name] = i
	}
	soc, ok1 := idx[LayoutHistorySoC]
	mem, ok2 := idx[LayoutHistorySoCMem]
	if !ok1 || !ok2 {
		t.Fatal("history_soc layouts missing from layoutOrder")
	}
	if mem != soc+1 {
		t.Fatalf("history_soc_mem index %d, want %d (immediately after history_soc)", mem, soc+1)
	}
	if len(layoutOrder) != 23 {
		t.Fatalf("layout count %d, want 23", len(layoutOrder))
	}
}

func TestHistorySoCBottomColumns(t *testing.T) {
	orig := currentConfig.DefaultLayout
	t.Cleanup(func() { currentConfig.DefaultLayout = orig })

	currentConfig.DefaultLayout = LayoutHistorySoC
	if historySoCBottomColumns() != 3 || !isHistorySoCLayout() {
		t.Fatalf("history_soc columns = %d", historySoCBottomColumns())
	}
	currentConfig.DefaultLayout = LayoutHistorySoCMem
	if historySoCBottomColumns() != 2 || !isHistorySoCLayout() {
		t.Fatalf("history_soc_mem columns = %d", historySoCBottomColumns())
	}
	currentConfig.DefaultLayout = LayoutDefault
	if historySoCBottomColumns() != 0 || isHistorySoCLayout() {
		t.Fatal("default layout is not a SoC history layout")
	}
}
