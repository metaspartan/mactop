# mactop

[![Go Report Card](https://goreportcard.com/badge/github.com/metaspartan/mactop/v2)](https://goreportcard.com/report/github.com/metaspartan/mactop/v2)
[![GoDoc](https://godoc.org/github.com/metaspartan/mactop?status.svg)](https://godoc.org/github.com/metaspartan/mactop/v2)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/metaspartan/mactop/total) ![GitHub Release](https://img.shields.io/github/v/release/metaspartan/mactop)

`mactop` is a terminal-based monitoring tool "top" designed to display real-time metrics for Apple Silicon chips written by Carsen Klock. It provides a simple and efficient way to monitor CPU and GPU usage, E-Cores and P-Cores, power consumption, GPU frequency, temperatures, and other system metrics directly from your terminal

![mactop](mactop.gif)

## Compatibility

- Apple Silicon Only (ARM64)
- macOS Monterey 12.3+

## Features

- **No sudo required** - Uses native Apple APIs (SMC, IOReport, IOKit, IOHIDEventSystemClient)
- Apple Silicon Monitor Top written in Go Lang and CGO
- Real-time CPU, GPU, ANE, DRAM, and system power wattage usage display
- GPU frequency and usage percentage display
- CPU and GPU temperatures + Thermal State
- Detailed native metrics for CPU cores (E and P cores) via Apple's Mach Kernel API
- Memory usage and swap information
- Network usage information (upload/download speeds)
- **Thunderbolt bandwidth monitoring**: Real-time throughput for Thunderbolt Bridge interfaces
- **Thunderbolt Device Tree**: Visual tree of connected Thunderbolt/USB4 devices and their speeds
- **RDMA Support**: Detection of RDMA over Thunderbolt 5 availability
- Disk I/O activity (read/write speeds)
- Multiple volume display (shows Mac HD + mounted external volumes)
- Easy-to-read terminal UI
- **11 Layouts**: (`L` to cycle layouts)
- **Persistent Settings**: Remembers your Layout and Theme choice across restarts
- Customizable UI color (green, red, blue, skyblue, magenta, yellow, gold, silver, white, lime, orange, violet, pink, and more) (`C` to cycle colors)
- Customizable background color (`B` to cycle colors)
- Customizable update interval (default is 1000ms) (`-` or `=` to speed up, `+` to slow down)
- Process list matching htop format (VIRT in GB, CPU normalized by core count)
- **Process Management**: Kill processes directly from the UI (F9). List pauses while selecting.
- **Headless Mode**: Output JSON metrics to stdout for scripting/logging (`--headless`)
- **JSON Formatting**: Pretty print JSON output (`--pretty`) or set collection count (`--count <n>`)
- Party Mode (Randomly cycles through colors) (P to toggle)
- Optional Prometheus Metrics server (default is disabled) (`-p <port>` or `--prometheus <port>`)
- Support for all Apple Silicon models
- **Auto-detect Light/Dark Mode**: Automatically adjusts UI colors based on your terminal's background color or system theme.
- **Configurable Units**: Customize units for network, disk, and temperature display (`--unit-network`, `--unit-disk`, `--unit-temp`)

## Install via Homebrew

You can install [mactop](https://github.com/metaspartan/mactop) via Homebrew! https://brew.sh

```bash
brew install mactop
```

```bash
mactop
```

## Updating via Homebrew

```bash
brew update
```

```bash
brew upgrade mactop
```

## Installation

To install `mactop`, follow these steps:

1. Ensure you have Go installed on your machine. If not, you can install it by following the instructions here: [Go Installation Guide](https://go.dev/doc/install).

2. Clone the repository:
   ```bash
   git clone https://github.com/metaspartan/mactop.git
   cd mactop
   ```

3. Build the application:
   ```bash
   go build
   ```

4. Run the application:
   ```bash
   ./mactop
   ```

## Usage

After installation, you can start `mactop` by simply running:
```bash
./mactop
```

Example with flags:
```bash
mactop --interval 1000 --color green
```

Headless Mode (JSON Output):
```bash
# Run once and exit (great for scripts)
mactop --headless --count 1

# Run continuously with pretty printing
mactop --headless --pretty
```

## mactop Flags

- `--headless`: Run in headless mode (no TUI, output JSON to stdout).
- `--count`: Number of samples to collect in headless mode (0 = infinite).
- `--pretty`: Pretty print JSON output in headless mode.
- `--interval` or `-i`: Set the update interval in milliseconds. Default is 1000.
- `--color` or `-c`: Set the UI color. Default is white. 
Options are 'green', 'red', 'blue', 'skyblue', 'magenta', 'yellow', 'gold', 'silver', 'white', 'lime', 'orange', 'violet', etc. (-c green)
- `--prometheus` or `-p`: Set and enable the local Prometheus metrics server on the given port. Default is disabled. (e.g. -p 2112 to enable Prometheus metrics on port 2112)
- `--unit-network`: Network unit: auto, byte, kb, mb, gb (default: auto)
- `--unit-disk`: Disk unit: auto, byte, kb, mb, gb (default: auto)
- `--unit-temp`: Temperature unit: celsius, fahrenheit (default: celsius)
- `--test` or `-t`: Test IOReport power metrics (no sudo required)
- `--version` or `-v`: Print the version of mactop.
- `--help` or `-h`: Show a help message about these flags and how to run mactop.

## mactop Commands
Use the following keys to interact with the application while its running:
- `q`: Quit the application.
- `r`: Refresh the UI data manually.
- `c`: Cycle through the color themes.
- `b`: Cycle through the background colors.
- `p`: Party Mode (Randomly cycles through colors)
- `i`: Toggle Info layout (displays system info)
- `l`: Cycle through the 10 available layouts.
- `+` or `=`: Increase update interval (slower updates).
- `-`: Decrease update interval (faster updates).
- `F9`: Kill the currently selected process (pauses updates while selecting).
- `Arrow Keys` or `h/j/k/l`: Navigate the process list and select columns.
- `Enter` or `Space`: Sort by the selected column.
- `h` or `?`: Toggle the help menu.

## Example Theme (Green) Screenshot (mactop -c green) on Advanced layout (Hit "l" key to toggle)

![mactop theme](screenshota.png)

## Example Headless Output (mactop --headless --count 1)

```json
[{
  "timestamp": "2025-12-22T18:16:57-07:00",
  "soc_metrics": {
    "cpu_power": 2.1959999999999997,
    "gpu_power": 5.38552801,
    "ane_power": 0,
    "dram_power": 5.702,
    "gpu_sram_power": 0.158,
    "system_power": 37.47166092432617,
    "total_power": 50.91318893432617,
    "gpu_freq_mhz": 643,
    "soc_temp": 62.562572,
    "cpu_temp": 62.562572,
    "gpu_temp": 58.38886
  },
  "memory": {
    "total": 137438953472,
    "used": 74062512128,
    "available": 63376441344,
    "swap_total": 5368709120,
    "swap_used": 4094689280
  },
  "net_disk": {
    "out_packets_per_sec": 589.4371024829012,
    "out_bytes_per_sec": 196028.18012655922,
    "in_packets_per_sec": 593.3256375185223,
    "in_bytes_per_sec": 74461.55739710879,
    "read_ops_per_sec": 0.32404458630175986,
    "write_ops_per_sec": 40.18152870141822,
    "read_kbytes_per_sec": 1.2961783452070395,
    "write_kbytes_per_sec": 308.4904461592754
  },
  "cpu_usage": 15.140564910346725,
  "gpu_usage": 53.51970510465884,
  "core_usages": [
    42.244224422442244,
    37.17105263157895,
    55.26315789473685,
    37.704918032786885,
    27.21311475409836,
    21.241830065359476,
    14.563106796116504,
    6.148867313915858,
    4.193548387096775,
    1.9417475728155338,
    30.718954248366014,
    20.846905537459286,
    1.9417475728155338,
    1.2944983818770228,
    0.3236245954692557,
    0,
    0,
    0,
    0,
    0
  ],
  "system_info": {
    "name": "Apple M1 Ultra",
    "core_count": 20,
    "e_core_count": 4,
    "p_core_count": 16,
    "gpu_core_count": 64
  },
  "thermal_state": "Normal",
  "thunderbolt_info": {
    "buses": [
      {
        "name": "TB4 Bus 5",
        "status": "Active (USB)",
        "icon": "⏺",
        "speed": "Up to 40 Gb/s",
        "domain_uuid": "9FDBA52F-DF7C-425E-B67B-FB80F9E1DCD6",
        "switch_uid": "0x05AC38BE5E390FE5",
        "receptacle_id": "6",
        "devices": [
          {
            "name": "ASM236X NVME",
            "mode": "USB",
            "info_string": "USB, SSD"
          }
        ],
        "network_stats": {
          "interface_name": "en7",
          "bytes_in": 0,
          "bytes_out": 0,
          "bytes_in_per_sec": 0,
          "bytes_out_per_sec": 0,
          "packets_in": 0,
          "packets_out": 0
        }
      },
      {
        "name": "TB4 Bus 4",
        "status": "Inactive",
        "icon": "○",
        "speed": "Up to 40 Gb/s",
        "domain_uuid": "5DD6DE43-051D-4B32-B044-23E57AA0EEC8",
        "switch_uid": "0x05AC38BE5E390FE4",
        "receptacle_id": "5",
        "network_stats": {
          "interface_name": "en6",
          "bytes_in": 0,
          "bytes_out": 0,
          "bytes_in_per_sec": 0,
          "bytes_out_per_sec": 0,
          "packets_in": 0,
          "packets_out": 0
        }
      },
      {
        "name": "TB4 @ TB3 Bus 3",
        "status": "Active",
        "icon": "ϟ",
        "speed": "20 Gb/s",
        "domain_uuid": "AF4CE493-9005-4E9D-8B8B-0198D27BABA3",
        "switch_uid": "0x05AC38BE5E390FE3",
        "receptacle_id": "4",
        "devices": [
          {
            "name": "Studio Display",
            "vendor": "Apple Inc.",
            "vendor_id": "0x0001",
            "mode": "TB3",
            "switch_uid": "0x0001AFBD0C588A00",
            "device_id": "0x801F",
            "info_string": "Apple Inc., TB3"
          }
        ],
        "network_stats": {
          "interface_name": "en5",
          "bytes_in": 0,
          "bytes_out": 0,
          "bytes_in_per_sec": 0,
          "bytes_out_per_sec": 0,
          "packets_in": 0,
          "packets_out": 0
        }
      },
      {
        "name": "TB4 Bus 2",
        "status": "Inactive",
        "icon": "○",
        "speed": "Up to 40 Gb/s",
        "domain_uuid": "D589EEFE-98CF-42EC-A729-6C3E523AA321",
        "switch_uid": "0x05AC38BE5E390FE2",
        "receptacle_id": "3",
        "network_stats": {
          "interface_name": "en4",
          "bytes_in": 0,
          "bytes_out": 0,
          "bytes_in_per_sec": 0,
          "bytes_out_per_sec": 0,
          "packets_in": 0,
          "packets_out": 0
        }
      },
      {
        "name": "TB4 Bus 1",
        "status": "Inactive",
        "icon": "○",
        "speed": "Up to 40 Gb/s",
        "domain_uuid": "6DAC6FCD-E102-4CCD-9D01-A75540E660CA",
        "switch_uid": "0x05AC38BE5E390FE1",
        "receptacle_id": "2",
        "network_stats": {
          "interface_name": "en3",
          "bytes_in": 0,
          "bytes_out": 0,
          "bytes_in_per_sec": 0,
          "bytes_out_per_sec": 0,
          "packets_in": 0,
          "packets_out": 0
        }
      },
      {
        "name": "TB4 Bus 0",
        "status": "Inactive",
        "icon": "○",
        "speed": "Up to 40 Gb/s",
        "domain_uuid": "63A80A32-E70C-4F22-84BE-EDE632A5BA3E",
        "switch_uid": "0x05AC38BE5E390FE0",
        "receptacle_id": "1",
        "network_stats": {
          "interface_name": "en2",
          "bytes_in": 0,
          "bytes_out": 0,
          "bytes_in_per_sec": 0,
          "bytes_out_per_sec": 0,
          "packets_in": 0,
          "packets_out": 0
        }
      }
    ]
  },
  "tb_net_total_bytes_in_per_sec": 0,
  "tb_net_total_bytes_out_per_sec": 0,
  "rdma_status": {
    "available": false,
    "status": "RDMA Disabled (use rdma_ctl enable in Recovery Mode)"
  },
  "cpu_temp": 62.562572,
  "gpu_temp": 58.38886
}
]
```

## Confirmed tested working M series chips

- M1
- M1 Pro
- M1 Max
- M1 Ultra
- M2
- M2 Pro
- M2 Max
- M2 Ultra
- M3
- M3 Pro
- M3 Max
- M3 Ultra
- M4
- M4 Pro
- M4 Max
- M5

(If you have a confirmed working M series chip that is not listed, please open an issue, so we may add it here!)

## Contributing

Contributions are what make the open-source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork mactop
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## What does mactop use to get real-time data?

- **Apple SMC**: For SoC temperature sensors and System Power (PSTR)
- **IOReport API**: For CPU, GPU, ANE, and DRAM power consumption (no sudo required)
- **IOKit**: For GPU frequency table from `pmgr` device
- **IOHIDEventSystemClient**: Fallback for SoC temperature sensors
- **NSProcessInfo.thermalState**: For system thermal state (Nominal/Fair/Serious/Critical)
- **Mach Kernel API** (`host_processor_info`): For CPU metrics (E and P cores) via CGO
- **gopsutil**: For memory, swap, network, and disk I/O metrics
- **ps**: For process list information
- `sysctl`: For CPU model information
- `system_profiler`: For GPU Core Count

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Author and Contact

Carsen Klock - [@carsenklock](https://x.com/carsenklock)

Project Link: [https://github.com/metaspartan/mactop](https://github.com/metaspartan/mactop)

## Disclaimer

This tool is not officially supported by Apple. It is provided as is, and may not work as expected. Use at your own risk.

## Acknowledgements

- [gotui](https://github.com/metaspartan/gotui) for the modern terminal UI framework.
- [termui](https://github.com/gizak/termui) for the original terminal UI framework.
- [gopsutil](https://github.com/shirou/gopsutil) for system memory, network, and disk monitoring.
- [asitop](https://github.com/tlkh/asitop) for the original inspiration!
- [htop](https://github.com/htop-dev/htop) for the process list and CPU cores inspiration!
