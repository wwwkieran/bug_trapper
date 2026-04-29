package printer

import (
	"image"

	"golang.org/x/image/draw"
)

const (
	// PrinterDotsWide is the print head width in dots for a 58mm thermal
	// printer (384 dots = 48 bytes per row).
	PrinterDotsWide = 384
	// chunkRows is the per-GS-v-0-command height limit. The printer
	// concatenates back-to-back raster commands seamlessly, so taller images
	// are split into chunks of this height.
	chunkRows = 256
)

// encodeRaster scales img to fit PrinterDotsWide wide (preserving aspect
// ratio), Floyd-Steinberg-dithers it to 1-bit, and returns the ESC/POS
// GS v 0 command bytes — split into chunks of chunkRows so that arbitrarily
// tall images print without per-command height limits.
func encodeRaster(img image.Image) []byte {
	scaled := scaleToPrinter(img)
	bits := ditherFloydSteinberg(scaled)
	return packRasterChunks(bits, chunkRows)
}

func scaleToPrinter(img image.Image) *image.Gray {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return image.NewGray(image.Rect(0, 0, 0, 0))
	}

	dstW := PrinterDotsWide
	dstH := srcH * dstW / srcW

	gray := image.NewGray(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(gray, gray.Bounds(), img, bounds, draw.Over, nil)
	return gray
}

// ditherFloydSteinberg converts a grayscale image to a 2D bool slice where
// true = black pixel. Uses Floyd-Steinberg error diffusion for natural
// halftones.
func ditherFloydSteinberg(src *image.Gray) [][]bool {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()

	// Work buffer of float errors; copy source into it.
	buf := make([][]float32, h)
	for y := range buf {
		buf[y] = make([]float32, w)
		for x := 0; x < w; x++ {
			buf[y][x] = float32(src.GrayAt(x, y).Y)
		}
	}

	out := make([][]bool, h)
	for y := 0; y < h; y++ {
		out[y] = make([]bool, w)
		for x := 0; x < w; x++ {
			old := buf[y][x]
			var newPx float32 = 255
			if old < 128 {
				newPx = 0
				out[y][x] = true
			}
			err := old - newPx
			if x+1 < w {
				buf[y][x+1] += err * 7 / 16
			}
			if y+1 < h {
				if x > 0 {
					buf[y+1][x-1] += err * 3 / 16
				}
				buf[y+1][x] += err * 5 / 16
				if x+1 < w {
					buf[y+1][x+1] += err * 1 / 16
				}
			}
		}
	}
	return out
}

// packRasterChunks emits one GS v 0 command per chunk of up to maxRows rows.
// The printer prints them back-to-back with no visible seam.
func packRasterChunks(bits [][]bool, maxRows int) []byte {
	if maxRows <= 0 {
		return packRaster(bits)
	}
	var out []byte
	for y := 0; y < len(bits); y += maxRows {
		end := y + maxRows
		if end > len(bits) {
			end = len(bits)
		}
		out = append(out, packRaster(bits[y:end])...)
	}
	return out
}

// packRaster turns the bool grid into ESC/POS GS v 0 command bytes.
// Format: 0x1D 0x76 0x30 m xL xH yL yH + (xL+xH*256)*yH*256+yL bytes
// with 8 pixels packed MSB-first per byte, left-to-right, top-to-bottom.
func packRaster(bits [][]bool) []byte {
	h := len(bits)
	if h == 0 {
		return nil
	}
	w := len(bits[0])
	widthBytes := (w + 7) / 8

	data := make([]byte, 0, 8+widthBytes*h)
	data = append(data,
		0x1D, 0x76, 0x30, 0x00,
		byte(widthBytes&0xff), byte((widthBytes>>8)&0xff),
		byte(h&0xff), byte((h>>8)&0xff),
	)

	for y := 0; y < h; y++ {
		row := bits[y]
		for bx := 0; bx < widthBytes; bx++ {
			var b byte
			for bit := 0; bit < 8; bit++ {
				x := bx*8 + bit
				if x < w && row[x] {
					b |= 1 << (7 - bit)
				}
			}
			data = append(data, b)
		}
	}
	return data
}
