const { app, BrowserWindow, ipcMain } = require("electron");
const { spawn } = require("child_process");
const path = require("path");

const POLL_INTERVAL_MS = 1000;
const SIDECAR_ARGS = ["--headless", "--count", "1", "--pretty"];

let mainWindow = null;
let sidecarProcess = null;
let pollTimer = null;

function sidecarBinaryPath() {
  const devBinary = path.resolve(__dirname, "..", "..", "mactop");
  if (process.env.MACTOP_BINARY) {
    return process.env.MACTOP_BINARY;
  }
  return devBinary;
}

function parseHeadlessOutput(stdout) {
  const trimmed = stdout.trim();
  if (!trimmed) {
    return null;
  }

  const payload = trimmed.startsWith("[") ? trimmed : `[${trimmed}]`;
  const samples = JSON.parse(payload);
  return Array.isArray(samples) ? samples[0] : samples;
}

function fetchMetrics() {
  return new Promise((resolve, reject) => {
    const binary = sidecarBinaryPath();
    const child = spawn(binary, SIDECAR_ARGS, {
      cwd: path.resolve(__dirname, "..", ".."),
    });

    let stdout = "";
    let stderr = "";

    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });

    child.on("error", (err) => {
      reject(err);
    });

    child.on("close", (code) => {
      if (code !== 0) {
        reject(new Error(stderr || `sidecar exited with code ${code}`));
        return;
      }

      try {
        resolve(parseHeadlessOutput(stdout));
      } catch (err) {
        reject(err);
      }
    });
  });
}

function startPolling() {
  stopPolling();

  const poll = async () => {
    if (!mainWindow || mainWindow.isDestroyed()) {
      return;
    }

    try {
      const metrics = await fetchMetrics();
      mainWindow.webContents.send("metrics:update", metrics);
    } catch (err) {
      mainWindow.webContents.send("metrics:error", err.message);
    }
  };

  poll();
  pollTimer = setInterval(poll, POLL_INTERVAL_MS);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1100,
    height: 720,
    minWidth: 960,
    minHeight: 600,
    title: "mactop-desktop",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  mainWindow.loadFile(path.join(__dirname, "..", "src", "index.html"));

  mainWindow.on("closed", () => {
    mainWindow = null;
    stopPolling();
  });

  mainWindow.webContents.on("did-finish-load", () => {
    startPolling();
  });
}

ipcMain.handle("metrics:fetch", () => fetchMetrics());

app.whenReady().then(() => {
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  stopPolling();
  if (sidecarProcess) {
    sidecarProcess.kill();
    sidecarProcess = null;
  }
  if (process.platform !== "darwin") {
    app.quit();
  }
});
