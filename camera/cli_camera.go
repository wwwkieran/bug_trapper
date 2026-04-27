//go:build !gocv

package camera

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CLICamera captures images using platform-specific CLI tools.
// macOS: imagesnap (brew install imagesnap)
// Linux: fswebcam (sudo apt install fswebcam)
type CLICamera struct {
	tmpDir string
}

func NewCLICamera() *CLICamera {
	return &CLICamera{}
}

func (c *CLICamera) Open() error {
	tool := captureTool()
	if _, err := exec.LookPath(tool); err != nil {
		switch runtime.GOOS {
		case "darwin":
			return fmt.Errorf("%s not found; install with: brew install imagesnap", tool)
		case "linux":
			return fmt.Errorf("%s not found; install with: sudo apt install fswebcam", tool)
		default:
			return fmt.Errorf("%s not found", tool)
		}
	}
	tmpDir, err := os.MkdirTemp("", "bugtrapper-")
	if err != nil {
		return err
	}
	c.tmpDir = tmpDir
	return nil
}

func (c *CLICamera) Capture() (image.Image, error) {
	outPath := filepath.Join(c.tmpDir, "capture.jpg")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("imagesnap", "-w", "1", outPath)
	case "linux":
		cmd = exec.Command("fswebcam", "-r", "1280x720", "--no-banner", outPath)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("capture failed: %s: %w", string(out), err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding captured image: %w", err)
	}
	return img, nil
}

func (c *CLICamera) Close() error {
	if c.tmpDir != "" {
		return os.RemoveAll(c.tmpDir)
	}
	return nil
}

func captureTool() string {
	switch runtime.GOOS {
	case "darwin":
		return "imagesnap"
	case "linux":
		return "fswebcam"
	default:
		return "imagesnap"
	}
}
