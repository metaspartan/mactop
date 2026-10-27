// Copyright (c) 2024-2026 Carsen Klock under MIT License
// pmp_names.go - Go access to the pure PMP helpers in pmp_names.h, for tests
package app

/*
#include <stdlib.h>
#include "pmp_names.h"
*/
import "C"

import "unsafe"

func withCStrings(a, b string, fn func(a, b *C.char) C.bool) bool {
	ca, cb := C.CString(a), C.CString(b)
	defer C.free(unsafe.Pointer(ca))
	defer C.free(unsafe.Pointer(cb))
	return bool(fn(ca, cb))
}

func pmpIsGroup(grp string) bool {
	return withCStrings(grp, "", func(a, _ *C.char) C.bool { return C.isPMPGroup(a) })
}

func pmpIsAneFloorChannel(chn, sub string) bool {
	return withCStrings(chn, sub, func(a, b *C.char) C.bool { return C.isAneFloorChannelName(a, b) })
}

func pmpIsAneEngineStateChannel(chn, sub string) bool {
	return withCStrings(chn, sub, func(a, b *C.char) C.bool { return C.isAneEngineStateChannelName(a, b) })
}

func pmpIsCPUPowerChannel(sub, chn string) bool {
	return withCStrings(sub, chn, func(a, b *C.char) C.bool { return C.isPmpCpuPowerChannel(a, b) })
}

func pmpIsAmccDcsBwChannel(sub, chn string) bool {
	return withCStrings(sub, chn, func(a, b *C.char) C.bool { return C.isAmccDcsBwChannel(a, b) })
}

func pmpAmccDramBwKind(chn string) int {
	c := C.CString(chn)
	defer C.free(unsafe.Pointer(c))
	return int(C.pmpAmccDramBwKind(c))
}

func pmpAneBwKind(chn string) int {
	c := C.CString(chn)
	defer C.free(unsafe.Pointer(c))
	return int(C.aneBwKind(c))
}

func pmpIsAneBwDirectionChannel(chn string) bool {
	c := C.CString(chn)
	defer C.free(unsafe.Pointer(c))
	return bool(C.isAneBwDirectionChannel(c))
}

func pmpBinWeightedAverage(bins []float64, residency []int64, skipLowest bool) float64 {
	if len(bins) == 0 {
		return 0
	}
	return float64(C.binWeightedAverage((*C.double)(unsafe.Pointer(&bins[0])),
		(*C.int64_t)(unsafe.Pointer(&residency[0])), C.int(len(bins)), C.bool(skipLowest)))
}

type pmpRecord struct {
	grp, chn string
	value    float64
	kind     int
}

// pmpKeyedSums records each entry into one table and returns the per-kind sums.
func pmpKeyedSums(records []pmpRecord, kinds int) []float64 {
	var table [C.PMP_KEYED_MAX]C.PmpKeyedMax
	var count C.int
	for _, r := range records {
		g, c := C.CString(r.grp), C.CString(r.chn)
		C.pmpKeyedMaxRecord(&table[0], &count, g, c, C.double(r.value), C.int(r.kind))
		C.free(unsafe.Pointer(g))
		C.free(unsafe.Pointer(c))
	}
	sums := make([]float64, kinds)
	for k := range sums {
		sums[k] = float64(C.pmpKeyedMaxSum(&table[0], count, C.int(k)))
	}
	return sums
}
