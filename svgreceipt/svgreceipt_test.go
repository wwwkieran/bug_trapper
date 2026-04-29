package svgreceipt

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestPrepareSubstitutesTextPlaceholders(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 100">
  <text x="10" y="20">Hello {{name}}!</text>
</svg>`)

	prep, err := Prepare(in, map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !strings.Contains(string(prep.SVG), "Hello World!") {
		t.Errorf("placeholder not substituted; got: %s", prep.SVG)
	}
}

func TestPrepareSubstitutesAttributePlaceholders(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 100">
  <rect fill="{{color}}" width="10" height="10"/>
</svg>`)

	prep, err := Prepare(in, map[string]string{"color": "black"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !strings.Contains(string(prep.SVG), `fill="black"`) {
		t.Errorf("attribute placeholder not substituted; got: %s", prep.SVG)
	}
}

func TestPrepareLeavesUnknownPlaceholdersAlone(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 100">
  <text>{{unknown}}</text>
</svg>`)

	prep, err := Prepare(in, map[string]string{})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !strings.Contains(string(prep.SVG), "{{unknown}}") {
		t.Errorf("unknown placeholder should be left untouched; got: %s", prep.SVG)
	}
}

func TestPrepareExtractsIllustrationSlot(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 800">
  <rect id="illustration" x="10" y="100" width="364" height="364" fill="red"/>
</svg>`)

	prep, err := Prepare(in, nil)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prep.Slot == nil {
		t.Fatal("expected illustration slot to be extracted")
	}
	if prep.Slot.X != 10 || prep.Slot.Y != 100 || prep.Slot.W != 364 || prep.Slot.H != 364 {
		t.Errorf("slot mismatch: %+v", *prep.Slot)
	}
	// The slot rect should still be present, but with fill="none".
	if !strings.Contains(string(prep.SVG), `fill="none"`) {
		t.Errorf("illustration rect should have fill=\"none\"; got: %s", prep.SVG)
	}
}

func TestPrepareViewBox(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 800"></svg>`)
	prep, err := Prepare(in, nil)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prep.ViewW != 384 || prep.ViewH != 800 {
		t.Errorf("viewBox: got %v×%v, want 384×800", prep.ViewW, prep.ViewH)
	}
}

func TestRasterizeProducesExpectedDimensions(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">
  <rect width="100" height="50" fill="black"/>
</svg>`)

	img, err := Rasterize(in, 384)
	if err != nil {
		t.Fatalf("Rasterize: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Errorf("width: got %d, want 384", img.Bounds().Dx())
	}
	if img.Bounds().Dy() != 192 {
		t.Errorf("height: got %d, want 192 (384*50/100)", img.Bounds().Dy())
	}

	// Black rect should have rendered: center pixel must not be white.
	c := img.RGBAAt(192, 96)
	if c.R == 255 && c.G == 255 && c.B == 255 {
		t.Errorf("center pixel is white; rect did not render: %+v", c)
	}
}

func TestPrepareWrapsTextWithDataWrap(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 200">
  <text x="10" y="40" font-size="14" data-wrap="10" data-line-height="20">{{msg}}</text>
</svg>`)

	prep, err := Prepare(in, map[string]string{"msg": "hello world from the wrapper"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := string(prep.SVG)
	// Expect 4 lines from greedy wrap at width=10: "hello", "world from", "the", "wrapper"
	if got := strings.Count(out, "<text"); got != 4 {
		t.Errorf("expected 4 wrapped <text> elements; got %d in: %s", got, out)
	}
	for _, want := range []string{">hello<", ">world from<", ">the<", ">wrapper<"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing wrapped line %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "data-wrap") {
		t.Errorf("data-wrap should be stripped; got: %s", out)
	}
	for _, y := range []string{`y="40"`, `y="60"`, `y="80"`, `y="100"`} {
		if !strings.Contains(out, y) {
			t.Errorf("missing %s in: %s", y, out)
		}
	}
}

func TestWrapText(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  []string
	}{
		{"hello world", 32, []string{"hello world"}},
		{"hello world from the wrapper", 10, []string{"hello", "world from", "the", "wrapper"}},
		{"", 10, nil},
		{"supercalifragilistic short", 10, []string{"supercalifragilistic", "short"}},
	}
	for _, tc := range cases {
		got := wrapText(tc.in, tc.width)
		if len(got) != len(tc.want) {
			t.Errorf("wrapText(%q, %d): got %d lines %v, want %d %v",
				tc.in, tc.width, len(got), got, len(tc.want), tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("wrapText(%q, %d): line %d got %q, want %q",
					tc.in, tc.width, i, got[i], tc.want[i])
			}
		}
	}
}

func TestRenderEndToEnd(t *testing.T) {
	in := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 800">
  <text x="192" y="40" text-anchor="middle">{{name}}</text>
  <rect id="illustration" x="0" y="100" width="384" height="384" fill="red"/>
</svg>`)

	// Build a small all-blue illustration.
	illust := image.NewRGBA(image.Rect(0, 0, 64, 64))
	blue := color.RGBA{0, 0, 255, 255}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			illust.Set(x, y, blue)
		}
	}

	out, err := Render(in, map[string]string{"name": "Beetle"}, illust, 384)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if out.Bounds().Dx() != 384 {
		t.Errorf("width: got %d, want 384", out.Bounds().Dx())
	}
	// The slot is at y=100..484 in SVG units. With output 384×800, slot is
	// the same in pixels (1:1). Center of the slot should be blue-ish.
	cx, cy := 192, 100+192
	c := out.RGBAAt(cx, cy)
	if c.B < 200 || c.R > 50 {
		t.Errorf("expected blue illustration at slot center, got %+v", c)
	}
}
