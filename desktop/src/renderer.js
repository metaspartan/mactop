const tabs = document.querySelectorAll(".tab");
const views = {
  simple: document.getElementById("view-simple"),
  advanced: document.getElementById("view-advanced"),
  complex: document.getElementById("view-complex"),
};
const statusEl = document.getElementById("status");

function formatPercent(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "—";
  }
  return `${value.toFixed(1)}%`;
}

function formatGiB(bytes) {
  if (typeof bytes !== "number" || Number.isNaN(bytes)) {
    return "—";
  }
  const gib = bytes / 1024 ** 3;
  return `${gib.toFixed(1)} GiB`;
}

function formatWatts(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "—";
  }
  return `${value.toFixed(1)} W`;
}

function formatTemp(celsius) {
  if (typeof celsius !== "number" || Number.isNaN(celsius)) {
    return "—";
  }
  return `${celsius.toFixed(1)} °C`;
}

function setStatus(message, isError = false) {
  if (!message) {
    statusEl.hidden = true;
    statusEl.textContent = "";
    statusEl.classList.remove("error");
    return;
  }

  statusEl.hidden = false;
  statusEl.textContent = message;
  statusEl.classList.toggle("error", isError);
}

function renderSimple(metrics) {
  const memory = metrics.memory || {};
  const soc = metrics.soc_metrics || {};

  document.getElementById("simple-cpu").textContent = formatPercent(metrics.cpu_usage);
  document.getElementById("simple-gpu").textContent = formatPercent(metrics.gpu_usage);
  document.getElementById("simple-memory").textContent = `${formatGiB(memory.used)} / ${formatGiB(memory.total)}`;
  document.getElementById("simple-power").textContent = formatWatts(soc.total_power);
  document.getElementById("simple-temp").textContent = formatTemp(soc.soc_temp ?? soc.cpu_temp);
  document.getElementById("simple-thermal").textContent = metrics.thermal_state || "—";
}

function renderAdvanced(metrics) {
  const soc = metrics.soc_metrics || {};
  const netDisk = metrics.net_disk || {};
  const processes = metrics.processes || [];

  document.getElementById("advanced-soc").textContent = [
    `Chip: ${metrics.system_info?.name || "—"}`,
    `CPU power: ${formatWatts(soc.cpu_power)}`,
    `GPU power: ${formatWatts(soc.gpu_power)}`,
    `ANE power: ${formatWatts(soc.ane_power)}`,
    `DRAM power: ${formatWatts(soc.dram_power)}`,
    `GPU freq: ${soc.gpu_freq_mhz || "—"} MHz`,
    `Cores tracked: ${metrics.core_usages?.length || 0}`,
  ].join("\n");

  document.getElementById("advanced-io").textContent = [
    `Net in: ${(netDisk.in_bytes_per_sec / 1024).toFixed(1)} KiB/s`,
    `Net out: ${(netDisk.out_bytes_per_sec / 1024).toFixed(1)} KiB/s`,
    `Disk read: ${netDisk.read_kbytes_per_sec?.toFixed(1) || "—"} KiB/s`,
    `Disk write: ${netDisk.write_kbytes_per_sec?.toFixed(1) || "—"} KiB/s`,
  ].join("\n");

  if (processes.length === 0) {
    document.getElementById("advanced-processes").textContent = "No process data.";
    return;
  }

  document.getElementById("advanced-processes").textContent = processes
    .slice(0, 10)
    .map((proc) => `${proc.pid}\t${formatPercent(proc.cpu)}\t${proc.command}`)
    .join("\n");
}

function renderComplex(metrics) {
  document.getElementById("complex-json").textContent = JSON.stringify(metrics, null, 2);
}

function renderMetrics(metrics) {
  if (!metrics) {
    return;
  }

  setStatus("");
  renderSimple(metrics);
  renderAdvanced(metrics);
  renderComplex(metrics);
}

function switchView(viewName) {
  tabs.forEach((tab) => {
    const isActive = tab.dataset.view === viewName;
    tab.classList.toggle("active", isActive);
    tab.setAttribute("aria-selected", String(isActive));
  });

  Object.entries(views).forEach(([name, element]) => {
    const isActive = name === viewName;
    element.classList.toggle("active", isActive);
    element.hidden = !isActive;
  });
}

tabs.forEach((tab) => {
  tab.addEventListener("click", () => switchView(tab.dataset.view));
});

if (window.mactop) {
  window.mactop.onMetricsUpdate(renderMetrics);
  window.mactop.onMetricsError((message) => {
    setStatus(`Sidecar error: ${message}`, true);
  });

  window.mactop
    .fetchMetrics()
    .then(renderMetrics)
    .catch((err) => setStatus(`Sidecar error: ${err.message}`, true));
} else {
  setStatus("Desktop bridge unavailable. Run inside Electron.", true);
}
