//go:build darwin

package mgpbox

import (
	"fmt"
	"strings"

	bugst "go.bug.st/serial"
)

// enumeratePorts lists the FTDI USB-serial ports on macOS by the device-name convention
// (/dev/cu.usbserial-*). Reading the USB VID on macOS would require the enumerator's cgo
// (IOKit) path, which has no CGO_ENABLED=0 fallback and would break cross-compilation to
// darwin; GetPortsList is pure Go, so discovery here is name-based. The FT231X bridge
// presents as a usbserial node (as does the Unihedron SQM); Discover then confirms which
// one is actually an MGPBox by its streamed content. The serial is recoverable from the
// node name (/dev/cu.usbserial-<SERIAL>).
func enumeratePorts() ([]DeviceInfo, error) {
	names, err := bugst.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("mgpbox: list ports: %w", err)
	}
	var out []DeviceInfo
	for _, n := range names {
		if strings.HasPrefix(n, "/dev/cu.") && strings.Contains(n, "usbserial") {
			serial := ""
			if i := strings.Index(n, "usbserial-"); i >= 0 {
				serial = n[i+len("usbserial-"):]
			}
			out = append(out, DeviceInfo{Port: n, Serial: serial})
		}
	}
	return out, nil
}
