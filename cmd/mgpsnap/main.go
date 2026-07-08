// mgpsnap is a small CLI over the mgpbox driver: it opens an Astromi.ch MGPBox (by port
// or the first one discovered), enables meteo + GPS streaming, and prints the latest
// weather / GPS / calibration snapshot — once, or continuously with -watch.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mikefsq/astromi.ch/mgpbox"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mgpsnap:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		port    = flag.String("port", "", "serial port (e.g. /dev/cu.usbserial-XXXX); default: first discovered")
		list    = flag.Bool("list", false, "discover MGPBox ports and exit")
		watch   = flag.Duration("watch", 0, "reprint every interval (e.g. 2s); 0 = single shot")
		raw     = flag.Bool("raw", false, "also echo every raw line received")
		settle  = flag.Duration("settle", 3*time.Second, "time to wait for the first readings after enabling streaming")
	)
	flag.Parse()

	if *list {
		found, err := mgpbox.Discover()
		if err != nil {
			return err
		}
		for _, d := range found {
			fmt.Printf("%s\tserial=%s\tproduct=%s\n", d.Port, d.Serial, d.Product)
		}
		if len(found) == 0 {
			fmt.Fprintln(os.Stderr, "no MGPBox found")
		}
		return nil
	}

	box, err := open(*port)
	if err != nil {
		return err
	}
	defer box.Close()

	if *raw {
		box.SetLineHook(func(l string) { fmt.Fprintln(os.Stderr, "«", l) })
	}

	// Provoke the stream and ask for calibration.
	box.GpsOn()
	box.EnableMeteo()
	box.EnableGPSFix()
	box.CalGet()

	fmt.Printf("port           %s\n", box.Info().Port)
	time.Sleep(*settle)

	for {
		printSnapshot(box)
		if *watch <= 0 {
			return nil
		}
		time.Sleep(*watch)
	}
}

func open(port string) (*mgpbox.MGPBox, error) {
	if port != "" {
		return mgpbox.OpenPort(port)
	}
	return mgpbox.Open()
}

func printSnapshot(box *mgpbox.MGPBox) {
	if me, ok := box.Meteo(); ok {
		fmt.Printf("meteo          %.1f°C  %.1f%%RH  %.1f hPa  dewpoint %.1f°C  (dew offset %d%%, pwm %d)\n",
			me.Temperature, me.Humidity, me.Pressure, me.Dewpoint, me.DewOffset, me.DewPWM)
	} else {
		fmt.Println("meteo          (no data yet)")
	}
	if fx, ok := box.Fix(); ok {
		fmt.Printf("gps            lat %.6f  lon %.6f  alt %.1fm  sats %d  quality %q  %v\n",
			fx.Latitude, fx.Longitude, fx.Altitude, fx.Satellites, fx.Quality, fx.Time)
	} else {
		fmt.Println("gps            (no fix yet)")
	}
	if c, ok := box.Calibration(); ok {
		fmt.Printf("calibration    Pcal=%d Tcal=%d Hcal=%d meteo-stream=%v gps-stream=%v\n",
			c.Pcal, c.Tcal, c.Hcal, c.MeteoStreaming, c.GpsStreaming)
	}
}
