package mgpbox

import (
	"bufio"
	"fmt"
	"sync"
	"time"
)

// MGPBox is an opened Astromi.ch MGPBox. Once Open* returns, a background reader is
// parsing the device's line stream; the Meteo/Fix/Calibration accessors return the
// latest snapshot. Close stops the reader and releases the port.
type MGPBox struct {
	t    Transport
	info DeviceInfo

	mu    sync.Mutex
	meteo Meteo
	fix   Fix
	cal   Calibration

	onLine func(string) // optional raw-line hook (debugging)

	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

// now returns the current time; a var so tests can freeze it.
var now = time.Now

// New wraps an already-open Transport and starts the background reader. Most callers use
// Open / OpenPort; New is for a custom Transport (alternate backend, or a fake for tests).
func New(t Transport, info DeviceInfo) *MGPBox {
	m := &MGPBox{t: t, info: info, done: make(chan struct{})}
	m.wg.Add(1)
	go m.readLoop()
	return m
}

// Open finds and opens the first attached MGPBox (FTDI serial port that streams
// MGPBox content). See Discover for how candidates are identified.
func Open() (*MGPBox, error) {
	t, info, err := openFirst()
	if err != nil {
		return nil, err
	}
	return New(t, info), nil
}

// OpenPort opens the MGPBox on a specific serial port (from Enumerate/Discover).
func OpenPort(port string) (*MGPBox, error) {
	t, info, err := openPort(port)
	if err != nil {
		return nil, err
	}
	return New(t, info), nil
}

// OpenBySerial opens the MGPBox whose FTDI USB-bridge serial number matches serial
// (case-insensitive) — a stable per-unit identity that survives replug / port renumbering.
func OpenBySerial(serial string) (*MGPBox, error) {
	t, info, err := openBySerial(serial)
	if err != nil {
		return nil, err
	}
	return New(t, info), nil
}

// SetLineHook installs an optional callback invoked with every raw line received (for
// debugging / logging). Pass nil to clear.
func (m *MGPBox) SetLineHook(fn func(string)) { m.mu.Lock(); m.onLine = fn; m.mu.Unlock() }

func (m *MGPBox) Info() DeviceInfo { return m.info }

// Close stops the reader and closes the port. Safe to call more than once.
func (m *MGPBox) Close() error {
	var err error
	m.closeOnce.Do(func() {
		close(m.done)
		err = m.t.Close() // unblocks the reader's Read
		m.wg.Wait()
	})
	return err
}

// readLoop consumes newline-delimited sentences until the port closes. It uses blocking
// reads (no read timeout), so Close's port-close is what ends it.
func (m *MGPBox) readLoop() {
	defer m.wg.Done()
	sc := bufio.NewScanner(m.t)
	sc.Buffer(make([]byte, 0, 4096), 64*1024)
	for sc.Scan() {
		select {
		case <-m.done:
			return
		default:
		}
		line := sc.Text()
		m.mu.Lock()
		hook := m.onLine
		m.mu.Unlock()
		if hook != nil {
			hook(line)
		}
		m.parseLine(line, now())
	}
}

// Meteo returns the latest weather snapshot and whether any meteo field has been parsed.
func (m *MGPBox) Meteo() (Meteo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.meteo, m.meteo.Valid
}

// Fix returns the latest GPS snapshot and whether any GPS field has been parsed.
func (m *MGPBox) Fix() (Fix, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fix, m.fix.Valid
}

// Calibration returns the latest calibration snapshot (populated after CalGet) and
// whether a $PCAL sentence has been parsed.
func (m *MGPBox) Calibration() (Calibration, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cal, m.cal.Valid
}

func (m *MGPBox) send(cmd string) error {
	if _, err := m.t.Write([]byte(":" + cmd + "*")); err != nil {
		return fmt.Errorf("mgpbox: write %q: %w", cmd, err)
	}
	return nil
}

// EnableMeteo asks the box to stream meteo ($PXDR) sentences (":mm,1*").
func (m *MGPBox) EnableMeteo() error { return m.send("mm,1") }

// EnableGPSFix asks the box to stream GPS fix sentences (":mg,1*").
func (m *MGPBox) EnableGPSFix() error { return m.send("mg,1") }

// GpsOn / GpsOff power the GPS receiver.
func (m *MGPBox) GpsOn() error  { return m.send("gpson") }
func (m *MGPBox) GpsOff() error { return m.send("gpsoff") }

// CalGet requests the calibration sentence ($PCAL), which the reader folds into
// Calibration.
func (m *MGPBox) CalGet() error { return m.send("calget") }

// RebootGps restarts the GPS module.
func (m *MGPBox) RebootGps() error { return m.send("rebootGps") }

// Command mgpbox reads device status and provides diagnostic controls.
func (m *MGPBox) Command(body string) error { return m.send(body) }
