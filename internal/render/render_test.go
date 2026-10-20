package render

import (
	"image"
	"image/color"
	"math"
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

// TestMaskFrameKeepsTheBand checks the mask leaves pixels inside the band
// untouched and clears everything outside it.
func TestMaskFrameKeepsTheBandContent(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 3))
	src.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	src.SetRGBA(0, 1, color.RGBA{R: 128, G: 128, B: 128, A: 128})
	src.SetRGBA(0, 2, color.RGBA{})

	r := New()
	r.wasOpen = true
	r.surfaceW = 1
	r.reveal = Reveal{YLeft: 75, YRight: 75, HLeft: 400, HRight: 400, Yc: 75}

	r.MaskFrame(src)
	if got := src.RGBAAt(0, 0); got.R != 255 || got.G != 255 || got.B != 255 || got.A != 255 {
		t.Errorf("opaque = %+v, want white at full alpha", got)
	}
	if got := src.RGBAAt(0, 1); got.R != 128 || got.A != 128 {
		t.Errorf("half = %+v, want the premultiplied pixel untouched", got)
	}
	if got := src.RGBAAt(0, 2); got.A != 0 {
		t.Errorf("transparent = %+v, want alpha 0", got)
	}

	// The same frame with the band moved off the rows clears them entirely.
	r.reveal = Reveal{YLeft: -500, YRight: -500, HLeft: 100, HRight: 100}
	src.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	r.MaskFrame(src)
	if got := src.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("outside the band = %+v, want alpha 0", got)
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

// TestDrawFrameClipsContentToTheBand proves nothing paints outside the slab.
// Cards are sheared more steeply than the band tilts, so without a clip their
// corners and the reel overflow the trapezoid and the overlay leaks onto the
// desktop.
func TestDrawFrameClipsContentToTheBand(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	rev := Reveal{
		YLeft: 260, YRight: 640,
		HLeft: 120, HRight: 320,
		Angle: math.Atan2(380, 1600),
		Shear: 0.30,
		Yc:    450,
	}
	r := New()
	r.wasOpen = true
	r.reveal = rev

	view := &fakeView{open: true, results: []rank.Match{
		{Item: backend.Item{Title: "Ghostty", Icon: "terminal", Actions: []backend.Action{{Type: "exec", Value: "ghostty"}}}},
		{Item: backend.Item{Title: "Firefox", Icon: "firefox", Actions: []backend.Action{{Type: "exec", Value: "firefox"}}}},
	}}
	dc := gg.NewContext(1600, 900)
	r.DrawFrame(dc, view, th, config.Default().Reveal)
	img := dc.Image().(*image.RGBA)

	quad := [4][2]float64{
		{0, rev.YLeft - rev.HLeft/2},
		{1600, rev.YRight - rev.HRight/2},
		{1600, rev.YRight + rev.HRight/2},
		{0, rev.YLeft + rev.HLeft/2},
	}
	before := countOutsideBand(img, quad)
	r.MaskFrame(img)
	leaks := countOutsideBand(img, quad)
	if before == 0 {
		t.Fatal("no content painted outside the band: the mask is not exercised")
	}
	if leaks != 0 {
		t.Errorf("%d painted pixels still outside the band after masking (was %d)", leaks, before)
	}
}

// countOutsideBand counts painted pixels outside the quad.
func countOutsideBand(img *image.RGBA, quad [4][2]float64) int {
	leaks := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y).A == 0 {
				continue
			}
			if insideConvex(quad, float64(x)+0.5, float64(y)+0.5) {
				continue
			}
			leaks++
		}
	}
	return leaks
}
