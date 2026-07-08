# astromi.ch

Pure-Go drivers for [Astromi.ch](https://astromi.ch) astronomy accessories.

## `mgpbox` — MGPBox / MGPBox v2

A driver for the **MGPBox**: a combined **GPS + weather** (temperature / humidity /
pressure / dewpoint) + **dew-heater** box. The v2 units use an **FTDI FT231X** USB-serial
bridge (VID `0x0403`, `/dev/cu.usbserial-*` / `/dev/ttyUSB*`) at **38400 8N1**.

The box **streams** lines continuously, so the driver runs a background reader that keeps
the latest snapshot; accessors return it. Hardware-validated against a live MGPBox v2
(FT231X, serial `D30B0DP6`).

```go
box, err := mgpbox.Open()        // discover; or OpenPort(...), OpenBySerial("D30B0DP6")
if err != nil { log.Fatal(err) }
defer box.Close()
box.EnableMeteo()                // ensure meteo streaming is on

me, _ := box.Meteo()             // Temperature/Humidity/Pressure(hPa)/Dewpoint + dew heater
fx, _ := box.Fix()               // GPS lat/long/alt/sats/time (when it has a fix)
cal, _ := box.Calibration()      // after CalGet()
```

Commands: `EnableMeteo`/`EnableGPSFix`, `GpsOn`/`GpsOff`, `CalGet`, `RebootGps`, and
`Command(body)` for anything else — e.g. `Command("reboot")`, `Command("devicetype")`,
`Command("calreset")` (each wire command is `:body*`).

Line formats parsed: `$PXDR` (meteo; pressure in Pascal/bar normalised to hPa), `$PCAL`
(calibration + MM/MG streaming flags), and standard NMEA GPS (`$GPGGA/$GPGSA/$GPRMC`).
Banner / `$PMTK` / `LOG` lines are ignored.

### CLI — `mgpsnap`

```
go build ./cmd/mgpsnap
./mgpsnap                 # discover, enable streaming, print a snapshot
./mgpsnap -watch 2s       # poll
./mgpsnap -list           # discover MGPBox ports (probes FTDI ports for MGPBox content)
./mgpsnap -raw            # also echo every raw line
./mgpsnap -port /dev/cu.usbserial-XXXX
```

### Notes

- Shares FTDI's VID `0x0403` with the Unihedron SQM; `Discover` tells them apart by
  content (and the differing line speed — MGPBox 38400 vs SQM 115200), not by VID.
- On macOS, `/dev/cu.*` allows concurrent opens: quit any app holding the port first, or
  it will consume the stream.
- The reference implementation this was distilled from is `mikefsq/gomgpbox`.

## License

MIT — see [LICENSE](LICENSE).
