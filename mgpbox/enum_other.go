//go:build !darwin

package mgpbox

import (
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

// vidFTDI selects candidate bridges; Discover confirms the device from its data.
const vidFTDI = "0403"

// enumeratePorts finds candidate serial ports by USB identifiers.
func enumeratePorts() ([]DeviceInfo, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("mgpbox: enumerate ports: %w", err)
	}
	var out []DeviceInfo
	for _, p := range ports {
		if p.IsUSB && strings.EqualFold(p.VID, vidFTDI) {
			out = append(out, DeviceInfo{Port: p.Name, Serial: p.SerialNumber, Product: p.Product})
		}
	}
	return out, nil
}
