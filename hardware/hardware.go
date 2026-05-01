package hardware

import (
	"context"
	"errors"
	"time"
)

var ErrDeviceNotFound = errors.New("hardware: device not found")

type Color struct{ R, G, B uint8 }

var White = Color{255, 255, 255}

type Button interface {
	Wait(ctx context.Context) error
	Close() error
}

type LEDRing interface {
	On(c Color) error
	Off() error
	Close() error
}

type Matrix interface {
	StartChase(ctx context.Context)
	Stop()
	FlashX(d time.Duration)
	FlashSmiley(d time.Duration)
	Close() error
}

type Devices struct {
	Button Button
	Ring   LEDRing
	Matrix Matrix
}

// NewNoOp returns a Devices with all no-op implementations. Use when running
// on a hardware-capable build but with peripherals intentionally disabled.
func NewNoOp() *Devices {
	return &Devices{
		Button: noopButton{},
		Ring:   noopRing{},
		Matrix: noopMatrix{},
	}
}

func (d *Devices) Close() error {
	var errs []error
	if d.Button != nil {
		if err := d.Button.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.Ring != nil {
		if err := d.Ring.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.Matrix != nil {
		if err := d.Matrix.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type noopButton struct{}

func (noopButton) Wait(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (noopButton) Close() error                   { return nil }

type noopRing struct{}

func (noopRing) On(Color) error { return nil }
func (noopRing) Off() error     { return nil }
func (noopRing) Close() error   { return nil }

type noopMatrix struct{}

func (noopMatrix) StartChase(context.Context) {}
func (noopMatrix) Stop()                      {}
func (noopMatrix) FlashX(time.Duration)       {}
func (noopMatrix) FlashSmiley(time.Duration)  {}
func (noopMatrix) Close() error               { return nil }
