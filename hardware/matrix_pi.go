//go:build pi

package hardware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/max7219"
)

const (
	matrixSPIName   = "SPI1.0"
	matrixIntensity = 4
	chaseTickPeriod = 50 * time.Millisecond
)

// edgePath traces one lap of the 8x8 perimeter as (col, row) pairs.
// 28 cells: top row L→R, right col T→B, bottom row R→L, left col B→T.
var edgePath = func() [][2]int {
	pts := make([][2]int, 0, 28)
	for c := 0; c < 8; c++ {
		pts = append(pts, [2]int{c, 0})
	}
	for r := 1; r < 8; r++ {
		pts = append(pts, [2]int{7, r})
	}
	for c := 6; c >= 0; c-- {
		pts = append(pts, [2]int{c, 7})
	}
	for r := 6; r >= 1; r-- {
		pts = append(pts, [2]int{0, r})
	}
	return pts
}()

var xBitmap = [8]byte{
	0b10000001,
	0b01000010,
	0b00100100,
	0b00011000,
	0b00011000,
	0b00100100,
	0b01000010,
	0b10000001,
}

type piMatrix struct {
	port spi.PortCloser
	dev  *max7219.Dev

	mu      sync.Mutex
	stopCh  chan struct{}
	running bool
}

func newMatrix() (*piMatrix, error) {
	port, err := spireg.Open(matrixSPIName)
	if err != nil {
		return nil, fmt.Errorf("matrix: open %s: %w", matrixSPIName, err)
	}
	dev, err := max7219.NewSPI(port, 1, 8)
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("matrix: new: %w", err)
	}
	if err := dev.SetDecode(max7219.DecodeNone); err != nil {
		port.Close()
		return nil, fmt.Errorf("matrix: set decode: %w", err)
	}
	if err := dev.SetIntensity(matrixIntensity); err != nil {
		port.Close()
		return nil, fmt.Errorf("matrix: set intensity: %w", err)
	}
	if err := dev.Clear(); err != nil {
		port.Close()
		return nil, fmt.Errorf("matrix: clear: %w", err)
	}
	return &piMatrix{port: port, dev: dev}, nil
}

func (m *piMatrix) StartChase(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	m.stopCh = stop
	m.running = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.running = false
			m.stopCh = nil
			m.mu.Unlock()
			_ = m.dev.Clear()
		}()

		ticker := time.NewTicker(chaseTickPeriod)
		defer ticker.Stop()

		idx := 0
		var frame [8]byte
		for {
			for i := range frame {
				frame[i] = 0
			}
			c, r := edgePath[idx][0], edgePath[idx][1]
			frame[r] = 1 << uint(7-c)
			_ = m.dev.Write(frame[:])
			idx = (idx + 1) % len(edgePath)

			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (m *piMatrix) Stop() {
	m.mu.Lock()
	stop := m.stopCh
	m.mu.Unlock()
	if stop == nil {
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
	for {
		m.mu.Lock()
		running := m.running
		m.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (m *piMatrix) FlashX(d time.Duration) {
	m.Stop()
	_ = m.dev.Write(xBitmap[:])
	time.Sleep(d)
	_ = m.dev.Clear()
}

func (m *piMatrix) Close() error {
	m.Stop()
	_ = m.dev.Clear()
	return m.port.Close()
}
