package render

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
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

// TestItemToBackendPreservesControls pins the eval→backend round trip: the
// card composer reads Kind, Min/Max/Step, Label, Options, Display and
// Primary, so dropping any of them silently removes accessories from cards.
func TestItemToBackendPreservesControls(t *testing.T) {
	item := eval.Item{ID: "a", Title: "Speakers", Controls: []eval.ItemControl{{
		ID: "volume", Kind: "range", Label: "Volume", Ratio: 0.65,
		Min: 0, Max: 1, Step: 0.05, Primary: true,
	}}}
	got := itemToBackend(&item)
	if len(got.Controls) != 1 {
		t.Fatalf("controls = %d, want 1", len(got.Controls))
	}
	c := got.Controls[0]
	if c.Kind != backend.ControlRange {
		t.Errorf("Kind = %q, want range", c.Kind)
	}
	if c.Label != "Volume" {
		t.Errorf("Label = %q, want Volume", c.Label)
	}
	if c.Value != 0.65 || c.Min != 0 || c.Max != 1 || c.Step != 0.05 {
		t.Errorf("range fields = %v/%v/%v step %v, want 0.65/0/1 step 0.05", c.Value, c.Min, c.Max, c.Step)
	}
	if !c.Primary {
		t.Error("Primary dropped")
	}
}

// TestCardPainterDrawsControlAccessories proves a range control on a scene
// item reaches the card as a painted meter.
func TestCardPainterDrawsControlAccessories(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	dc, img := testContext(t, 400, 260)
	item := eval.Item{ID: "a", Title: "Speakers", Controls: []eval.ItemControl{{
		ID: "volume", Kind: "range", Label: "Volume", Ratio: 0.5, Min: 0, Max: 1, Primary: true,
	}}}
	n := &eval.ResolvedNode{Name: "Card", Rect: eval.RectF{X: 10, Y: 10, W: 340, H: 220},
		Props: map[string]eval.Value{"item": eval.ItemVal{I: &item}}}
	paintCard(dc, th, n, &PainterDeps{CardCfg: config.Default().Reveal})
	if !containsColor(img, 0xFF, 0x2E, 0x63) {
		t.Error("accent meter pixels missing: control kind lost in round trip")
	}
}
