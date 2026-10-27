// pmp_names.h - pure helpers for PMP IOReport channel names and histograms.
// No CoreFoundation here, so pmp_names_test.go can exercise them directly.
#ifndef PMP_NAMES_H
#define PMP_NAMES_H

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

// M5 Max and Ultra put PMP data in per-die groups ("PMP0", "PMP1", ...) and
// leave the bare "PMP" group empty. Older chips use "PMP".
static inline bool isPMPGroup(const char *grp) {
  if (strncmp(grp, "PMP", 3) != 0)
    return false;
  for (const char *p = grp + 3; *p; p++) {
    if (*p < '0' || *p > '9')
      return false;
  }
  return true;
}

static inline const char *skipDigits(const char *p) {
  while (*p >= '0' && *p <= '9')
    p++;
  return p;
}

// Returns the character after "ANE<digits>", or NULL when chn does not start
// with "ANE". Single-die chips name the engine "ANE" or "ANE0"; Ultra has one
// per die ("ANE0", "ANE1").
static inline const char *skipAneEngineName(const char *chn) {
  if (strncmp(chn, "ANE", 3) != 0)
    return NULL;
  return skipDigits(chn + 3);
}

static inline bool isAneChannelName(const char *chn) {
  return strstr(chn, "ANE") != NULL || strstr(chn, "ane") != NULL;
}

// ANE performance-floor request channels: "ANE-AF-BW", "ANE-DCS-BW",
// "ANE-LNK0-AF-BW" (M5 Max), "ANE0-LNK0-AF-BW", "ANE1-DCS-BW" (Ultra).
static inline bool isAneFloorChannelName(const char *chn, const char *sub) {
  if (strstr(sub, "Floor") == NULL)
    return false;
  const char *p = skipAneEngineName(chn);
  if (p == NULL)
    return false;
  if (strncmp(p, "-LNK", 4) == 0)
    p = skipDigits(p + 4);
  return strcmp(p, "-AF-BW") == 0 || strcmp(p, "-DCS-BW") == 0;
}

// Bare engine state channel: "ANE", "ANE0", "ANE1".
static inline bool isAneEngineStateChannelName(const char *chn,
                                               const char *sub) {
  const char *p = skipAneEngineName(chn);
  if (p == NULL || *p != '\0')
    return false;
  return strstr(sub, "Floor") != NULL || strstr(sub, "Fast-Die") != NULL ||
         strstr(sub, "CE") != NULL || strstr(sub, "SOC") != NULL ||
         strstr(sub, "Util") != NULL;
}

typedef enum { ANE_BW_READ, ANE_BW_WRITE, ANE_BW_COMBINED } AneBwKind;

// "ANE0 RD", "ANE L1 WR", "ANE1 L0 RD+WR".
static inline AneBwKind aneBwKind(const char *chn) {
  if (strstr(chn, "RD+WR") != NULL || strstr(chn, "RW") != NULL)
    return ANE_BW_COMBINED;
  if (strstr(chn, "RD") != NULL)
    return ANE_BW_READ;
  return ANE_BW_WRITE;
}

// isAneBwDirectionChannel accepts every channel naming aneBwKind() can
// classify, including the "ANE0 RW" spelling. The gate must not test for
// "RD" or "WR" alone: neither substring occurs in "RW", so such a channel
// would be filtered out here and aneBwKind() would never classify it.
static inline bool isAneBwDirectionChannel(const char *chn) {
  return strstr(chn, "RD+WR") != NULL || strstr(chn, "RW") != NULL ||
         strstr(chn, "RD") != NULL || strstr(chn, "WR") != NULL;
}

// CPU cluster power histograms in PMP<n> / "Energy": "PACC", "PACC0",
// "MACC1", "PACC0 SRAM". Not "AGX" (GPU).
static inline bool isPmpCpuPowerChannel(const char *sub, const char *chn) {
  if (strcmp(sub, "Energy") != 0 || strlen(chn) < 4)
    return false;
  if (strncmp(chn + 1, "ACC", 3) != 0)
    return false;
  const char *p = skipDigits(chn + 4);
  return *p == '\0' || strcmp(p, " SRAM") == 0;
}

typedef enum {
  PMP_DRAM_BW_INVALID = -1,
  PMP_DRAM_BW_READ = 0,
  PMP_DRAM_BW_WRITE = 1,
  PMP_DRAM_BW_COMBINED = 2
} PmpDramBwKind;

static inline PmpDramBwKind pmpAmccDramBwKind(const char *chn) {
  if (chn == NULL || strncmp(chn, "AMCC", 4) != 0)
    return PMP_DRAM_BW_INVALID;
  const char *direction = chn + 4;
  if (*direction >= '0' && *direction <= '9')
    direction = skipDigits(direction);
  if (strcmp(direction, " RD") == 0)
    return PMP_DRAM_BW_READ;
  if (strcmp(direction, " WR") == 0)
    return PMP_DRAM_BW_WRITE;
  if (strcmp(direction, " RD+WR") == 0 || strcmp(direction, " RW") == 0)
    return PMP_DRAM_BW_COMBINED;
  return PMP_DRAM_BW_INVALID;
}

static inline bool isAmccDcsBwChannel(const char *sub, const char *chn) {
  return sub != NULL && strcmp(sub, "DCS BW") == 0 &&
         pmpAmccDramBwKind(chn) != PMP_DRAM_BW_INVALID;
}

// Residency-weighted average of histogram bins, each read at its label
// ("12GB/s" -> 12, "2W" -> 2). With skipLowest the first bin reads as 0:
// an idle, power-gated CPU cluster sits in "2W" all the time, and reading
// it at its label would add 2 W per idle cluster.
static inline double binWeightedAverage(const double *bins,
                                        const int64_t *residency, int n,
                                        bool skipLowest) {
  int64_t total = 0;
  double weighted = 0;
  for (int i = 0; i < n; i++) {
    total += residency[i];
    if (skipLowest && i == 0)
      continue;
    weighted += bins[i] * (double)residency[i];
  }
  return total > 0 ? weighted / (double)total : 0;
}

// Per-sample table that keeps the max value per distinct group/channel, so a
// channel merged into the subscription twice is counted once, while distinct
// channels (one per die and link on Ultra) still sum.
#define PMP_KEYED_MAX 32

typedef struct {
  char grp[16];
  char chn[64];
  double value;
  int kind;
} PmpKeyedMax;

static inline void pmpKeyedMaxRecord(PmpKeyedMax *table, int *count,
                                     const char *grp, const char *chn,
                                     double value, int kind) {
  for (int i = 0; i < *count; i++) {
    if (strcmp(table[i].grp, grp) != 0 || strcmp(table[i].chn, chn) != 0)
      continue;
    if (value > table[i].value)
      table[i].value = value;
    return;
  }
  if (*count >= PMP_KEYED_MAX)
    return;
  PmpKeyedMax *e = &table[(*count)++];
  snprintf(e->grp, sizeof(e->grp), "%s", grp);
  snprintf(e->chn, sizeof(e->chn), "%s", chn);
  e->value = value;
  e->kind = kind;
}

static inline double pmpKeyedMaxSum(const PmpKeyedMax *table, int count,
                                    int kind) {
  double sum = 0;
  for (int i = 0; i < count; i++) {
    if (table[i].kind == kind)
      sum += table[i].value;
  }
  return sum;
}

#endif
