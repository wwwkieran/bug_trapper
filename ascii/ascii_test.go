package ascii

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func solidImage(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestConvertOutputWidth(t *testing.T) {
	img := solidImage(100, 100, color.White)
	conv := &Converter{}
	result, err := conv.Convert(img, 32)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	for i, line := range strings.Split(strings.TrimRight(result, "\n"), "\n") {
		if len(line) != 32 {
			t.Errorf("line %d width=%d, want 32: %q", i+1, len(line), line)
		}
	}
}

func TestConvertWhiteImage(t *testing.T) {
	img := solidImage(64, 64, color.White)
	conv := &Converter{}
	result, err := conv.Convert(img, 32)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	// White pixels should map to lightest character (space)
	trimmed := strings.TrimRight(result, "\n")
	for _, line := range strings.Split(trimmed, "\n") {
		for _, ch := range line {
			if ch != ' ' {
				t.Errorf("white image should produce spaces, got %q", string(ch))
				return
			}
		}
	}
}

func TestConvertBlackImage(t *testing.T) {
	img := solidImage(64, 64, color.Black)
	conv := &Converter{}
	result, err := conv.Convert(img, 32)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	// Black pixels should map to densest character
	trimmed := strings.TrimRight(result, "\n")
	for _, line := range strings.Split(trimmed, "\n") {
		for _, ch := range line {
			if ch != '@' {
				t.Errorf("black image should produce '@', got %q", string(ch))
				return
			}
		}
	}
}

func TestConvertNonEmpty(t *testing.T) {
	img := solidImage(64, 64, color.RGBA{128, 128, 128, 255})
	conv := &Converter{}
	result, err := conv.Convert(img, 32)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	if len(strings.TrimSpace(result)) == 0 {
		t.Error("result should not be empty for a non-trivial image")
	}
}

func TestConvertSmallImage(t *testing.T) {
	// Image smaller than target width — should still produce correct width
	img := solidImage(10, 10, color.RGBA{100, 100, 100, 255})
	conv := &Converter{}
	result, err := conv.Convert(img, 32)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one line of output")
	}
	for i, line := range lines {
		if len(line) != 32 {
			t.Errorf("line %d width=%d, want 32", i+1, len(line))
		}
	}
}

func TestConvertProducesMultipleLines(t *testing.T) {
	img := solidImage(100, 200, color.Gray{128})
	conv := &Converter{}
	result, err := conv.Convert(img, 32)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) < 2 {
		t.Errorf("tall image should produce multiple lines, got %d", len(lines))
	}
}
