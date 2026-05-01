//go:build pi

package hardware

import (
	"fmt"

	"github.com/rpi-ws281x/rpi-ws281x-go"
)

const (
	ringGPIO       = 12
	ringLEDCount   = 12
	ringBrightness = 191
)

type piRing struct {
	dev *ws2811.WS2811
}

func newRing() (*piRing, error) {
	opt := ws2811.DefaultOptions
	opt.Channels[0].GpioPin = ringGPIO
	opt.Channels[0].LedCount = ringLEDCount
	opt.Channels[0].Brightness = ringBrightness
	opt.Channels[0].StripeType = ws2811.WS2811StripGRB

	dev, err := ws2811.MakeWS2811(&opt)
	if err != nil {
		return nil, fmt.Errorf("ring: make: %w", err)
	}
	if err := dev.Init(); err != nil {
		return nil, fmt.Errorf("ring: init: %w", err)
	}
	return &piRing{dev: dev}, nil
}

func (r *piRing) On(c Color) error {
	rgb := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
	leds := r.dev.Leds(0)
	for i := range leds {
		leds[i] = rgb
	}
	return r.dev.Render()
}

func (r *piRing) Off() error {
	leds := r.dev.Leds(0)
	for i := range leds {
		leds[i] = 0
	}
	return r.dev.Render()
}

func (r *piRing) Close() error {
	leds := r.dev.Leds(0)
	for i := range leds {
		leds[i] = 0
	}
	_ = r.dev.Render()
	r.dev.Fini()
	return nil
}
