//go:build !darwin

package mgpbox

import (
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

// vidFTDI is the MGPBox v2 bridge's USB vendor ID (FTDI), as the enumerator reports it (a
// hex string; matched case-insensitively). The FT231X's PID varies, so we match the vendor
// and let Discover confirm identity by content.
const vidFTDI = "0403"

// enumeratePorts lists FTDI (VID 0x0403) USB serial ports via go.bug.st/serial's
// enumerator and reports USB VID/PID.
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
