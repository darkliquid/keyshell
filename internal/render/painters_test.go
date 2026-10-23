package render

import (
	"image"
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

func testContext(t *testing.T, w, h int) (*gg.Context, *image.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	pm := gg.NewPixmapFromBuffer(img.Pix, w, h)
	return gg.NewContextForPixmap(pm), img
}

func TestPaintersPaint(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 200, 100)

	tests := []struct {
		name string
		node *eval.ResolvedNode
	}{
		{"Box", &eval.ResolvedNode{Name: "Box", Rect: eval.RectF{X: 10, Y: 10, W: 50, H: 30},
			Props: map[string]eval.Value{"fill": eval.Color{R: 0xFF, G: 0, B: 0, A: 0xFF}}}},
		{"Path", &eval.ResolvedNode{Name: "Path", Rect: eval.RectF{X: 0, Y: 0, W: 200, H: 100},
			Props: map[string]eval.Value{"d": eval.Str("M 0 0 L 200 0 L 200 100 L 0 100 Z"), "fill": eval.Color{R: 0, G: 0, B: 0xFF, A: 0xFF}}}},
		{"Cursor", &eval.ResolvedNode{Name: "Cursor", Rect: eval.RectF{X: 60, Y: 10, W: 3, H: 24},
			Props: map[string]eval.Value{"color": eval.Color{R: 0xFF, G: 0x2E, B: 0x63, A: 0xFF}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc.ClearWithColor(gg.RGBA{})
			painterFor(tt.node.Name)(dc, th, tt.node, &PainterDeps{CursorAlpha: 1})
			if !anyColor(img) {
				t.Errorf("%s painted nothing", tt.name)
			}
		})
	}
}

func TestPaintTextUppercase(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 100, 40)
	n := &eval.ResolvedNode{Name: "Text", Rect: eval.RectF{X: 5, Y: 10, W: 90, H: 20},
		Props: map[string]eval.Value{
			"text": eval.Str("hi"), "size": eval.Length{V: 12, Unit: "px"},
			"color": eval.Color{R: 0xF2, G: 0xF2, B: 0xF7, A: 0xFF}, "uppercase": eval.Bool(true),
		}}
	painterFor("Text")(dc, th, n, &PainterDeps{CursorAlpha: 1})
	if !containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Errorf("uppercased text not drawn")
	}
}

func TestPaintMeterTicks(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 120, 20)
	n := &eval.ResolvedNode{Name: "Meter", Rect: eval.RectF{X: 10, Y: 10, W: 100, H: 7},
		Props: map[string]eval.Value{"ratio": eval.Num(0.5), "variant": eval.Kw("ticks"), "ticks": eval.Num(10)}}
	painterFor("Meter")(dc, th, n, &PainterDeps{})
	if !anyColor(img) {
		t.Errorf("meter ticks not drawn")
	}
}

func TestPaintListClipsChildrenToViewport(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 200, 200)
	dc.ClearWithColor(gg.RGBA{})

	// List at (20, 20, 50, 50). Child Box spans (0, 0, 200, 200) with solid red fill.
	child := &eval.ResolvedNode{
		Name: "Box", Rect: eval.RectF{X: 0, Y: 0, W: 200, H: 200},
		Props: map[string]eval.Value{"fill": eval.Color{R: 0xFF, G: 0, B: 0, A: 0xFF}},
	}
	list := &eval.ResolvedNode{
		Name: "List", Rect: eval.RectF{X: 20, Y: 20, W: 50, H: 50},
		Props:    map[string]eval.Value{"overflow": eval.Kw("clamp")},
		Children: []*eval.ResolvedNode{child},
	}

	painterFor("List")(dc, th, list, &PainterDeps{})

	// Pixel at (10, 10) is outside the List. It MUST be transparent.
	pxOutside := img.RGBAAt(10, 10)
	if pxOutside.A != 0 || pxOutside.R != 0 {
		t.Errorf("pixel outside List viewport was painted: %+v, want transparent", pxOutside)
	}

	// Pixel at (30, 30) is inside the List. It MUST be red.
	pxInside := img.RGBAAt(30, 30)
	if pxInside.R != 0xFF || pxInside.A != 0xFF {
		t.Errorf("pixel inside List viewport was not painted: %+v, want red", pxInside)
	}
}

func TestPaintTextTruncatesWithEllipsis(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 200, 40)
	dc.ClearWithColor(gg.RGBA{})

	// Node constrained to width 60 at X=10. Text is very long.
	n := &eval.ResolvedNode{
		Name: "Text",
		Rect: eval.RectF{X: 10, Y: 10, W: 60, H: 20},
		Props: map[string]eval.Value{
			"text":  eval.Str("This is an extremely long string that definitely exceeds 60 pixels width"),
			"size":  eval.Num(14),
			"color": eval.Color{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		},
	}
	painterFor("Text")(dc, th, n, &PainterDeps{})

	// Check that no pixels were painted beyond X=75 (60px width + 10px X + 5px buffer for antialiasing)
	for y := 0; y < 40; y++ {
		for x := 75; x < 200; x++ {
			if img.RGBAAt(x, y).A != 0 {
				t.Fatalf("paintText painted past its allocated width at (%d, %d): %+v", x, y, img.RGBAAt(x, y))
			}
		}
	}

	// Verify that text pixels were painted inside X=10..70
	painted := false
	for y := 10; y < 30; y++ {
		for x := 10; x < 70; x++ {
			if img.RGBAAt(x, y).A != 0 {
				painted = true
				break
			}
		}
	}
	if !painted {
		t.Errorf("paintText did not paint truncated text within bounds")
	}
}

func anyColor(img *image.RGBA) bool {
	for y := 4; y < img.Bounds().Dy(); y += 4 {
		for x := 4; x < img.Bounds().Dx(); x += 4 {
			if img.RGBAAt(x, y).A != 0 {
				return true
			}
		}
	}
	return false
}
