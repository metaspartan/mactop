#ifndef DRAM_BW_H
#define DRAM_BW_H

#include <math.h>
#include <stdbool.h>
#include <stdint.h>

static inline bool dramPmpFallbackNeeded(bool fallbackAllowed,
                                         int64_t readBytes,
                                         int64_t writeBytes) {
  return fallbackAllowed && readBytes == 0 && writeBytes == 0;
}

static inline bool dramPowerCalibrationNeeded(bool fallbackAllowed,
                                              bool pmpChannelsInSample,
                                              int64_t pmpReadBytes,
                                              int64_t pmpWriteBytes,
                                              int64_t pmpCombinedBytes,
                                              int64_t selectedReadBytes,
                                              int64_t selectedWriteBytes,
                                              bool trafficLikely) {
  if (!fallbackAllowed || !pmpChannelsInSample || !trafficLikely ||
      pmpReadBytes < 0 || pmpWriteBytes < 0 || pmpCombinedBytes < 0 ||
      selectedReadBytes < 0 || selectedWriteBytes < 0)
    return false;
  return pmpReadBytes == 0 && pmpWriteBytes == 0 && pmpCombinedBytes == 0 &&
         selectedReadBytes == 0 && selectedWriteBytes == 0;
}

static inline bool dramCalibrationParameters(double idlePowerW,
                                             double stressPowerW,
                                             int64_t bytesRead,
                                             double sampleSec,
                                             double *gbpsPerWatt,
                                             double *idleFloorW) {
  if (gbpsPerWatt == NULL || idleFloorW == NULL)
    return false;
  *gbpsPerWatt = 0.0;
  *idleFloorW = 0.0;
  if (!isfinite(idlePowerW) || !isfinite(stressPowerW) ||
      !isfinite(sampleSec) || idlePowerW < 0.0 || stressPowerW < 0.0 ||
      sampleSec <= 0.0 || bytesRead <= 0)
    return false;

  double deltaPowerW = stressPowerW - idlePowerW;
  double throughputGBs = (double)bytesRead / 1e9 / sampleSec;
  if (!isfinite(deltaPowerW) || !isfinite(throughputGBs) ||
      deltaPowerW <= 0.01 || throughputGBs <= 1.0)
    return false;

  double ratio = throughputGBs / deltaPowerW;
  if (!isfinite(ratio) || ratio < 5.0 || ratio > 500.0)
    return false;

  *gbpsPerWatt = ratio;
  *idleFloorW = idlePowerW * 0.9;
  return true;
}

static inline double estimateDramBandwidthGBs(double currentPowerW,
                                              double idleFloorW,
                                              double gbpsPerWatt) {
  if (!isfinite(currentPowerW) || !isfinite(idleFloorW) ||
      !isfinite(gbpsPerWatt) || currentPowerW < 0.0 || idleFloorW < 0.0 ||
      gbpsPerWatt <= 0.0)
    return 0.0;
  double activePowerW = currentPowerW - idleFloorW;
  if (activePowerW < 0.0)
    activePowerW = 0.0;
  double bandwidthGBs = activePowerW * gbpsPerWatt;
  return isfinite(bandwidthGBs) && bandwidthGBs <= 2048.0
             ? bandwidthGBs
             : 0.0;
}

#endif
