package app

/*
#include "dram_bw.h"
*/
import "C"

import "math"

const maxPlausibleDramBandwidthGBs = 2048.0

func dramPmpFallbackNeeded(fallbackAllowed bool, readBytes, writeBytes int64) bool {
	return bool(C.dramPmpFallbackNeeded(C.bool(fallbackAllowed), C.int64_t(readBytes), C.int64_t(writeBytes)))
}

func selectPmpDramBytes(legacyReadBytes, legacyWriteBytes, legacyCombinedBytes, amccReadBytes, amccWriteBytes, amccCombinedBytes int64) (int64, int64, int64) {
	var readBytes, writeBytes, combinedBytes C.int64_t
	C.selectPmpDramBytes(
		C.int64_t(legacyReadBytes), C.int64_t(legacyWriteBytes), C.int64_t(legacyCombinedBytes),
		C.int64_t(amccReadBytes), C.int64_t(amccWriteBytes), C.int64_t(amccCombinedBytes),
		&readBytes, &writeBytes, &combinedBytes,
	)
	return int64(readBytes), int64(writeBytes), int64(combinedBytes)
}

func dramPowerCalibrationNeeded(fallbackAllowed, pmpChannelsInSample bool, pmpReadBytes, pmpWriteBytes, pmpCombinedBytes, selectedReadBytes, selectedWriteBytes int64, trafficLikely bool) bool {
	return bool(C.dramPowerCalibrationNeeded(
		C.bool(fallbackAllowed), C.bool(pmpChannelsInSample),
		C.int64_t(pmpReadBytes), C.int64_t(pmpWriteBytes), C.int64_t(pmpCombinedBytes),
		C.int64_t(selectedReadBytes), C.int64_t(selectedWriteBytes), C.bool(trafficLikely),
	))
}

func dramBandwidthFromBytes(readBytes, writeBytes, actualDurationNs int64, requestedDurationMs int) (float64, float64, float64) {
	if readBytes < 0 || writeBytes < 0 {
		return 0, 0, 0
	}

	intervalSec := float64(actualDurationNs) / 1e9
	if actualDurationNs <= 0 {
		intervalSec = float64(requestedDurationMs) / 1000
	}
	if math.IsNaN(intervalSec) || math.IsInf(intervalSec, 0) || intervalSec <= 0 {
		return 0, 0, 0
	}

	readGBs := float64(readBytes) / intervalSec / 1e9
	writeGBs := float64(writeBytes) / intervalSec / 1e9
	combinedGBs := readGBs + writeGBs
	if math.IsNaN(combinedGBs) || math.IsInf(combinedGBs, 0) ||
		readGBs > maxPlausibleDramBandwidthGBs ||
		writeGBs > maxPlausibleDramBandwidthGBs ||
		combinedGBs > maxPlausibleDramBandwidthGBs {
		return 0, 0, 0
	}
	return readGBs, writeGBs, combinedGBs
}

func deriveDramCalibration(idlePowerW, stressPowerW float64, bytesRead int64, sampleSec float64) (float64, float64, bool) {
	var gbpsPerWatt, idleFloorW C.double
	valid := C.dramCalibrationParameters(
		C.double(idlePowerW), C.double(stressPowerW), C.int64_t(bytesRead),
		C.double(sampleSec), &gbpsPerWatt, &idleFloorW,
	)
	return float64(gbpsPerWatt), float64(idleFloorW), bool(valid)
}

func estimateDramBandwidthFromPower(currentPowerW, idleFloorW, gbpsPerWatt float64) float64 {
	return float64(C.estimateDramBandwidthGBs(
		C.double(currentPowerW), C.double(idleFloorW), C.double(gbpsPerWatt),
	))
}
