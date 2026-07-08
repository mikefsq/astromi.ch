// Package mgpbox is a pure-Go driver for the Astromi.ch MGPBox / MGPBox v2 — a combined
// GPS + weather (temperature / humidity / pressure / dewpoint) + dew-heater box. The v2
// units use an FTDI FT231X USB-serial bridge (VID 0x0403, appearing as /dev/cu.usbserial-*
// or /dev/ttyUSB*) at 38400 8N1.
//
// Unlike request/response instruments (e.g. the Unihedron SQM), the MGPBox *streams*
// lines continuously once meteo/GPS reporting is enabled. This driver runs a background
// reader that parses each line and keeps the latest Meteo / Fix / Calibration snapshot,
// and exposes the small ":cmd*" command set. It speaks the protocol directly over
// go.bug.st/serial — no vendor library — and is CGO-free.
//
// Line formats accepted: the proprietary $PXDR (meteo) and $PCAL (calibration) sentences,
// and standard NMEA GPS sentences ($GPGGA/$GPGSA/$GPRMC/...). Banner and $PMTK/LOG lines
// are ignored.
//
// Note the MGPBox shares FTDI's VID 0x0403 with the Unihedron SQM; discovery tells them
// apart by content (and the differing line speed), not by VID — see Discover.
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

// DeviceInfo identifies an opened serial port plus the USB-descriptor properties the
// enumerator reports for it before the port is opened.
type DeviceInfo struct {
	Port    string // e.g. /dev/cu.usbserial-XXXX, /dev/ttyUSB0, COM3
	Serial  string // USB iSerialNumber (from the enumerator); "" if unavailable
	Product string // USB iProduct string (from the enumerator); "" if unavailable
}
