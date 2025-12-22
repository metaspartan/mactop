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
	NetworkStats  *ThunderboltNetStats   `json:"network_stats,omitempty"`
}

type ThunderboltReceptacle struct {
	Status       string `json:"receptacle_status_key"`
	CurrentSpeed string `json:"current_speed_key"`
	ReceptacleID string `json:"receptacle_id_key"`
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

// USB device types for SPUSBDataType
type USBInfo struct {
	Items []USBBus `json:"SPUSBDataType"`
}

type USBBus struct {
	Name       string      `json:"_name"`
	HostCtrl   string      `json:"host_controller"`
	PCIDev     string      `json:"pci_device"`
	PCIVendor  string      `json:"pci_vendor"`
	USBDevices []USBDevice `json:"_items"`
}

type USBDevice struct {
	Name         string `json:"_name"`
	Manufacturer string `json:"manufacturer"`
	ProductID    string `json:"product_id"`
	VendorID     string `json:"vendor_id"`
	Speed        string `json:"device_speed"`
	LocationID   string `json:"location_id"`
}

var cachedUSBInfo *USBInfo

func GetUSBInfo() (*USBInfo, error) {
	if cachedUSBInfo != nil {
		return cachedUSBInfo, nil
	}

	cmd := exec.Command("system_profiler", "-json", "SPUSBDataType")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var usbInfo USBInfo
	if err := json.Unmarshal(out, &usbInfo); err != nil {
		return nil, err
	}

	cachedUSBInfo = &usbInfo
	return &usbInfo, nil
}

// USBDeviceWithPort represents a USB device with its USB-C port number
type USBDeviceWithPort struct {
	Device     ThunderboltDeviceOutput
	PortNumber string // USB-C port number (maps to TB receptacle_id)
}

// GetUSBDevicesWithPorts returns external USB storage devices with their USB-C port numbers
func GetUSBDevicesWithPorts() []USBDeviceWithPort {
	// First, get USB device port mappings from ioreg
	portMap := getUSBDevicePortMap()

	// Get storage devices
	cmd := exec.Command("system_profiler", "-json", "SPStorageDataType")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var storageInfo struct {
		Items []struct {
			Name          string `json:"_name"`
			MountPoint    string `json:"mount_point"`
			PhysicalDrive struct {
				DeviceName string `json:"device_name"`
				IsInternal string `json:"is_internal_disk"`
				Protocol   string `json:"protocol"`
				MediumType string `json:"medium_type"`
			} `json:"physical_drive"`
		} `json:"SPStorageDataType"`
	}

	if err := json.Unmarshal(out, &storageInfo); err != nil {
		return nil
	}

	var devices []USBDeviceWithPort
	seen := make(map[string]bool)

	for _, vol := range storageInfo.Items {
		if vol.PhysicalDrive.IsInternal != "no" {
			continue
		}
		protocol := vol.PhysicalDrive.Protocol
		if protocol != "USB" && protocol != "Thunderbolt" {
			continue
		}

		devName := vol.PhysicalDrive.DeviceName
		if devName == "" {
			devName = vol.Name
		}
		if seen[devName] {
			continue
		}
		seen[devName] = true

		devInfo := protocol
		if vol.PhysicalDrive.MediumType != "" {
			devInfo += ", " + strings.ToUpper(vol.PhysicalDrive.MediumType)
		}

		// Look for port number in port map (match by device name substring)
		portNum := ""
		for key, port := range portMap {
			if strings.Contains(strings.ToLower(devName), strings.ToLower(key)) ||
				strings.Contains(strings.ToLower(key), strings.ToLower(devName)) {
				portNum = port
				break
			}
		}

		devices = append(devices, USBDeviceWithPort{
			Device: ThunderboltDeviceOutput{
				Name:   devName,
				Vendor: "",
				Mode:   protocol,
				Info:   devInfo,
			},
			PortNumber: portNum,
		})
	}
	return devices
}

// getUSBDevicePortMap returns a map of USB device names to their USB-C port numbers
func getUSBDevicePortMap() map[string]string {
	return parseIOUSBHostDeviceGrep()
}

func parseIOUSBHostDeviceGrep() map[string]string {
	portMap := make(map[string]string)

	// Use grep-like extraction
	cmd2 := exec.Command("bash", "-c", `ioreg -r -c IOUSBHostDevice 2>/dev/null | grep -E "USB Product Name|UsbCPortNumber" | paste - - 2>/dev/null`)
	out2, _ := cmd2.Output()
	lines2 := strings.Split(string(out2), "\n")
	for _, line := range lines2 {
		// Parse paired lines like: "USB Product Name" = "ASM236X series" ... "UsbCPortNumber" = 5
		if strings.Contains(line, "USB Product Name") && strings.Contains(line, "UsbCPortNumber") {
			// Extract product name
			nameStart := strings.Index(line, `"USB Product Name" = "`)
			if nameStart >= 0 {
				nameStart += len(`"USB Product Name" = "`)
				nameEnd := strings.Index(line[nameStart:], `"`)
				if nameEnd > 0 {
					prodName := line[nameStart : nameStart+nameEnd]
					// Extract port number
					portStart := strings.Index(line, `"UsbCPortNumber" = `)
					if portStart >= 0 {
						portStart += len(`"UsbCPortNumber" = `)
						portEnd := portStart
						for portEnd < len(line) && line[portEnd] >= '0' && line[portEnd] <= '9' {
							portEnd++
						}
						if portEnd > portStart {
							portMap[prodName] = line[portStart:portEnd]
						}
					}
				}
			}
		}
	}
	return portMap
}

// GetUSBDevicesForDisplay returns external storage devices (legacy function for compatibility)
func GetUSBDevicesForDisplay() []ThunderboltDeviceOutput {
	devicesWithPorts := GetUSBDevicesWithPorts()
	var devices []ThunderboltDeviceOutput
	for _, d := range devicesWithPorts {
		devices = append(devices, d.Device)
	}
	return devices
}

type ThunderboltOutput struct {
	Buses []ThunderboltBusOutput `json:"buses"`
}

type ThunderboltBusOutput struct {
	Name         string                    `json:"name"`
	Status       string                    `json:"status"` // Active, Inactive
	Icon         string                    `json:"icon"`   // ⚡, ○
	Speed        string                    `json:"speed,omitempty"`
	Devices      []ThunderboltDeviceOutput `json:"devices,omitempty"`
	NetworkStats *ThunderboltNetStats      `json:"network_stats,omitempty"`
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

	maxPortCapability := getMaxPortCapability(info.Items)
	output := &ThunderboltOutput{}
	for _, bus := range info.Items {
		output.Buses = append(output.Buses, processThunderboltBus(bus, maxPortCapability))
	}

	assignUSBDevicesToBuses(output, info.Items)

	return output, nil
}

func getMaxPortCapability(items []ThunderboltBus) string {
	maxPortCapability := "TB4" // Default for modern Macs
	for _, bus := range items {
		if bus.Receptacle != nil {
			speed := bus.Receptacle.CurrentSpeed
			if strings.Contains(speed, "120") || strings.Contains(speed, "80 Gb") {
				maxPortCapability = "TB5"
				break
			}
		}
	}
	return maxPortCapability
}

func getBusNumber(busName string) string {
	busNum := ""
	if strings.Contains(busName, "_bus_") {
		parts := strings.Split(busName, "_bus_")
		if len(parts) > 1 {
			busNum = parts[1]
		}
	}
	return busNum
}

func getBusActivityInfo(bus ThunderboltBus) (bool, string, string) {
	isActive := false
	speed := ""
	activeProtocol := ""

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
	return isActive, speed, activeProtocol
}

func formatConnectedDevices(devices []ThunderboltDevice) []ThunderboltDeviceOutput {
	var outputs []ThunderboltDeviceOutput
	for _, dev := range devices {
		devName := dev.Name
		if devName == "" {
			devName = dev.DeviceName
		}

		devInfo := ""
		if dev.Vendor != "" {
			devInfo = fmt.Sprintf("%s", dev.Vendor)
		}
		modePretty := getFormattedMode(dev.Mode)

		if modePretty != "" {
			if devInfo != "" {
				devInfo += ", " + modePretty
			} else {
				devInfo = modePretty
			}
		}

		outputs = append(outputs, ThunderboltDeviceOutput{
			Name:   devName,
			Vendor: dev.Vendor,
			Mode:   modePretty,
			Info:   devInfo,
		})
	}
	return outputs
}

func getFormattedMode(rawMode string) string {
	if rawMode == "" {
		return ""
	}
	mode := strings.ToLower(rawMode)
	mode = strings.ReplaceAll(mode, "_", " ")
	switch {
	case strings.Contains(mode, "thunderbolt 5") || strings.Contains(mode, "thunderbolt5") || strings.Contains(mode, "thunderbolt five"):
		return "TB5"
	case strings.Contains(mode, "thunderbolt 4") || strings.Contains(mode, "thunderbolt4") || strings.Contains(mode, "thunderbolt four"):
		return "TB4"
	case strings.Contains(mode, "thunderbolt 3") || strings.Contains(mode, "thunderbolt3") || strings.Contains(mode, "thunderbolt three"):
		return "TB3"
	case strings.Contains(mode, "usb4") || strings.Contains(mode, "usb 4"):
		return "USB4"
	default:
		return strings.Title(strings.ReplaceAll(rawMode, "_", " "))
	}
}

func processThunderboltBus(bus ThunderboltBus, maxPortCapability string) ThunderboltBusOutput {
	busNum := getBusNumber(bus.Name)
	isActive, speed, activeProtocol := getBusActivityInfo(bus)

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
		Name:    busLabel,
		Status:  statusStr,
		Icon:    icon,
		Speed:   speed,
		Devices: formatConnectedDevices(bus.ConnectedDevs),
	}
	return busOut
}

