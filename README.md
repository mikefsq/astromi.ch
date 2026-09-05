# astromi.ch

Go driver for the Astromi.ch MGPBox / MGPBox v2 weather and GPS accessory.
The `mgpbox` package reads temperature, humidity, pressure, dew point, GPS,
and calibration data over USB-serial. It has been tested with an MGPBox v2.

## Build and run

Requires Go 1.25 or later.

```sh
go build -o mgpsnap ./cmd/mgpsnap
./mgpsnap -list
./mgpsnap
./mgpsnap -watch 2s
```

The default run discovers a box, enables streaming, and prints a snapshot.
Use `-port` to select a serial port and `-raw` to show received lines.
Close other applications using the same port; concurrent readers can consume
each other's data.

## Use the library

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/mikefsq/astromi.ch/mgpbox"
)

func run() error {
    box, err := mgpbox.Open()
    if err != nil {
        return err
    }
    defer box.Close()
    if err := box.EnableMeteo(); err != nil {
        return err
    }

    deadline := time.Now().Add(5 * time.Second)
    for time.Now().Before(deadline) {
        if weather, ok := box.Meteo(); ok {
            fmt.Println(weather)
            return nil
        }
        time.Sleep(100 * time.Millisecond)
    }
    return fmt.Errorf("no weather sample received")
}

func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}
```

A background reader keeps the latest snapshots. `Meteo`, `Fix`, and
`Calibration` return a value and a boolean indicating whether a sample is
available. Check the GPS fix state before using its coordinates.
`OpenBySerial` selects a box by its FTDI bridge serial.

`EnableGPSFix`, `GpsOn`, `GpsOff`, `CalGet`, and `RebootGps` expose device
commands. `Command(body)` sends other commands using the `:body*` format.

## Protocol and testing

The port uses 38400 baud, 8N1. Discovery identifies MGPBox data rather than
relying only on the shared FTDI vendor ID. The parser handles `$PXDR`
weather, `$PCAL` calibration, and NMEA GPS sentences. Pressure is normalized
to hPa; banner and unrelated status lines are ignored.

```sh
go test -race ./...
```

Tests exercise parsing and streaming with fake transports. Applications can
supply their own `Transport` through `mgpbox.New`.

## License

[MIT](LICENSE).
