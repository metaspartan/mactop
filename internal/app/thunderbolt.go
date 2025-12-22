package app

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ThunderboltInfo struct {
	Items []ThunderboltBus `json:"SPThunderboltDataType"`
}

type ThunderboltBus struct {
	Name          string                 `json:"_name"`
	Vendor        string                 `json:"vendor_name_key"`
	Receptacle    *ThunderboltReceptacle `json:"receptacle_1_tag"`
	ConnectedDevs []ThunderboltDevice    `json:"_items"`
}

type ThunderboltReceptacle struct {
	Status       string `json:"receptacle_status_key"`
	CurrentSpeed string `json:"current_speed_key"`
}

type ThunderboltDevice struct {
	Name       string `json:"_name"`
	Vendor     string `json:"vendor_name_key"`
	Mode       string `json:"mode_key"`
	DeviceName string `json:"device_name_key"`
}

var cachedThunderboltInfo *ThunderboltInfo

func GetThunderboltInfo() (*ThunderboltInfo, error) {
	if cachedThunderboltInfo != nil {
		return cachedThunderboltInfo, nil
	}

	cmd := exec.Command("system_profiler", "-json", "SPThunderboltDataType")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tbInfo ThunderboltInfo
	if err := json.Unmarshal(out, &tbInfo); err != nil {
		return nil, err
	}

	cachedThunderboltInfo = &tbInfo
	return &tbInfo, nil
}

type ThunderboltOutput struct {
	Buses []ThunderboltBusOutput `json:"buses"`
}

type ThunderboltBusOutput struct {
	Name    string                    `json:"name"`
	Status  string                    `json:"status"` // Active, Inactive
	Icon    string                    `json:"icon"`   // ⚡, ○
	Speed   string                    `json:"speed,omitempty"`
	Devices []ThunderboltDeviceOutput `json:"devices,omitempty"`
}

type ThunderboltDeviceOutput struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Info   string `json:"info_string,omitempty"`
}

// GetFormattedThunderboltInfo returns a structured representation for JSON output
func GetFormattedThunderboltInfo() (*ThunderboltOutput, error) {
	info, err := GetThunderboltInfo()
	if err != nil {
		return nil, err
	}

	// First pass: detect machine's max port capability
	// Check for TB5 (120 Gb/s or 80 Gb/s) based on any port's speed
	maxPortCapability := "TB4" // Default for modern Macs
	for _, bus := range info.Items {
		if bus.Receptacle != nil {
			speed := bus.Receptacle.CurrentSpeed
			if strings.Contains(speed, "120") || strings.Contains(speed, "80 Gb") {
				maxPortCapability = "TB5"
				break
			}
		}
	}

	output := &ThunderboltOutput{}
	for _, bus := range info.Items {
		// Extract bus number from name
		busNum := ""
		if strings.Contains(bus.Name, "_bus_") {
			parts := strings.Split(bus.Name, "_bus_")
			if len(parts) > 1 {
				busNum = parts[1]
			}
		}

		isActive := false
		speed := ""
		activeProtocol := "" // The protocol the connection is actually running at

		if bus.Receptacle != nil {
			if bus.Receptacle.Status == "receptacle_connected" {
				isActive = true
			}
			if bus.Receptacle.CurrentSpeed != "" {
				speed = bus.Receptacle.CurrentSpeed
				if isActive && !strings.Contains(speed, "Up to") {
					if strings.Contains(speed, "120") || strings.Contains(speed, "80") {
						activeProtocol = "TB5"
					} else if strings.Contains(speed, "40") {
						activeProtocol = "TB4"
					} else if strings.Contains(speed, "20") {
						activeProtocol = "TB3"
					}
				}
			}
		} else if len(bus.ConnectedDevs) > 0 {
			isActive = true
		}

		// Build bus label: show capability, and if active at different protocol, show that too
		var busLabel string
		if isActive && activeProtocol != "" && activeProtocol != maxPortCapability {
			busLabel = fmt.Sprintf("%s @ %s Bus %s", maxPortCapability, activeProtocol, busNum)
		} else {
			busLabel = fmt.Sprintf("%s Bus %s", maxPortCapability, busNum)
		}

		statusStr := "Inactive"
		icon := "○"
		if isActive {
			statusStr = "Active"
			icon = "ϟ"
		}

		busOut := ThunderboltBusOutput{
			Name:   busLabel,
			Status: statusStr,
			Icon:   icon,
			Speed:  speed,
		}

		for _, dev := range bus.ConnectedDevs {
			devName := dev.Name
			if devName == "" {
				devName = dev.DeviceName
			}

			devInfo := ""
			if dev.Vendor != "" {
				devInfo = fmt.Sprintf("%s", dev.Vendor)
			}
			modePretty := ""
			if dev.Mode != "" {
				// Convert to short format: "Thunderbolt 3" -> "TB3"
				mode := strings.ToLower(dev.Mode)
				mode = strings.ReplaceAll(mode, "_", " ")
				switch {
				case strings.Contains(mode, "thunderbolt 5") || strings.Contains(mode, "thunderbolt5") || strings.Contains(mode, "thunderbolt five"):
					modePretty = "TB5"
				case strings.Contains(mode, "thunderbolt 4") || strings.Contains(mode, "thunderbolt4") || strings.Contains(mode, "thunderbolt four"):
					modePretty = "TB4"
				case strings.Contains(mode, "thunderbolt 3") || strings.Contains(mode, "thunderbolt3") || strings.Contains(mode, "thunderbolt three"):
					modePretty = "TB3"
				case strings.Contains(mode, "usb4") || strings.Contains(mode, "usb 4"):
					modePretty = "USB4"
				default:
					modePretty = strings.Title(strings.ReplaceAll(dev.Mode, "_", " "))
				}
				if devInfo != "" {
					devInfo += ", " + modePretty
				} else {
					devInfo = modePretty
				}
			}

			busOut.Devices = append(busOut.Devices, ThunderboltDeviceOutput{
				Name:   devName,
				Vendor: dev.Vendor,
				Mode:   modePretty,
				Info:   devInfo,
			})
		}
		output.Buses = append(output.Buses, busOut)
	}

	return output, nil
}

func (t *ThunderboltInfo) Description() string {
	formatted, err := GetFormattedThunderboltInfo()
	if err != nil {
		return "Error loading Thunderbolt info."
	}
	if len(formatted.Buses) == 0 {
		return "No Thunderbolt controllers found."
	}

	var sb strings.Builder
	for _, bus := range formatted.Buses {
		speedStr := ""
		if bus.Speed != "" {
			speedStr = " @ " + bus.Speed
		}
		sb.WriteString(fmt.Sprintf("%s %s (%s)%s\n", bus.Icon, bus.Name, bus.Status, speedStr))

		if len(bus.Devices) > 0 {
			for i, dev := range bus.Devices {
				prefix := "  ├─"
				if i == len(bus.Devices)-1 {
					prefix = "  └─"
				}
				if dev.Info != "" {
					sb.WriteString(fmt.Sprintf("%s %s (%s)\n", prefix, dev.Name, dev.Info))
				} else {
					sb.WriteString(fmt.Sprintf("%s %s\n", prefix, dev.Name))
				}
			}
		}
	}
	return strings.TrimSpace(sb.String())
}
