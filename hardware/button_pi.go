//go:build pi

package hardware

import (
	"context"
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

const (
	buttonPinName = "GPIO16"
	debounce      = 30 * time.Millisecond
	pollInterval  = 100 * time.Millisecond
)

type piButton struct {
	pin gpio.PinIO
}

func newButton() (*piButton, error) {
	pin := gpioreg.ByName(buttonPinName)
	if pin == nil {
		return nil, fmt.Errorf("%w: %s", ErrDeviceNotFound, buttonPinName)
	}
	if err := pin.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		return nil, fmt.Errorf("button: configure %s: %w", buttonPinName, err)
	}
	return &piButton{pin: pin}, nil
}

func (b *piButton) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.pin.WaitForEdge(pollInterval) {
			time.Sleep(debounce)
			if b.pin.Read() == gpio.Low {
				return nil
			}
		}
	}
}

func (b *piButton) Close() error {
	return b.pin.In(gpio.Float, gpio.NoEdge)
}
