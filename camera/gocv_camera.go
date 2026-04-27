//go:build gocv

package camera

import (
	"fmt"
	"image"

	"gocv.io/x/gocv"
)

// GoCV implements Camera using OpenCV via GoCV.
type GoCV struct {
	deviceID int
	capture  *gocv.VideoCapture
	window   *gocv.Window
}

func NewGoCV(deviceID int) *GoCV {
	return &GoCV{deviceID: deviceID}
}

func (c *GoCV) Open() error {
	cap, err := gocv.OpenVideoCapture(c.deviceID)
	if err != nil {
		return fmt.Errorf("opening camera %d: %w", c.deviceID, err)
	}
	if !cap.IsOpened() {
		return fmt.Errorf("camera %d is not available", c.deviceID)
	}
	c.capture = cap
	return nil
}

func (c *GoCV) Capture() (image.Image, error) {
	if c.capture == nil {
		return nil, fmt.Errorf("camera not opened")
	}
	mat := gocv.NewMat()
	defer mat.Close()

	if ok := c.capture.Read(&mat); !ok || mat.Empty() {
		return nil, fmt.Errorf("failed to read frame from camera")
	}

	img, err := mat.ToImage()
	if err != nil {
		return nil, fmt.Errorf("converting frame to image: %w", err)
	}
	return img, nil
}

func (c *GoCV) Close() error {
	if c.window != nil {
		c.window.Close()
	}
	if c.capture != nil {
		return c.capture.Close()
	}
	return nil
}

// ShowPreview opens a window showing the live camera feed.
// This blocks and updates the preview until StopPreview is called or the window is closed.
func (c *GoCV) ShowPreview() error {
	if c.capture == nil {
		return fmt.Errorf("camera not opened")
	}
	c.window = gocv.NewWindow("Bug Trapper - Camera Preview")
	return nil
}

// UpdatePreview reads one frame and displays it. Call in a loop.
func (c *GoCV) UpdatePreview() bool {
	if c.window == nil || c.capture == nil {
		return false
	}
	mat := gocv.NewMat()
	defer mat.Close()

	if ok := c.capture.Read(&mat); !ok || mat.Empty() {
		return false
	}

	c.window.IMShow(mat)
	// WaitKey with 1ms delay — returns key code or -1
	key := c.window.WaitKey(1)
	_ = key
	return true
}

func (c *GoCV) StopPreview() {
	if c.window != nil {
		c.window.Close()
		c.window = nil
	}
}
