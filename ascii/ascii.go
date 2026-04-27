package ascii

import (
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/draw"
)

// Character ramp from lightest to darkest.
// Chosen for visual distinctness at small sizes on a receipt printer.
var ramp = []byte(" .,:;+*#@")

// Converter converts images to ASCII art.
type Converter struct{}

// Convert takes an image and produces ASCII art of the given width.
func (c *Converter) Convert(img image.Image, width int) (string, error) {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// Character aspect ratio correction. Receipt printer chars are roughly
	// 2x taller than wide, so we compress vertical by 0.45.
	aspectRatio := float64(srcH) / float64(srcW)
	height := int(float64(width) * aspectRatio * 0.45)
	if height < 1 {
		height = 1
	}

	// Resize using CatmullRom for sharper downscaling
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// Collect all grayscale values for contrast stretching
	pixels := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixels[y*width+x] = toGray(resized.At(x, y))
		}
	}

	// Auto-contrast: stretch histogram to use full range
	pixels = stretchContrast(pixels)

	// Apply a slight gamma curve to push midtones apart
	for i, p := range pixels {
		normalized := float64(p) / 255.0
		pixels[i] = uint8(math.Pow(normalized, 0.8) * 255)
	}

	// Build ASCII output
	var b strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			gray := pixels[y*width+x]
			// Map 0-255: 0=black=@(dense), 255=white=space(light)
			idx := int(255-gray) * (len(ramp) - 1) / 255
			b.WriteByte(ramp[idx])
		}
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// stretchContrast remaps pixel values so the darkest pixel becomes 0
// and the brightest becomes 255, using the 2nd/98th percentile to
// avoid outlier pixels blowing out the range.
func stretchContrast(pixels []uint8) []uint8 {
	if len(pixels) == 0 {
		return pixels
	}

	// Build histogram
	var hist [256]int
	for _, p := range pixels {
		hist[p]++
	}

	// Find 2nd and 98th percentile
	total := len(pixels)
	lo, hi := uint8(0), uint8(255)
	count := 0
	for i := 0; i < 256; i++ {
		count += hist[i]
		if count >= total*2/100 {
			lo = uint8(i)
			break
		}
	}
	count = 0
	for i := 255; i >= 0; i-- {
		count += hist[i]
		if count >= total*2/100 {
			hi = uint8(i)
			break
		}
	}

	if hi <= lo {
		return pixels
	}

	// Remap
	result := make([]uint8, len(pixels))
	span := float64(hi - lo)
	for i, p := range pixels {
		if p <= lo {
			result[i] = 0
		} else if p >= hi {
			result[i] = 255
		} else {
			result[i] = uint8(float64(p-lo) / span * 255)
		}
	}
	return result
}

func toGray(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	// Standard luminance formula, values are in [0,65535]
	gray := (19595*r + 38470*g + 7471*b + 1<<15) >> 24
	return uint8(gray)
}
