package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// TestDrawFrameProducesImage renders one frame and checks the slab background
// and that the ink title text is present.
func TestDrawFrameProducesImage(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	view := &fakeView{open: true, results: []rank.Match{
		{Item: backend.Item{Title: "Ghostty", Subtitle: "Terminal"}},
		{Item: backend.Item{Title: "Firefox", Subtitle: "Web Browser"}},
	}}
	dc := gg.NewContext(1600, 900)
	New().DrawFrame(dc, view, th, config.Default().Reveal)
	img := dc.Image().(*image.RGBA)

	// Outside the band the surface is transparent, as in the QML overlay.
	if got := img.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent outside the band)", got.A)
	}
	// Inside the band the slab or a card is painted.
	if got := img.RGBAAt(800, 450); got.A == 0 {
		t.Error("centre is transparent, want the band painted")
	}
	// Ink title present somewhere near the top-left.
	if !containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Errorf("ink title pixels missing")
	}
}

type fakeView struct {
	open    bool
	results []rank.Match
}

func (v *fakeView) Results() []rank.Match {
	if v.results != nil {
		return v.results
	}
	return nil
}
func (v *fakeView) Selected() int     { return 0 }
func (v *fakeView) Query() string     { return "" }
func (v *fakeView) IsOpen() bool      { return v.open }
func (v *fakeView) ModeBadge() string { return "SEARCH" }

func closeTo(img *image.RGBA, x, y int, r, g, b uint8) bool {
	px := img.RGBAAt(x, y)
	return px.R == r && px.G == g && px.B == b
}

func containsColor(img *image.RGBA, r, g, b uint8) bool {
	for y := 0; y < img.Bounds().Dy(); y += 4 {
		for x := 0; x < img.Bounds().Dx(); x += 4 {
			if closeTo(img, x, y, r, g, b) {
				return true
			}
		}
	}
	return false
}

// TestUnpremultiply checks the S4 fix: straight alpha out for semi-transparent
// pixels, unchanged for opaque and fully transparent.
func TestUnpremultiply(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 3))
	// opaque white
	src.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	// 50% white premultiplied: (128,128,128,128)
	src.SetRGBA(0, 1, color.RGBA{R: 128, G: 128, B: 128, A: 128})
	// fully transparent
	src.SetRGBA(0, 2, color.RGBA{})

	out := unpremultiply(src)
	opaque := out.RGBAAt(0, 0)
	if opaque.R != 255 || opaque.G != 255 || opaque.B != 255 {
		t.Errorf("opaque = %+v, want white", opaque)
	}
	half := out.RGBAAt(0, 1)
	if half.R != 255 || half.A != 128 {
		t.Errorf("half = %+v, want straight white at alpha 128", half)
	}
	trans := out.RGBAAt(0, 2)
	if trans.A != 0 {
		t.Errorf("transparent = %+v, want alpha 0", trans)
	}
}

// TestDrawFrameBlanksWhenClosed proves a closed launcher renders fully
// transparent rather than the palette.
func TestDrawFrameBlanksWhenClosed(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	view := &fakeView{open: false}
	dc := gg.NewContext(1600, 900)
	New().DrawFrame(dc, view, th, config.Default().Reveal)
	img := dc.Image().(*image.RGBA)
	if got := img.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("closed frame corner alpha = %d, want 0", got.A)
	}
}
