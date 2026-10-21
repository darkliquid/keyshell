package render

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
)

func TestCardPainterDrawsAPlan(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 400, 200)
	item := eval.Item{ID: "a", Title: "Firefox", Badge: "APP"}
	n := &eval.ResolvedNode{Name: "Card", Rect: eval.RectF{X: 20, Y: 20, W: 320, H: 160},
		Props: map[string]eval.Value{"item": eval.ItemVal{I: &item}}}
	paintCard(dc, th, n, &PainterDeps{CardCfg: config.Default().Reveal})
	if !anyColor(img) {
		t.Errorf("card body unpainted")
	}
}

func TestCardPainterSelected(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 400, 200)
	item := eval.Item{ID: "a", Title: "Firefox"}
	n := &eval.ResolvedNode{Name: "Card", Rect: eval.RectF{X: 20, Y: 20, W: 320, H: 160},
		Props: map[string]eval.Value{"item": eval.ItemVal{I: &item}, "selected": eval.Bool(true)}}
	paintCard(dc, th, n, &PainterDeps{CardCfg: config.Default().Reveal})
	if !anyColor(img) {
		t.Errorf("selected card unpainted")
	}
}
