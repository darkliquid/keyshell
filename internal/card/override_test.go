package card

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// TestOverrideReplacesTheBody checks an explicit list drives the card and that
// the header and footer still come from the item.
func TestOverrideReplacesTheBody(t *testing.T) {
	p := Compose(backend.Item{
		Title: "Speakers", Badge: "OUTPUT", Hint: "ENTER SELECT",
		Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Value: 0.4, Min: 0, Max: 1},
		},
		Card: []backend.CardPart{
			{Kind: backend.PartTitle, Text: "SPEAKERS"},
			{Kind: backend.PartMeter, Control: "volume", Text: "VOL"},
		},
	})
	if p.Badge.Kind != backend.PartBadge || p.Badge.Text != "OUTPUT" {
		t.Errorf("badge = %+v, want the derived badge", p.Badge)
	}
	if p.Footer.Text != "ENTER SELECT" {
		t.Errorf("footer = %q, want the item hint", p.Footer.Text)
	}
	if p.Title.Text != "SPEAKERS" {
		t.Errorf("title = %q, want the override text", p.Title.Text)
	}
	if len(p.Accessories) != 1 || p.Accessories[0].Kind != backend.PartMeter {
		t.Fatalf("accessories = %+v, want the override meter", p.Accessories)
	}
	if p.Accessories[0].Control != 0 || p.Accessories[0].Ratio < 0.39 || p.Accessories[0].Ratio > 0.41 {
		t.Errorf("meter = %+v, want control 0 at ratio 0.4", p.Accessories[0])
	}
}

// TestOverrideSkipsUnknownControls keeps a stale id from blanking the card.
func TestOverrideSkipsUnknownControls(t *testing.T) {
	p := Compose(backend.Item{
		Title: "T",
		Card:  []backend.CardPart{{Kind: backend.PartMeter, Control: "gone"}},
	})
	if len(p.Accessories) != 0 {
		t.Errorf("accessories = %+v, want none", p.Accessories)
	}
	if p.Title.Kind != backend.PartTitle || p.Title.Text != "T" {
		t.Errorf("title = %+v, want the item title as the default", p.Title)
	}
}

// TestOverrideStillCapsAccessories checks the budget survives the override.
func TestOverrideStillCapsAccessories(t *testing.T) {
	p := Compose(backend.Item{
		Title: "T",
		Controls: []backend.Control{
			{ID: "a", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1},
			{ID: "b", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1},
			{ID: "c", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1},
		},
		Card: []backend.CardPart{
			{Kind: backend.PartMeter, Control: "a"},
			{Kind: backend.PartMeter, Control: "b"},
			{Kind: backend.PartMeter, Control: "c"},
		},
	})
	if len(p.Accessories) != AccessoryRows {
		t.Errorf("accessories = %+v, want %d", p.Accessories, AccessoryRows)
	}
	if p.Overflow != 1 {
		t.Errorf("overflow = %d, want 1", p.Overflow)
	}
}
