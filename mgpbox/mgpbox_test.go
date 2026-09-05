package mgpbox

import (
	"io"
	"math"
	"sync"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

func init() { now = func() time.Time { return fixedTime } }

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestParsePXDR(t *testing.T) {
	// Exact sentence captured live from an MGPBox v2 (FT231X @ 38400): pressure in Pascal
	// (unit "P", 101531.0 → 1015.31 hPa), plus a trailing extra field before the checksum.
	m := &MGPBox{}
	line := `$PXDR,P,101531.0,P,0,C,23.5,C,1,H,54.0,P,2,C,13.6,C,3,1.1*02`
	if !m.parseLine(line, fixedTime) {
		t.Fatal("parseLine PXDR returned false")
	}
	me, _ := m.Meteo()
	if !approx(me.Pressure, 1015.31) || !approx(me.Temperature, 23.5) ||
		!approx(me.Humidity, 54.0) || !approx(me.Dewpoint, 13.6) {
		t.Errorf("PXDR meteo = %+v", me)
	}
}

func TestParsePXDRBar(t *testing.T) {
	// The alternate firmware reports pressure in bar (unit "B") → ×1000 to hPa.
	m := &MGPBox{}
	if !m.parseLine(`$PXDR,P,1.0132,B,0,C,12.3,C,1,H,56.7,P,2,C,4.5,C,3*5A`, fixedTime) {
		t.Fatal("parseLine PXDR(bar) returned false")
	}
	me, _ := m.Meteo()
	if !approx(me.Pressure, 1013.2) {
		t.Errorf("PXDR bar pressure = %v, want 1013.2", me.Pressure)
	}
}

func TestParsePCAL(t *testing.T) {
	m := &MGPBox{}
	line := `$PCAL,P,100,T,-50,H,200,MM,1,MG,0*7F`
	if !m.parseLine(line, fixedTime) {
		t.Fatal("parseLine PCAL returned false")
	}
	c, ok := m.Calibration()
	if !ok || c.Pcal != 100 || c.Tcal != -50 || c.Hcal != 200 || !c.MeteoStreaming || c.GpsStreaming {
		t.Errorf("cal = %+v", c)
	}
}

func TestParseRMC(t *testing.T) {
	m := &MGPBox{}
	// Known-good NMEA sentence (valid checksum) — exercises applyNMEA + rmcTime.
	line := `$GPRMC,220516,A,5133.82,N,00042.24,W,173.8,231.8,130694,004.2,W*70`
	if !m.parseLine(line, fixedTime) {
		t.Fatal("parseLine RMC returned false")
	}
	fx, _ := m.Fix()
	if !approx(fx.Latitude, 51.5636666667) || fx.Time.Year() != 1994 || fx.Time.Month() != time.June || fx.Time.Day() != 13 {
		t.Errorf("RMC fix = %+v (time %v)", fx, fx.Time)
	}
}

func TestParseIgnored(t *testing.T) {
	m := &MGPBox{}
	for _, line := range []string{"MGPBox by Astromi.ch v2", "$PMTK001,604,3*32", "LOG something", ""} {
		if m.parseLine(line, fixedTime) {
			t.Errorf("parseLine(%q) = true, want ignored", line)
		}
	}
}

// fakeT serves canned bytes then blocks reads until Close, like a real streaming port.
type fakeT struct {
	mu      sync.Mutex
	data    []byte
	writes  [][]byte
	closed  chan struct{}
	closeCh sync.Once
}

func newFakeT(data string) *fakeT { return &fakeT{data: []byte(data), closed: make(chan struct{})} }

func (f *fakeT) Read(p []byte) (int, error) {
	f.mu.Lock()
	if len(f.data) > 0 {
		n := copy(p, f.data)
		f.data = f.data[n:]
		f.mu.Unlock()
		return n, nil
	}
	f.mu.Unlock()
	<-f.closed
	return 0, io.EOF
}

func (f *fakeT) Write(p []byte) (int, error) {
	f.mu.Lock()
	f.writes = append(f.writes, append([]byte(nil), p...))
	f.mu.Unlock()
	return len(p), nil
}

func (f *fakeT) Close() error { f.closeCh.Do(func() { close(f.closed) }); return nil }

func TestStreamingReader(t *testing.T) {
	data := `MGPBox by Astromi.ch v2
$PXDR,P,1.0132,B,0,C,12.3,C,1,H,56.7,P,2,C,4.5,C,3*5A
$GPRMC,220516,A,5133.82,N,00042.24,W,173.8,231.8,130694,004.2,W*70
`
	f := newFakeT(data)
	m := New(f, DeviceInfo{Port: "fake"})
	defer m.Close()

	// Poll until the reader has folded in the meteo + fix lines.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		me, mok := m.Meteo()
		fx, fok := m.Fix()
		if mok && fok && approx(me.Temperature, 12.3) && fx.Time.Year() == 1994 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	me, ok := m.Meteo()
	if !ok || !approx(me.Pressure, 1013.2) {
		t.Errorf("streamed meteo = %+v (ok=%v)", me, ok)
	}
}

func TestCommandEncoding(t *testing.T) {
	f := newFakeT("")
	m := New(f, DeviceInfo{Port: "fake"})
	defer m.Close()
	if err := m.EnableMeteo(); err != nil {
		t.Fatal(err)
	}
	if err := m.CalGet(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	got := []string{string(f.writes[0]), string(f.writes[1])}
	if got[0] != ":mm,1*" || got[1] != ":calget*" {
		t.Errorf("commands = %q, want [:mm,1* :calget*]", got)
	}
}
