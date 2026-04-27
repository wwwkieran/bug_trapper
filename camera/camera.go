package camera

import "image"

// Camera captures images from a video source.
type Camera interface {
	Open() error
	Capture() (image.Image, error)
	Close() error
}

// PreviewCamera is a Camera that can also show a live preview window.
type PreviewCamera interface {
	Camera
	ShowPreview() error
	UpdatePreview() bool
	StopPreview()
}
