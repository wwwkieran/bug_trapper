//go:build !gocv

package camera

// NewDefaultCamera returns a CLICamera when GoCV is not available.
func NewDefaultCamera() Camera {
	return NewCLICamera()
}

// SupportsPreview reports whether this build supports live camera preview.
func SupportsPreview() bool {
	return false
}
