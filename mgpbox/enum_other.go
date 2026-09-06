//go:build !darwin

package mgpbox

import (
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

// USB ids of the MGPBox v2 bridge, as the enumerator reports them (hex strings, matched
// case-insensitively). Vendor 0x0403 is every FTDI adapter, so the PID narrows the match
// to this bridge before any port is opened; Discover still confirms the device from its
// data. A set, because an OEM can reprogram an FTDI part's PID: add the reported id here
// rather than widening back to the vendor.
const vidFTDI = "0403"

var pidsFTDI = []string{
	"6015", // FT231X — the stock bridge
}

// matchesFTDI reports whether an enumerated port is one of this instrument's bridges.
func matchesFTDI(vid, pid string) bool {
	if !strings.EqualFold(vid, vidFTDI) {
		return false
	}
	for _, want := range pidsFTDI {
		if strings.EqualFold(pid, want) {
			return true
		}
	}
	return false
}

// enumeratePorts finds candidate serial ports by USB identifiers.
func enumeratePorts() ([]DeviceInfo, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("mgpbox: enumerate ports: %w", err)
	}
	var out []DeviceInfo
	for _, p := range ports {
		if p.IsUSB && matchesFTDI(p.VID, p.PID) {
			out = append(out, DeviceInfo{Port: p.Name, Serial: p.SerialNumber, Product: p.Product})
		}
	}
	return out, nil
}
