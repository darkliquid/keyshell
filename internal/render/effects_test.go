package render

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

func TestImagePainterPlaceholder(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 100, 60)
	cache := NewImageCache()
	n := &eval.ResolvedNode{Name: "Image", Rect: eval.RectF{X: 10, Y: 10, W: 40, H: 30},
		Props: map[string]eval.Value{
			"source": eval.Str("/no/such.png"),
			"fit":    eval.Kw("cover"),
			"tint":   eval.Color{R: 0xFF, G: 0, B: 0, A: 0x80},
		}}
	painterFor("Image")(dc, th, n, &PainterDeps{Images: cache})
	if img.Pix[25*img.Stride+20*4+3] == 0 {
		t.Errorf("placeholder not painted for an unloaded image")
	}
}

func TestGradientFill(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 10, 10)
	n := &eval.ResolvedNode{Name: "Box", Rect: eval.RectF{X: 0, Y: 0, W: 10, H: 10},
		Props: map[string]eval.Value{"fill": eval.FuncVal{Name: "linear-gradient", Args: []eval.Value{
			eval.Color{R: 0, G: 0, B: 0, A: 0xFF}, eval.Color{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}}}}
	painterFor("Box")(dc, th, n, &PainterDeps{})
	top := img.Pix[0*img.Stride+5*4]
	bot := img.Pix[9*img.Stride+5*4]
	if top == bot {
		t.Errorf("gradient painted flat: top=%v bot=%v", top, bot)
	}
}

func TestPaintImageResolvesNamedIcons(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 60, 60)
	dc.ClearWithColor(gg.RGBA{})

	n := &eval.ResolvedNode{
		Name: "Image",
		Rect: eval.RectF{X: 10, Y: 10, W: 40, H: 40},
		Props: map[string]eval.Value{
			"source": eval.Str("terminal"),
		},
	}
	painterFor("Image")(dc, th, n, &PainterDeps{})
	if !anyColor(img) {
		t.Errorf("named icon did not paint fallback mark")
	}
}

func TestPaintImageGlyph(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 60, 60)
	dc.ClearWithColor(gg.RGBA{})

	n := &eval.ResolvedNode{
		Name: "Icon",
		Rect: eval.RectF{X: 10, Y: 10, W: 40, H: 40},
		Props: map[string]eval.Value{
			"glyph": eval.Str("▶"),
			"size":  eval.Num(20),
			"color": eval.Color{R: 0xFF, G: 0, B: 0, A: 0xFF},
		},
	}
	painterFor("Icon")(dc, th, n, &PainterDeps{})
	totalNonZero := 0
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			if img.RGBAAt(x, y).A != 0 {
				totalNonZero++
			}
		}
	}
	if totalNonZero == 0 {
		t.Errorf("glyph did not paint any pixels (totalNonZero=0)")
	}

	// Test text glyph
	n.Props["glyph"] = eval.Str("A")
	dc.ClearWithColor(gg.RGBA{})
	painterFor("Icon")(dc, th, n, &PainterDeps{})
	totalNonZero = 0
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			if img.RGBAAt(x, y).A != 0 {
				totalNonZero++
			}
		}
	}
	if totalNonZero == 0 {
		t.Errorf("glyph 'A' did not paint any pixels")
	}
}

func TestPaintImageWordGlyphDoesNotOverflow(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 100, 40)
	dc.ClearWithColor(gg.RGBA{})

	// Node constrained to width 24, height 24 at X=10, Y=10.
	n := &eval.ResolvedNode{
		Name: "Icon",
		Rect: eval.RectF{X: 10, Y: 10, W: 24, H: 24},
		Props: map[string]eval.Value{
			"glyph": eval.Str("command"),
		},
	}
	painterFor("Icon")(dc, th, n, &PainterDeps{})

	// Check that no pixels were painted outside the 24x24 box (X > 36)
	for y := 0; y < 40; y++ {
		for x := 36; x < 100; x++ {
			if img.RGBAAt(x, y).A != 0 {
				t.Fatalf("word glyph overflowed 24px icon box at (%d, %d): %+v", x, y, img.RGBAAt(x, y))
			}
		}
	}
}
