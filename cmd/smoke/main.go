// smoke renders the card and receipt SVG templates with sample data so
// you can eyeball the output without needing the camera or OpenAI API.
//
//	go run ./cmd/smoke
//	go run ./cmd/smoke "Eastern Tiger Swallowtail"   # custom title
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"bug_trapper/svgreceipt"
)

func main() {
	width := flag.Int("width", 384, "Output width in pixels")
	flag.Parse()

	name := "Honeybee"
	if flag.NArg() > 0 {
		name = flag.Arg(0)
	}

	cards := []struct {
		template string
		out      string
		count    string
	}{
		{"assets/card.svg", "card-smoke.png", "3"},
		{"assets/receipt.svg", "receipt-smoke.png", "3"},
	}

	illust := blackBlob(256, 256)
	xp := "15"
	if v := os.Getenv("XP"); v != "" {
		xp = v
	}
	vars := map[string]string{
		"name":        name,
		"description": "A small flying insect that pollinates flowers and produces honey. Found across most of the world.",
		"N":           "3",
		"xp":          xp,
		"date":        "April 29th, 2026",
		"count":       "3",
		"timestamp":   "2026-04-29 12:34:56",
	}

	for _, c := range cards {
		tpl, err := os.ReadFile(c.template)
		if err != nil {
			die("read %s: %v", c.template, err)
		}
		img, err := svgreceipt.Render(tpl, vars, illust, *width)
		if err != nil {
			die("render %s: %v", c.template, err)
		}
		f, err := os.Create(c.out)
		if err != nil {
			die("create %s: %v", c.out, err)
		}
		if err := png.Encode(f, img); err != nil {
			die("encode %s: %v", c.out, err)
		}
		f.Close()
		abs, _ := filepath.Abs(c.out)
		fmt.Printf("wrote %s\n", abs)
	}
}

func blackBlob(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, white)
		}
	}
	cx, cy := w/2, h/2
	r := w / 3
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, black)
			}
		}
	}
	return img
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
