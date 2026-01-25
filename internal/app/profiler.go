package app

import (
	"fmt"
	"strconv"
	"sync"
)

type GlobalProfilerData struct {
	ThunderboltItems []ThunderboltBus `json:"SPThunderboltDataType"`
	StorageItems     []StorageItem    `json:"SPStorageDataType"`
	USBItems         []USBBus         `json:"SPUSBDataType"`
	DisplayItems     []DisplayItem    `json:"SPDisplaysDataType"`
}

type DisplayItem struct {
	Name   string `json:"_name"`
	Cores  string `json:"sppci_cores"`
	Model  string `json:"sppci_model"`
	Vendor string `json:"spdisplays_vendor"`
}

var (
	globalProfilerCache *GlobalProfilerData
	profilerMutex       sync.Mutex
)

func GetGlobalProfilerData() (*GlobalProfilerData, error) {
	profilerMutex.Lock()
	defer profilerMutex.Unlock()

	if globalProfilerCache != nil {
		return globalProfilerCache, nil
	}

	data := &GlobalProfilerData{}
	data.ThunderboltItems = buildThunderboltItemsFromIOKit()
	data.StorageItems = buildStorageItemsFromIOKit()
	data.USBItems = buildUSBItemsFromIOKit()
	data.DisplayItems = buildDisplayItemsFromIOKit()

	globalProfilerCache = data
	return globalProfilerCache, nil
}

func buildThunderboltItemsFromIOKit() []ThunderboltBus {
	switches := GetThunderboltSwitchesIOKit()
	if switches == nil {
		return nil
	}

	var buses []ThunderboltBus
	uidToBusIndex := make(map[uint64]int)

	for _, sw := range switches {
		if sw.Depth > 0 {
			continue
		}

		modeStr := determineThunderboltMode(sw)

		tbVersion := 4 // Default
		if len(modeStr) > 2 {
			if v, err := strconv.Atoi(modeStr[2:]); err == nil {
				tbVersion = v
			}
		}

		busNum := int(sw.UID & 0xF)

		bus := ThunderboltBus{
			Name:       fmt.Sprintf("TB%d Bus %d", tbVersion, busNum),
			Vendor:     sw.VendorName,
			SwitchUID:  fmt.Sprintf("0x%016X", sw.UID),
			DomainUUID: "0",
		}

		speed := "Up to 40 Gb/s"
		if tbVersion >= 5 {
			speed = "Up to 80 Gb/s"
		}
		bus.Receptacle = &ThunderboltReceptacle{
			Status:       "receptacle_no_devices_connected",
			CurrentSpeed: speed,
			ReceptacleID: strconv.Itoa(busNum + 1),
		}

		uidToBusIndex[sw.UID] = len(buses)
		buses = append(buses, bus)
	}

	for _, sw := range switches {
		if sw.Depth == 0 {
			continue
		}

		busIndex, exists := uidToBusIndex[sw.ParentUID]
		if !exists {
			continue
		}

		devMode := determineThunderboltMode(sw)

		dev := ThunderboltDevice{
			Name:      sw.DeviceName,
			Vendor:    sw.VendorName,
			VendorID:  fmt.Sprintf("0x%04X", sw.VendorID),
			DeviceID:  fmt.Sprintf("0x%04X", sw.DeviceID),
			SwitchUID: fmt.Sprintf("0x%016X", sw.UID),
			Mode:      devMode,
		}

		buses[busIndex].ConnectedDevs = append(buses[busIndex].ConnectedDevs, dev)
		buses[busIndex].Receptacle.Status = "receptacle_connected"
	}

	return buses
}

func determineThunderboltMode(sw ThunderboltSwitchInfo) string {
	// For host buses (Depth=0): Use Supported Link Speed (port capability)
	// For connected devices (Depth>0): Use Current Link Speed (negotiated speed)
	// Values from IOThunderboltPort: 14 = TB5 (80Gb/s), 12 = TB4 (40Gb/s), 8 = TB3 (20Gb/s)

	speed := sw.LinkSpeed // Default to supported speed
	if sw.Depth > 0 && sw.CurrentSpeed > 0 {
		speed = sw.CurrentSpeed // Use negotiated speed for connected devices
	}

	if speed >= 14 {
		return "TB5"
	} else if speed >= 12 {
		return "TB4"
	} else if speed > 0 {
		return "TB3"
	}
	return "TB4" // Default when no speed data available
}

func buildStorageItemsFromIOKit() []StorageItem {
	ioDevices := GetStorageDevicesIOKit()
	if ioDevices == nil {
		return nil
	}

	var items []StorageItem
	for _, d := range ioDevices {
		if !d.IsWhole {
			continue
		}
		item := StorageItem{
			Name: d.Name,
		}
		item.PhysicalDrive.DeviceName = d.Name
		item.PhysicalDrive.Protocol = d.Protocol
		item.PhysicalDrive.MediumType = d.MediumType
		if d.IsInternal {
			item.PhysicalDrive.IsInternal = "yes"
		} else {
			item.PhysicalDrive.IsInternal = "no"
		}
		items = append(items, item)
	}
	return items
}

func buildUSBItemsFromIOKit() []USBBus {
	devices := GetUSBDevicesIOKit()
	if devices == nil {
		return nil
	}

	bus := USBBus{
		Name: "USB",
	}

	for _, d := range devices {
		if d.ProductName == "" {
			continue
		}
		dev := USBDevice{
			Name:         d.ProductName,
			Manufacturer: d.VendorName,
			ProductID:    fmt.Sprintf("0x%04x", d.ProductID),
			VendorID:     fmt.Sprintf("0x%04x", d.VendorID),
			LocationID:   fmt.Sprintf("0x%08x", d.LocationID),
		}
		bus.USBDevices = append(bus.USBDevices, dev)
	}

	if len(bus.USBDevices) > 0 {
		return []USBBus{bus}
	}
	return nil
}

func buildDisplayItemsFromIOKit() []DisplayItem {
	cores := GetGPUCoreCountFast()
	if cores <= 0 {
		return nil
	}

	return []DisplayItem{
		{
			Name:  "Apple GPU",
			Cores: strconv.Itoa(cores),
		},
	}
}
