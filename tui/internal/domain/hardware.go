package domain

import (
	"regexp"
	"slices"
	"strings"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

const displayClassPrefix = "03"

var hexGroup = regexp.MustCompile(`\[([0-9a-fA-F]{4})\]`)

type PCIDevice struct {
	Slot     string
	ClassID  string
	VendorID string
}

// ParseLspci reads `lspci -vmmnn` records; lines without a tab after the tag are ignored.
func ParseLspci(text string) []PCIDevice {
	var devices []PCIDevice
	var current PCIDevice
	flush := func() {
		if current.Slot != "" {
			devices = append(devices, current)
		}
		current = PCIDevice{}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		tag, value, ok := strings.Cut(line, ":\t")
		if !ok {
			continue
		}
		switch tag {
		case "Slot":
			current.Slot = value
		case "Class":
			current.ClassID = lastHexGroup(value)
		case "Vendor":
			current.VendorID = lastHexGroup(value)
		}
	}
	flush()
	return devices
}

func lastHexGroup(s string) string {
	matches := hexGroup.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.ToLower(matches[len(matches)-1][1])
}

// ProposeDrivers returns the drivers whose vendor owns a display device; with none detected it proposes all drivers and reports false.
func ProposeDrivers(devices []PCIDevice, drivers []manifest.Driver) ([]string, bool) {
	var ids []string
	for _, d := range drivers {
		matches := slices.ContainsFunc(devices, func(dev PCIDevice) bool {
			return strings.HasPrefix(dev.ClassID, displayClassPrefix) && strings.EqualFold(dev.VendorID, d.VendorID)
		})
		if matches {
			ids = append(ids, d.ID)
		}
	}
	if len(ids) > 0 {
		return ids, true
	}
	all := make([]string, len(drivers))
	for i, d := range drivers {
		all[i] = d.ID
	}
	return all, false
}

func GPUEnvFileName(driverIDs []string) string {
	if len(driverIDs) == 0 {
		return ""
	}
	sorted := slices.Clone(driverIDs)
	slices.Sort(sorted)
	return "gpu_env_" + strings.Join(sorted, "_") + ".lua"
}
