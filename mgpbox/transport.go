// Package mgpbox reads Astromi.ch MGPBox weather and GPS data over USB-serial.
package mgpbox

// FTDI USB vendor ID (the FT231X bridge on MGPBox v2) and the MGPBox line speed. The PID
// varies by bridge/firmware, so discovery matches the vendor (and, on macOS, the usbserial
// name) and confirms identity by the streamed content rather than a fixed PID.
const (
	VID  uint16 = 0x0403
	Baud        = 38400
)

// Transport is a byte-level serial channel to the MGPBox serial port (satisfied by a
// go.bug.st/serial port). The device logic reads newline-delimited sentences from it and
// writes ":cmd*" command strings.
type Transport interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// DeviceInfo contains serial-port discovery metadata.
// Serial identifies the USB bridge, not a protocol-level device serial.
type DeviceInfo struct {
	Port    string // e.g. /dev/cu.usbserial-XXXX, /dev/ttyUSB0, COM3
	Serial  string // USB iSerialNumber (from the enumerator); "" if unavailable
	Product string // USB iProduct string (from the enumerator); "" if unavailable
}
