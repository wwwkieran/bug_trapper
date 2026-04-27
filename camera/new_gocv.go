//go:build gocv

package camera

// NewDefaultCamera returns a GoCV camera when built with the gocv tag.
func NewDefaultCamera() Camera {
	return NewGoCV(0)
}

// SupportsPreview reports whether this build supports live camera preview.
func SupportsPreview() bool {
	return true
}
