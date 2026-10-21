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
			Props: map[string]eval.Value{"fill": eval.Color{0xFF, 0, 0, 0xFF}}}},
		{"Path", &eval.ResolvedNode{Name: "Path", Rect: eval.RectF{X: 0, Y: 0, W: 200, H: 100},
			Props: map[string]eval.Value{"d": eval.Str("M 0 0 L 200 0 L 200 100 L 0 100 Z"), "fill": eval.Color{0, 0, 0xFF, 0xFF}}}},
		{"Cursor", &eval.ResolvedNode{Name: "Cursor", Rect: eval.RectF{X: 60, Y: 10, W: 3, H: 24},
			Props: map[string]eval.Value{"color": eval.Color{0xFF, 0x2E, 0x63, 0xFF}}}},
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
			"color": eval.Color{0xF2, 0xF2, 0xF7, 0xFF}, "uppercase": eval.Bool(true),
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
