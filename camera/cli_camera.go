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
// Linux: rpicam-jpeg / libcamera-jpeg (Raspberry Pi) or fswebcam (USB webcam)
type CLICamera struct {
	tmpDir string
	tool   string
}

func NewCLICamera() *CLICamera {
	return &CLICamera{}
}

func (c *CLICamera) Open() error {
	tool, err := findCaptureTool()
	if err != nil {
		return err
	}
	c.tool = tool
	tmpDir, err := os.MkdirTemp("", "bugtrapper-")
	if err != nil {
		return err
	}
	c.tmpDir = tmpDir
	return nil
}

func (c *CLICamera) Capture() (image.Image, error) {
	outPath := filepath.Join(c.tmpDir, "capture.jpg")

	cmd, err := buildCaptureCmd(c.tool, outPath)
	if err != nil {
		return nil, err
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("capture failed: %s: %w", string(out), err)
	}

	if _, err := os.Stat(outPath); err != nil {
		return nil, fmt.Errorf("capture produced no file at %s: %w", outPath, err)
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

// findCaptureTool returns the first available capture tool for this platform.
// On Linux, it prefers rpicam-jpeg / libcamera-jpeg (Raspberry Pi camera stack)
// before falling back to fswebcam for USB webcams.
func findCaptureTool() (string, error) {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"imagesnap"}
	case "linux":
		candidates = []string{"rpicam-jpeg", "libcamera-jpeg", "fswebcam"}
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	for _, tool := range candidates {
		if _, err := exec.LookPath(tool); err == nil {
			return tool, nil
		}
	}

	switch runtime.GOOS {
	case "darwin":
		return "", fmt.Errorf("imagesnap not found; install with: brew install imagesnap")
	case "linux":
		return "", fmt.Errorf("no capture tool found; install rpicam-apps (Raspberry Pi camera) or fswebcam (USB webcam)")
	default:
		return "", fmt.Errorf("no capture tool found for %s", runtime.GOOS)
	}
}

func buildCaptureCmd(tool, outPath string) (*exec.Cmd, error) {
	switch tool {
	case "imagesnap":
		return exec.Command("imagesnap", "-w", "1", outPath), nil
	case "rpicam-jpeg", "libcamera-jpeg":
		return exec.Command(tool, "-o", outPath, "--width", "1280", "--height", "720", "-n"), nil
	case "fswebcam":
		return exec.Command("fswebcam", "-r", "1280x720", "--no-banner", outPath), nil
	default:
		return nil, fmt.Errorf("unknown capture tool: %s", tool)
	}
}
