// Package svgreceipt renders an SVG template into a 384-dot-wide bitmap
// suitable for the thermal printer.
//
// The template is a normal SVG file with three kinds of placeholders:
//
//   - {{key}} tokens in any text content (including <text>) and in
//     attribute values are substituted from a string map.
//
//   - An element with id="illustration" (typically <rect>) marks the slot
//     where the DALL-E illustration is composited as a raster image after
//     the SVG itself has been rasterized. The marker element's fill and
//     stroke are blanked so it doesn't paint on the canvas.
//
//   - A <text> element with a data-wrap="N" attribute is word-wrapped to
//     N characters wide. After substitution the wrapped text is split into
//     one <text> element per line, stacked by data-line-height pixels (or
//     by font-size if data-line-height is absent). The original element
//     determines the first line's position; subsequent lines stack below.
package svgreceipt

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// IllustrationSlot describes the rectangle inside the rasterized SVG where
// the illustration image should be composited.
type IllustrationSlot struct {
	X, Y, W, H int
}

// Prepared is the result of preparing a template: the substituted SVG bytes
// (ready to feed to the rasterizer) plus the illustration slot in SVG user
// units. The slot must be scaled to output pixels by the caller.
type Prepared struct {
	SVG          []byte
	Slot         *IllustrationSlot
	ViewW, ViewH float64
}

var placeholderRE = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

// Prepare substitutes {{key}} placeholders, locates the illustration slot,
// expands data-wrap text elements into multiple lines, and returns the
// modified SVG bytes plus the slot geometry.
// textRun is one piece of text inside a <text> element, with its own
// position. A bare <text>X</text> produces one run; a <text> containing
// multiple <tspan>s produces one run per tspan (each with its own x/y).
type textRun struct {
	x, y    string // empty means "inherit from parent <text>"
	content string
}

func Prepare(svgBytes []byte, vars map[string]string) (*Prepared, error) {
	dec := xml.NewDecoder(bytes.NewReader(svgBytes))
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)

	prep := &Prepared{}

	// Buffer state for <text>...</text>. When we enter a <text> we start
	// buffering its child runs (handling nested <tspan>); on the end tag we
	// emit one or more <text> elements (after substitution + optional wrap).
	var textStart *xml.StartElement
	var textRuns []textRun
	var curRun textRun
	var inTspan bool

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("svgreceipt: parse: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "svg" {
				prep.ViewW, prep.ViewH = parseViewBox(t.Attr)
			}
			if id := attr(t.Attr, "id"); id == "illustration" {
				slot, ok := slotFromAttrs(t.Attr)
				if ok {
					prep.Slot = &slot
				}
				t.Attr = setAttr(t.Attr, "fill", "none")
				t.Attr = setAttr(t.Attr, "stroke", "none")
			}
			t.Attr = substAttrs(t.Attr, vars)

			if textStart == nil && t.Name.Local == "text" {
				cp := t.Copy()
				textStart = &cp
				textRuns = nil
				curRun = textRun{}
				continue
			}
			if textStart != nil && t.Name.Local == "tspan" {
				// Flush any direct text accumulated before this tspan.
				if curRun.content != "" {
					textRuns = append(textRuns, curRun)
				}
				curRun = textRun{
					x: attr(t.Attr, "x"),
					y: attr(t.Attr, "y"),
				}
				inTspan = true
				continue
			}
			if textStart != nil {
				// Unknown nested element inside <text> — ignore, keep buffering.
				continue
			}

			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

		case xml.EndElement:
			if textStart != nil && t.Name.Local == "tspan" {
				textRuns = append(textRuns, curRun)
				curRun = textRun{}
				inTspan = false
				continue
			}
			if textStart != nil && t.Name.Local == "text" {
				if curRun.content != "" {
					textRuns = append(textRuns, curRun)
				}
				if err := emitText(enc, *textStart, textRuns, vars); err != nil {
					return nil, err
				}
				textStart = nil
				textRuns = nil
				curRun = textRun{}
				continue
			}
			if textStart != nil {
				continue
			}
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

		case xml.CharData:
			if textStart != nil {
				_ = inTspan // (kept as a marker for future use)
				curRun.content += string(t)
				continue
			}
			if err := enc.EncodeToken(xml.CharData(substText(string(t), vars))); err != nil {
				return nil, err
			}

		default:
			if textStart != nil {
				continue
			}
			if err := enc.EncodeToken(tok); err != nil {
				return nil, err
			}
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}

	return &Prepared{
		SVG:   buf.Bytes(),
		Slot:  prep.Slot,
		ViewW: prep.ViewW,
		ViewH: prep.ViewH,
	}, nil
}

// emitText emits the buffered <text> element. If the start tag has
// data-wrap, all runs are joined and word-wrapped. Otherwise each run is
// emitted as its own <text> element with the run's x/y overriding the
// parent's (this flattens <tspan> children into siblings, which is what
// tdewolff/canvas needs since it doesn't render <tspan>).
func emitText(enc *xml.Encoder, start xml.StartElement, runs []textRun, vars map[string]string) error {
	cleanAttrs := stripAttrs(start.Attr, "data-wrap", "data-line-height")

	if attr(start.Attr, "data-wrap") != "" {
		var joined strings.Builder
		for _, r := range runs {
			if joined.Len() > 0 {
				joined.WriteByte(' ')
			}
			joined.WriteString(strings.TrimSpace(r.content))
		}
		// Use the first run's x/y if the parent <text> doesn't set them
		// directly (typical of Figma-exported SVGs where the position lives
		// on the <tspan>).
		wrapStart := xml.StartElement{Name: start.Name, Attr: copyAttrs(start.Attr)}
		if attr(wrapStart.Attr, "x") == "" && len(runs) > 0 && runs[0].x != "" {
			wrapStart.Attr = setAttr(wrapStart.Attr, "x", runs[0].x)
		}
		if attr(wrapStart.Attr, "y") == "" && len(runs) > 0 && runs[0].y != "" {
			wrapStart.Attr = setAttr(wrapStart.Attr, "y", runs[0].y)
		}
		return emitWrapped(enc, wrapStart, substText(joined.String(), vars))
	}

	if len(runs) == 0 {
		return nil
	}

	for _, r := range runs {
		runStart := xml.StartElement{Name: start.Name, Attr: copyAttrs(cleanAttrs)}
		if r.x != "" {
			runStart.Attr = setAttr(runStart.Attr, "x", r.x)
		}
		if r.y != "" {
			runStart.Attr = setAttr(runStart.Attr, "y", r.y)
		}
		if err := enc.EncodeToken(runStart); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.CharData(substText(r.content, vars))); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
			return err
		}
	}
	return nil
}

