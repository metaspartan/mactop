package app

import (
	"slices"
	"testing"
)

func TestLayoutAlternativeCPUInOrder(t *testing.T) {
	t.Parallel()
	index := slices.Index(layoutOrder, LayoutAlternativeCPU)
	if index < 1 || layoutOrder[index-1] != LayoutAlternative {
		t.Fatalf("LayoutAlternativeCPU index = %d, want directly after LayoutAlternative", index)
	}
	if _, ok := namedLayoutSetters[LayoutAlternativeCPU]; !ok {
		t.Fatal("LayoutAlternativeCPU has no grid setter")
	}
}

func TestFewestCoreColumns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		cols            int
		availableHeight int
		totalCores      int
		want            int
	}{
		{"10 cores, 8 rows", 4, 8, 10, 2},
		{"10 cores, 10 rows", 4, 10, 10, 1},
		{"10 cores, 3 rows", 4, 3, 10, 4},
		{"10 cores, 4 rows", 4, 4, 10, 3},
		{"24 cores, 6 rows", 8, 6, 24, 4},
		{"single column stays", 1, 2, 10, 1},
		{"zero height keeps cols", 4, 0, 10, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := fewestCoreColumns(tc.cols, tc.availableHeight, tc.totalCores); got != tc.want {
				t.Errorf("fewestCoreColumns(%d, %d, %d) = %d, want %d", tc.cols, tc.availableHeight, tc.totalCores, got, tc.want)
			}
		})
	}
}
