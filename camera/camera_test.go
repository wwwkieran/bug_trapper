package camera

import (
	"image"
	"image/color"
	"testing"
)

// MockCamera is a test double for Camera.
type MockCamera struct {
	opened  bool
	closed  bool
	img     image.Image
	openErr error
	capErr  error
}

func (m *MockCamera) Open() error {
	if m.openErr != nil {
		return m.openErr
	}
	m.opened = true
	return nil
}

func (m *MockCamera) Capture() (image.Image, error) {
	if m.capErr != nil {
		return nil, m.capErr
	}
	return m.img, nil
}

func (m *MockCamera) Close() error {
	m.closed = true
	return nil
}

func solidImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}
	return img
}

func TestMockCameraInterface(t *testing.T) {
	// Verify MockCamera satisfies Camera interface
	var cam Camera = &MockCamera{img: solidImage(640, 480)}
	if err := cam.Open(); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	img, err := cam.Capture()
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}
	if img == nil {
		t.Error("expected non-nil image")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 640 || bounds.Dy() != 480 {
		t.Errorf("unexpected image size: %dx%d", bounds.Dx(), bounds.Dy())
	}
	if err := cam.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestMockCameraOpenError(t *testing.T) {
	cam := &MockCamera{openErr: image.ErrFormat}
	err := cam.Open()
	if err == nil {
		t.Error("expected error")
	}
}

func TestMockCameraCaptureError(t *testing.T) {
	cam := &MockCamera{capErr: image.ErrFormat}
	_, err := cam.Capture()
	if err == nil {
		t.Error("expected error")
	}
}