func emitWrapped(enc *xml.Encoder, start xml.StartElement, content string) error {
	width, _ := strconv.Atoi(attr(start.Attr, "data-wrap"))
	if width <= 0 {
		width = 32
	}
	lineHeight, ok := parseFloat(attr(start.Attr, "data-line-height"))
	if !ok {
		fontSize, ok := parseFloat(attr(start.Attr, "font-size"))
		if !ok {
			fontSize = 14
		}
		lineHeight = fontSize * 1.25
	}
	startY, _ := parseFloat(attr(start.Attr, "y"))

	// Strip our internal data-* attrs before emitting so the SVG parser
	// doesn't see noise.
	cleanAttrs := stripAttrs(start.Attr, "data-wrap", "data-line-height")

	lines := wrapText(strings.TrimSpace(content), width)
	if len(lines) == 0 {
		lines = []string{""}
	}

	for i, line := range lines {
		lineStart := xml.StartElement{Name: start.Name, Attr: copyAttrs(cleanAttrs)}
		lineStart.Attr = setAttr(lineStart.Attr, "y", fmt.Sprintf("%g", startY+float64(i)*lineHeight))

		if err := enc.EncodeToken(lineStart); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.CharData(line)); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
			return err
		}
	}
	return nil
}

// wrapText word-wraps s to width characters. Words longer than width are
// emitted on their own line (not broken).
func wrapText(s string, width int) []string {
	if s == "" {
		return nil
	}
	var lines []string
	var cur strings.Builder
	for _, word := range strings.Fields(s) {
		if cur.Len() == 0 {
			cur.WriteString(word)
			continue
		}
		if cur.Len()+1+len(word) <= width {
			cur.WriteByte(' ')
			cur.WriteString(word)
			continue
		}
		lines = append(lines, cur.String())
		cur.Reset()
		cur.WriteString(word)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func substText(s string, vars map[string]string) string {
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		key := placeholderRE.FindStringSubmatch(m)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}

func substAttrs(attrs []xml.Attr, vars map[string]string) []xml.Attr {
	out := make([]xml.Attr, len(attrs))
	for i, a := range attrs {
		a.Value = substText(a.Value, vars)
		out[i] = a
	}
	return out
}

func copyAttrs(attrs []xml.Attr) []xml.Attr {
	out := make([]xml.Attr, len(attrs))
	copy(out, attrs)
	return out
}

func stripAttrs(attrs []xml.Attr, names ...string) []xml.Attr {
	skip := make(map[string]bool, len(names))
	for _, n := range names {
		skip[n] = true
	}
	out := attrs[:0:0]
	for _, a := range attrs {
		if !skip[a.Name.Local] {
			out = append(out, a)
		}
	}
	return out
}

func attr(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func setAttr(attrs []xml.Attr, name, value string) []xml.Attr {
	for i, a := range attrs {
		if a.Name.Local == name {
			attrs[i].Value = value
			return attrs
		}
	}
	return append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

func slotFromAttrs(attrs []xml.Attr) (IllustrationSlot, bool) {
	x, okx := parseFloat(attr(attrs, "x"))
	y, oky := parseFloat(attr(attrs, "y"))
	w, okw := parseFloat(attr(attrs, "width"))
	h, okh := parseFloat(attr(attrs, "height"))
	if !okx || !oky || !okw || !okh {
		return IllustrationSlot{}, false
	}
	return IllustrationSlot{X: int(x), Y: int(y), W: int(w), H: int(h)}, true
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "px")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseViewBox(attrs []xml.Attr) (float64, float64) {
	if vb := attr(attrs, "viewBox"); vb != "" {
		parts := strings.Fields(vb)
		if len(parts) == 4 {
			w, _ := parseFloat(parts[2])
			h, _ := parseFloat(parts[3])
			return w, h
		}
	}
	w, _ := parseFloat(attr(attrs, "width"))
	h, _ := parseFloat(attr(attrs, "height"))
	return w, h
}

// Compose composites illustration onto canvas at slot, scaling to fit while
// preserving aspect ratio (centered).
func Compose(canvas *image.RGBA, slot IllustrationSlot, illustration image.Image) {
	if illustration == nil || slot.W <= 0 || slot.H <= 0 {
		return
	}
	src := illustration.Bounds()
	srcW, srcH := src.Dx(), src.Dy()
	if srcW == 0 || srcH == 0 {
		return
	}

	scale := float64(slot.W) / float64(srcW)
	if s := float64(slot.H) / float64(srcH); s < scale {
		scale = s
	}
	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	dstX := slot.X + (slot.W-dstW)/2
	dstY := slot.Y + (slot.H-dstH)/2

	scaleAndPaste(canvas, dstX, dstY, dstW, dstH, illustration)
}
