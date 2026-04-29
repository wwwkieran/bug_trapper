//go:build pi

package hardware

import (
	"fmt"
	"log"

	"periph.io/x/host/v3"
)

func New() (*Devices, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("hardware: host init: %w", err)
	}

	d := &Devices{
		Button: noopButton{},
		Ring:   noopRing{},
		Matrix: noopMatrix{},
	}

	if b, err := newButton(); err != nil {
		log.Printf("    skipped (button not connected): %v", err)
	} else {
		d.Button = b
	}
	if r, err := newRing(); err != nil {
		log.Printf("    skipped (LED ring not connected): %v", err)
	} else {
		d.Ring = r
	}
	if m, err := newMatrix(); err != nil {
		log.Printf("    skipped (LED matrix not connected): %v", err)
	} else {
		d.Matrix = m
	}
	return d, nil
}
