const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("mactop", {
  fetchMetrics: () => ipcRenderer.invoke("metrics:fetch"),
  onMetricsUpdate: (callback) => {
    const listener = (_event, data) => callback(data);
    ipcRenderer.on("metrics:update", listener);
    return () => ipcRenderer.removeListener("metrics:update", listener);
  },
  onMetricsError: (callback) => {
    const listener = (_event, message) => callback(message);
    ipcRenderer.on("metrics:error", listener);
    return () => ipcRenderer.removeListener("metrics:error", listener);
  },
});
