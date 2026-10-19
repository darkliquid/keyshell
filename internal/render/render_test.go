package render

import (
	"image"
	"image/color"
	"testing"

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

	view := &fakeView{}
	dc := gg.NewContext(1600, 900)
	DrawFrame(dc, view, th)
	img := dc.Image().(*image.RGBA)

	// Slab at a corner away from content.
	if !closeTo(img, 0, 0, 0x0B, 0x0B, 0x0F) {
		t.Errorf("corner not slab background")
	}
	// Ink title present somewhere near the top-left.
	if !containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Errorf("ink title pixels missing")
	}
}

type fakeView struct{}

func (v *fakeView) Results() []rank.Match { return nil }
func (v *fakeView) Selected() int         { return 0 }
func (v *fakeView) Query() string         { return "" }

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
