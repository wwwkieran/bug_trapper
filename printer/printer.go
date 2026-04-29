package printer

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	"github.com/google/gousb"
)

const (
	defaultVID   = 0x0485
	defaultPID   = 0x5741
	writeTimeout = 30 * time.Second
	cutFeed      = "\n\n\n\n\n"
)

var (
	cmdInit   = []byte{0x1B, 0x40}
	cmdCut    = []byte{0x1D, 0x56, 0x00}
	cmdCenter = []byte{0x1B, 0x61, 0x01} // ESC a 1 — center align
	cmdLeft   = []byte{0x1B, 0x61, 0x00} // ESC a 0 — left align
)

var ErrDeviceNotFound = errors.New("printer: USB device not found")

type Printer interface {
	Print(text string) error
	PrintImage(img image.Image) error
	Close() error
}

type USBPrinter struct {
	ctx      *gousb.Context
	dev      *gousb.Device
	ep       *gousb.OutEndpoint
	done     func()
	closeOnce sync.Once
	closeErr error
}

func NewUSBPrinter() (*USBPrinter, error) {
	ctx := gousb.NewContext()

	dev, err := ctx.OpenDeviceWithVIDPID(defaultVID, defaultPID)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("printer: open device: %w", err)
	}
	if dev == nil {
		ctx.Close()
		return nil, ErrDeviceNotFound
	}

	if err := dev.SetAutoDetach(true); err != nil {
		log.Printf("printer: set auto-detach failed (continuing): %v", err)
	}

	intf, done, err := dev.DefaultInterface()
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("printer: claim interface: %w", err)
	}

	var epNum int
	var found bool
	for _, desc := range intf.Setting.Endpoints {
		if desc.Direction == gousb.EndpointDirectionOut {
			epNum = desc.Number
			found = true
			break
		}
	}
	if !found {
		done()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("printer: no OUT endpoint found")
	}

	ep, err := intf.OutEndpoint(epNum)
	if err != nil {
		done()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("printer: open OUT endpoint: %w", err)
	}

	return &USBPrinter{ctx: ctx, dev: dev, ep: ep, done: done}, nil
}

func (p *USBPrinter) Print(text string) error {
	// TODO: ESC/POS expects CP437/CP850, not UTF-8. Non-ASCII bytes (em-dashes,
	// degree signs, etc.) will render as garbage. Strip or transcode when wiring
	// this into the full receipt.
	data := make([]byte, 0, len(cmdInit)+len(text)+len(cutFeed)+len(cmdCut))
	data = append(data, cmdInit...)
	data = append(data, []byte(text)...)
	data = append(data, []byte(cutFeed)...)
	data = append(data, cmdCut...)
	return p.write(data)
}

// PrintImage prints a single raster image (the entire receipt is one
// pre-rendered bitmap) centered on the page, then feeds and cuts the paper.
func (p *USBPrinter) PrintImage(img image.Image) error {
	raster := encodeRaster(img)

	var buf []byte
	buf = append(buf, cmdInit...)
	buf = append(buf, cmdCenter...)
	buf = append(buf, raster...)
	buf = append(buf, cmdLeft...)
	buf = append(buf, []byte(cutFeed)...)
	buf = append(buf, cmdCut...)
	return p.write(buf)
}

func (p *USBPrinter) write(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	_, err := p.ep.WriteContext(ctx, data)
	if err != nil {
		return fmt.Errorf("printer: write: %w", err)
	}
	return nil
}

func (p *USBPrinter) Close() error {
	p.closeOnce.Do(func() {
		if p.done != nil {
			p.done()
		}
		if p.dev != nil {
			if err := p.dev.Close(); err != nil {
				p.closeErr = err
			}
		}
		if p.ctx != nil {
			if err := p.ctx.Close(); err != nil && p.closeErr == nil {
				p.closeErr = err
			}
		}
	})
	return p.closeErr
}

type NoopPrinter struct{}

func (NoopPrinter) Print(string) error           { return nil }
func (NoopPrinter) PrintImage(image.Image) error { return nil }
func (NoopPrinter) Close() error                 { return nil }