func assignUSBDevicesToBuses(output *ThunderboltOutput, infoItems []ThunderboltBus) {
	usbDevicesWithPorts := GetUSBDevicesWithPorts()
	for _, usbDev := range usbDevicesWithPorts {
		matched := false
		if usbDev.PortNumber != "" {
			for i := range output.Buses {
				for _, bus := range infoItems {
					busNum := getBusNumber(bus.Name)
					if strings.HasSuffix(output.Buses[i].Name, "Bus "+busNum) && bus.Receptacle != nil {
						if bus.Receptacle.ReceptacleID == usbDev.PortNumber {
							output.Buses[i].Devices = append(output.Buses[i].Devices, usbDev.Device)
							if output.Buses[i].Status == "Inactive" {
								output.Buses[i].Status = "Active (USB)"
								output.Buses[i].Icon = "⏺"
							}
							matched = true
							break
						}
					}
				}
				if matched {
					break
				}
			}
		}
		if !matched {
			for i := range output.Buses {
				if output.Buses[i].Status == "Inactive" && len(output.Buses[i].Devices) == 0 {
					output.Buses[i].Devices = append(output.Buses[i].Devices, usbDev.Device)
					output.Buses[i].Status = "Active (USB)"
					output.Buses[i].Icon = "⏺"
					break
				}
			}
		}
	}
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
