import {
  clampPercent,
  formatGiB,
  formatPercent,
  formatTemp,
  formatTimestamp,
  formatWatts,
} from "../format.js";

const THERMAL_STYLES = {
  0: "nominal",
  1: "fair",
  2: "serious",
  3: "critical",
  4: "sleeping",
};

const elements = {
  chip: document.getElementById("simple-chip"),
  cores: document.getElementById("simple-cores"),
  updated: document.getElementById("simple-updated"),
  cpuValue: document.getElementById("simple-cpu-value"),
  cpuGauge: document.getElementById("simple-cpu-gauge"),
  gpuValue: document.getElementById("simple-gpu-value"),
  gpuGauge: document.getElementById("simple-gpu-gauge"),
  memoryValue: document.getElementById("simple-memory-value"),
  memoryDetail: document.getElementById("simple-memory-detail"),
  memoryGauge: document.getElementById("simple-memory-gauge"),
  powerValue: document.getElementById("simple-power-value"),
  tempValue: document.getElementById("simple-temp-value"),
  tempDetail: document.getElementById("simple-temp-detail"),
  thermalPill: document.getElementById("simple-thermal-pill"),
};

function setGaugeFill(element, percent) {
  element.style.setProperty("--fill", `${clampPercent(percent)}%`);
}

function thermalStyleClass(level, stateText) {
  if (typeof level === "number" && THERMAL_STYLES[level]) {
    return THERMAL_STYLES[level];
  }

  const normalized = (stateText || "").toLowerCase();
  if (normalized.includes("nominal") || normalized.includes("normal")) {
    return "nominal";
  }
  if (normalized.includes("moderate") || normalized.includes("fair")) {
    return "fair";
  }
  if (normalized.includes("heavy") || normalized.includes("serious")) {
    return "serious";
  }
  if (normalized.includes("trapping") || normalized.includes("critical")) {
    return "critical";
  }
  if (normalized.includes("sleep")) {
    return "sleeping";
  }
  return "unknown";
}

export function setSimpleLoading(loading) {
  elements.updated.textContent = loading ? "Loading metrics…" : elements.updated.textContent;
}

export function renderSimple(metrics) {
  const memory = metrics.memory || {};
  const soc = metrics.soc_metrics || {};
  const systemInfo = metrics.system_info || {};

  const chipName = systemInfo.name || "Apple Silicon";
  elements.chip.textContent = chipName;

  const coreCount = systemInfo.core_count || 0;
  const gpuCores = systemInfo.gpu_core_count || 0;
  elements.cores.textContent =
    coreCount > 0 ? `${coreCount} CPU cores · ${gpuCores} GPU cores` : "—";

  elements.updated.textContent = `Updated ${formatTimestamp(metrics.timestamp)}`;

  const cpuUsage = metrics.cpu_usage;
  elements.cpuValue.textContent = formatPercent(cpuUsage);
  setGaugeFill(elements.cpuGauge, cpuUsage);

  const gpuUsage = metrics.gpu_usage;
  elements.gpuValue.textContent = formatPercent(gpuUsage);
  setGaugeFill(elements.gpuGauge, gpuUsage);

  const memUsed = memory.used;
  const memTotal = memory.total;
  elements.memoryValue.textContent = `${formatGiB(memUsed)} / ${formatGiB(memTotal)}`;
  const memPercent = memTotal > 0 ? (memUsed / memTotal) * 100 : 0;
  elements.memoryDetail.textContent = formatPercent(memPercent);
  setGaugeFill(elements.memoryGauge, memPercent);

  elements.powerValue.textContent = formatWatts(soc.total_power);

  const socTemp = soc.soc_temp;
  const cpuTemp = soc.cpu_temp;
  const hasSocTemp = typeof socTemp === "number" && !Number.isNaN(socTemp);
  const temp = hasSocTemp ? socTemp : cpuTemp;
  elements.tempValue.textContent = formatTemp(temp);
  elements.tempDetail.textContent = hasSocTemp ? "SoC" : "CPU";

  const thermalState = metrics.thermal_state || "—";
  const thermalClass = thermalStyleClass(metrics.thermal_level, thermalState);
  elements.thermalPill.textContent = thermalState;
  elements.thermalPill.className = `thermal-pill thermal-${thermalClass}`;
}
