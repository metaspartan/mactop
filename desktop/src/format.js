export function formatPercent(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "—";
  }
  return `${value.toFixed(1)}%`;
}

export function formatGiB(bytes) {
  if (typeof bytes !== "number" || Number.isNaN(bytes)) {
    return "—";
  }
  const gib = bytes / 1024 ** 3;
  return `${gib.toFixed(1)} GiB`;
}

export function formatWatts(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "—";
  }
  return `${value.toFixed(1)} W`;
}

export function formatTemp(celsius) {
  if (typeof celsius !== "number" || Number.isNaN(celsius)) {
    return "—";
  }
  return `${celsius.toFixed(1)} °C`;
}

export function formatTimestamp(isoString) {
  if (!isoString) {
    return "—";
  }
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleTimeString();
}

export function clampPercent(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return 0;
  }
  return Math.min(100, Math.max(0, value));
}
