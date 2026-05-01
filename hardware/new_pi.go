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
		log.Printf("    button init failed: %v", err)
	} else {
		d.Button = b
	}
	if r, err := newRing(); err != nil {
		log.Printf("    LED ring init failed: %v", err)
	} else {
		d.Ring = r
	}
	if m, err := newMatrix(); err != nil {
		log.Printf("    LED matrix init failed: %v", err)
	} else {
		d.Matrix = m
	}
	return d, nil
}
