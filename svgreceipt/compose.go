package svgreceipt

import (
	"image"
	"image/color"

	"golang.org/x/image/draw"
)

var whiteColor = color.RGBA{255, 255, 255, 255}

// scaleAndPaste resamples src to dstW×dstH and draws it onto dst at (dstX,dstY).
// Uses CatmullRom for high-quality downscaling — the printer's dither step
// will collapse any softness back to crisp 1-bit output.
func scaleAndPaste(dst *image.RGBA, dstX, dstY, dstW, dstH int, src image.Image) {
	if dstW <= 0 || dstH <= 0 {
		return
	}
	rect := image.Rect(dstX, dstY, dstX+dstW, dstY+dstH)
	draw.CatmullRom.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)
}
