package mgpbox

import (
	"errors"
	"fmt"
	"strings"
	"time"

	bugst "go.bug.st/serial"
)

// probeTimeout bounds how long Discover listens on a candidate port for MGPBox content.
// The box streams the boot banner and, once poked with :mm,1*/:mg,1*, meteo/GPS
// sentences; a non-MGPBox serial port stays silent (or never matches) and is skipped.
const probeTimeout = 3 * time.Second

// openPort opens dev at the MGPBox line speed (38400 8N1) with blocking reads (no read
// timeout), which suits the continuous line stream and lets Close's port-close end the
// reader.
func openPort(dev string) (Transport, DeviceInfo, error) {
	port, err := bugst.Open(dev, &bugst.Mode{
		BaudRate: Baud,
		DataBits: 8,
		Parity:   bugst.NoParity,
		StopBits: bugst.OneStopBit,
	})
	if err != nil {
		return nil, DeviceInfo{}, fmt.Errorf("mgpbox: open %s: %w", dev, err)
	}
	return port, DeviceInfo{Port: dev}, nil
}

// Enumerate lists candidate MGPBox serial ports (FTDI VID on non-darwin; usbserial device
// names on macOS). These are candidates by transport only — use Discover to confirm which
// actually stream MGPBox content.
func Enumerate() ([]DeviceInfo, error) { return enumeratePorts() }

// Discover enumerates candidate ports and listens on each for MGPBox content (poking it
// with :mm,1*/:mg,1* to provoke a stream), returning the ports that identify as an
// MGPBox. This distinguishes it from other FTDI devices (e.g. a Unihedron SQM) and avoids opening an
// unrelated serial device.
func Discover() ([]DeviceInfo, error) {
	ports, err := enumeratePorts()
	if err != nil {
		return nil, err
	}
	var out []DeviceInfo
	for _, d := range ports {
		if probe(d.Port) {
			out = append(out, d)
		}
	}
	return out, nil
}

// probe opens port, pokes it, and reads for probeTimeout looking for an MGPBox marker.
func probe(port string) bool {
	p, err := bugst.Open(port, &bugst.Mode{BaudRate: Baud})
	if err != nil {
		return false // busy or unopenable
	}
	defer p.Close()
	_ = p.SetReadTimeout(200 * time.Millisecond)
	p.Write([]byte(":mm,1*"))
	p.Write([]byte(":mg,1*"))

	deadline := time.Now().Add(probeTimeout)
	var buf []byte
	tmp := make([]byte, 256)
	for time.Now().Before(deadline) {
		n, err := p.Read(tmp)
		if err != nil {
			return false
		}
		if n == 0 {
			continue
		}
		buf = append(buf, tmp[:n]...)
		if isMGPBoxMarker(string(buf)) {
			return true
		}
	}
	return false
}

// isMGPBoxMarker reports whether s contains a sentence that only an MGPBox emits.
func isMGPBoxMarker(s string) bool {
	switch {
	case strings.Contains(s, "MGPBox by Astromi.ch"),
		strings.Contains(s, "PXDR,"),
		strings.Contains(s, "PCAL,"),
		strings.Contains(s, "$GPGGA"), strings.Contains(s, "$GNGGA"),
		strings.Contains(s, "$GPRMC"), strings.Contains(s, "$GNRMC"):
		return true
	}
	return false
}

// openFirst probes candidate ports and opens the first that identifies as an MGPBox.
func openFirst() (Transport, DeviceInfo, error) {
	found, err := Discover()
	if err != nil {
		return nil, DeviceInfo{}, err
	}
	if len(found) == 0 {
		return nil, DeviceInfo{}, errors.New("mgpbox: no MGPBox found")
	}
	return openPort(found[0].Port)
}

// openBySerial opens the MGPBox whose FTDI USB-bridge serial matches serial
// (case-insensitive, trimmed), from the enumerator/device-name — a stable per-unit
// identity that survives replug/renumbering. The serial is unique, so no content probe is
// needed to disambiguate it from the Unihedron or other FTDI devices.
func openBySerial(serial string) (Transport, DeviceInfo, error) {
	want := strings.TrimSpace(strings.ToLower(serial))
	if want == "" {
		return nil, DeviceInfo{}, errors.New("mgpbox: empty serial")
	}
	ports, err := enumeratePorts()
	if err != nil {
		return nil, DeviceInfo{}, err
	}
	for _, d := range ports {
		if strings.ToLower(strings.TrimSpace(d.Serial)) == want {
			t, _, err := openPort(d.Port)
			return t, d, err
		}
	}
	return nil, DeviceInfo{}, fmt.Errorf("mgpbox: no MGPBox with serial %q found", serial)
}
