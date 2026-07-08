package mgpbox

import (
	"strconv"
	"strings"
	"time"

	nmea "github.com/adrianmo/go-nmea"
)

// Meteo is the latest weather + dew-heater snapshot.
type Meteo struct {
	Time        time.Time // when this snapshot was parsed
	Temperature float64   // °C
	Humidity    float64   // %RH
	Pressure    float64   // hPa (millibar); $PXDR reports bar and is converted here
	Dewpoint    float64   // °C
	DewOffset   int       // dew-heater target offset above dewpoint, %
	DewPWM      int       // dew-heater drive, 0–255
	Valid       bool      // set once any meteo field has been parsed
}

// Fix is the latest GPS snapshot.
type Fix struct {
	Time       time.Time // UTC time-of-fix from the GPS (RMC), when available
	Latitude   float64   // degrees, +N
	Longitude  float64   // degrees, +E
	Altitude   float64   // metres above mean sea level
	Satellites int       // satellites used in the fix (GGA)
	Quality    string    // GGA fix quality
	FixType    string    // GSA fix type (none / 2D / 3D)
	PDOP       float64
	HDOP       float64
	VDOP       float64
	Valid      bool // set once any GPS sentence has been parsed
	HasFix     bool // the receiver has an actual position fix (GGA quality≠0 / RMC valid)
}

// Calibration mirrors a $PCAL sentence: the stored pressure/temperature/humidity
// calibration offsets and the two streaming-enable flags the box reports.
type Calibration struct {
	Pcal           int
	Tcal           int
	Hcal           int
	MeteoStreaming bool // "MM": meteo sentences are being streamed
	GpsStreaming   bool // "MG": GPS fix sentences are being streamed
	Valid          bool
}

// parseLine classifies one line and folds it into the snapshots under m.mu. now is the
// wall-clock used for snapshot timestamps (injected so tests are deterministic). It
// returns true if the line updated any snapshot.
func (m *MGPBox) parseLine(line string, now time.Time) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	switch {
	case strings.Contains(line, "MGPBox by Astromi.ch"):
		return false // banner
	case strings.Contains(line, "PCAL,"):
		return m.applyPCAL(line)
	case strings.Contains(line, "PXDR,"):
		return m.applyPXDR(line, now)
	case strings.Contains(line, "PMTK"), strings.Contains(line, "LOG"):
		return false
	case line[0] == '$' || line[0] == '!':
		return m.applyNMEA(line)
	}
	return false
}

// applyPXDR parses the proprietary $PXDR transducer sentence: fixed groups of four
// ("type,value,unit,id") for pressure, temperature, humidity, dewpoint. The field
// positions follow the reference implementation. Pressure reported in bar (unit "B") is
// converted to hPa (×1000).
func (m *MGPBox) applyPXDR(line string, now time.Time) bool {
	f := strings.Split(stripChecksum(line), ",")
	if len(f) < 15 {
		return false
	}
	pressure, okP := parseF(f[2])
	if okP {
		// Normalise the pressure to hPa (millibar) using the transducer unit field.
		switch strings.ToUpper(strings.TrimSpace(f[3])) {
		case "P": // Pascal (what the FT231X firmware reports, e.g. 101531.0)
			pressure /= 100
		case "B": // bar
			pressure *= 1000
		} // else assume already hPa
	}
	temp, _ := parseF(f[6])
	hum, _ := parseF(f[10])
	dew, _ := parseF(f[14])

	m.mu.Lock()
	defer m.mu.Unlock()
	m.meteo.Time = now
	if okP {
		m.meteo.Pressure = pressure
	}
	m.meteo.Temperature = temp
	m.meteo.Humidity = hum
	m.meteo.Dewpoint = dew
	m.meteo.Valid = true
	return true
}

// applyPCAL parses the proprietary $PCAL calibration sentence. The offsets are at fixed
// positions and the MM/MG streaming flags are the first character of their value fields
// (following the reference implementation).
func (m *MGPBox) applyPCAL(line string) bool {
	f := strings.Split(stripChecksum(line), ",")
	if len(f) < 11 {
		return false
	}
	var c Calibration
	c.Pcal, _ = strconv.Atoi(strings.TrimSpace(f[2]))
	c.Tcal, _ = strconv.Atoi(strings.TrimSpace(f[4]))
	c.Hcal, _ = strconv.Atoi(strings.TrimSpace(f[6]))
	c.MeteoStreaming = firstDigit(f[8]) == '1'
	c.GpsStreaming = firstDigit(f[10]) == '1'
	c.Valid = true

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cal = c
	return true
}

// applyNMEA parses a standard NMEA GPS sentence, folding GGA/GSA/RMC into the Fix.
func (m *MGPBox) applyNMEA(line string) bool {
	s, err := nmea.Parse(line)
	if err != nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch v := s.(type) {
	case nmea.GGA:
		m.fix.Latitude = v.Latitude
		m.fix.Longitude = v.Longitude
		m.fix.Altitude = v.Altitude
		m.fix.Satellites = int(v.NumSatellites)
		m.fix.Quality = v.FixQuality
		m.fix.HasFix = v.FixQuality != "" && v.FixQuality != "0" // "0" = no fix
		m.fix.Valid = true
	case nmea.GSA:
		m.fix.FixType = v.FixType
		m.fix.PDOP = v.PDOP
		m.fix.HDOP = v.HDOP
		m.fix.VDOP = v.VDOP
		m.fix.Valid = true
	case nmea.RMC:
		m.fix.Latitude = v.Latitude
		m.fix.Longitude = v.Longitude
		if t, ok := rmcTime(v); ok {
			m.fix.Time = t
		}
		m.fix.HasFix = v.Validity == "A" // "A" = valid, "V" = void
		m.fix.Valid = true
	default:
		return false
	}
	return true
}

// rmcTime combines the RMC date and time into a UTC timestamp (zero-value ok=false when
// the sentence carries no date, e.g. before a fix).
func rmcTime(v nmea.RMC) (time.Time, bool) {
	if v.Date.YY == 0 && v.Date.MM == 0 && v.Date.DD == 0 {
		return time.Time{}, false
	}
	// Two-digit GPS year: window on 80 (GPS epoch is 1980), so 94→1994, 26→2026.
	year := 2000 + v.Date.YY
	if v.Date.YY >= 80 {
		year = 1900 + v.Date.YY
	}
	return time.Date(year, time.Month(v.Date.MM), v.Date.DD,
		v.Time.Hour, v.Time.Minute, v.Time.Second, v.Time.Millisecond*int(time.Millisecond), time.UTC), true
}

// stripChecksum drops a trailing "*hh" NMEA checksum if present.
func stripChecksum(s string) string {
	if i := strings.LastIndexByte(s, '*'); i >= 0 {
		return s[:i]
	}
	return s
}

func parseF(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil
}

// firstDigit returns the first non-space byte of a field, or 0 if empty.
func firstDigit(s string) byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return s[0]
}
