package svgreceipt

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"golang.org/x/image/draw"
)

// Render is the high-level entry point: substitutes placeholders, rasterizes
// the SVG to widthPx pixels wide, and composites the illustration into the
// slot. Returns an RGBA image ready for the printer.
func Render(svgBytes []byte, vars map[string]string, illustration image.Image, widthPx int) (*image.RGBA, error) {
	prep, err := Prepare(svgBytes, vars)
	if err != nil {
		return nil, err
	}

	canvasImg, err := Rasterize(prep.SVG, widthPx)
	if err != nil {
		return nil, err
	}

	if prep.Slot != nil && illustration != nil {
		bounds := canvasImg.Bounds()
		var sx, sy float64
		if prep.ViewW > 0 {
			sx = float64(bounds.Dx()) / prep.ViewW
		} else {
			sx = 1
		}
		if prep.ViewH > 0 {
			sy = float64(bounds.Dy()) / prep.ViewH
		} else {
			sy = 1
		}
		scaled := IllustrationSlot{
			X: int(float64(prep.Slot.X) * sx),
			Y: int(float64(prep.Slot.Y) * sy),
			W: int(float64(prep.Slot.W) * sx),
			H: int(float64(prep.Slot.H) * sy),
		}
		Compose(canvasImg, scaled, illustration)
	}

	return canvasImg, nil
}

// Rasterize parses svgBytes and renders to an RGBA image widthPx wide.
// The height is derived from the SVG viewBox aspect ratio. Uses
// tdewolff/canvas, which has full text/font support.
func Rasterize(svgBytes []byte, widthPx int) (*image.RGBA, error) {
	c, err := canvas.ParseSVG(bytes.NewReader(svgBytes))
	if err != nil {
		return nil, fmt.Errorf("svgreceipt: parse svg: %w", err)
	}

	if c.W <= 0 {
		return nil, fmt.Errorf("svgreceipt: svg has zero width")
	}

	// canvas measures in mm; pick a resolution that produces exactly widthPx.
	resolution := canvas.DPMM(float64(widthPx) / c.W)

	rgba := rasterizer.Draw(c, resolution, canvas.DefaultColorSpace)

	// canvas can produce off-by-one pixel rounding vs the requested width.
	if rgba.Bounds().Dx() != widthPx {
		rgba = resizeWidth(rgba, widthPx)
	}

	// Flatten transparency onto white so the printer sees a clean canvas
	// regardless of what the SVG painted (or didn't paint).
	out := image.NewRGBA(rgba.Bounds())
	draw.Draw(out, out.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), rgba, image.Point{}, draw.Over)
	return out, nil
}

func resizeWidth(src *image.RGBA, widthPx int) *image.RGBA {
	srcB := src.Bounds()
	scale := float64(widthPx) / float64(srcB.Dx())
	heightPx := int(float64(srcB.Dy())*scale + 0.5)
	dst := image.NewRGBA(image.Rect(0, 0, widthPx, heightPx))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, srcB, draw.Src, nil)
	return dst
}
