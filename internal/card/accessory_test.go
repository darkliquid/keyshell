package card

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// TestAccessoriesShowVolumeAndMuteForASpeaker checks that a primary meter and a
// true toggle both fit in the two-row budget, ranked primary first.
func TestAccessoriesShowVolumeAndMuteForASpeaker(t *testing.T) {
	p := Compose(backend.Item{Title: "Speakers", Icon: "audio", Controls: []backend.Control{
		{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.35, Min: 0, Max: 1, Step: 0.01, Primary: true},
		{ID: "mute", Kind: backend.ControlToggle, Label: "Mute", Bool: true},
	}})
	if len(p.Accessories) != 2 {
		t.Fatalf("accessories = %+v, want 2", p.Accessories)
	}
	if p.Accessories[0].Kind != backend.PartMeter || p.Accessories[0].Control != 0 {
		t.Errorf("first accessory = %+v, want the volume meter", p.Accessories[0])
	}
	if p.Accessories[0].Ticks != 10 {
		t.Errorf("ticks = %d, want 10 for a ratio control", p.Accessories[0].Ticks)
	}
	if p.Accessories[1].Kind != backend.PartNote || p.Accessories[1].Control != 1 {
		t.Errorf("second accessory = %+v, want the mute note", p.Accessories[1])
	}
	if p.Overflow != 0 {
		t.Errorf("overflow = %d, want 0", p.Overflow)
	}
}

// TestAccessoriesDropAFalseToggle keeps silence for healthy state.
func TestAccessoriesDropAFalseToggle(t *testing.T) {
	p := Compose(backend.Item{Title: "Speakers", Controls: []backend.Control{
		{ID: "mute", Kind: backend.ControlToggle, Bool: false},
	}})
	if len(p.Accessories) != 0 || p.Overflow != 0 {
		t.Errorf("accessories = %+v overflow = %d, want none", p.Accessories, p.Overflow)
	}
}

// TestAccessoriesCapAtTwoAndCountTheRest checks the budget and the marker count.
func TestAccessoriesCapAtTwoAndCountTheRest(t *testing.T) {
	p := Compose(backend.Item{Title: "Song", Controls: []backend.Control{
		{ID: "position", Kind: backend.ControlProgress, Label: "1:23", Text: "4:56"},
		{ID: "volume", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1, Primary: true},
		{ID: "mute", Kind: backend.ControlToggle, Bool: true},
	}})
	if len(p.Accessories) != AccessoryRows {
		t.Fatalf("accessories = %+v, want %d", p.Accessories, AccessoryRows)
	}
	if p.Overflow != 1 {
		t.Errorf("overflow = %d, want 1", p.Overflow)
	}
}

// TestAccessoriesRankAnomalyAbovePlain checks the middle rank.
func TestAccessoriesRankAnomalyAbovePlain(t *testing.T) {
	p := Compose(backend.Item{Title: "Battery", Controls: []backend.Control{
		{ID: "level", Kind: backend.ControlRange, Value: 0.1, Min: 0, Max: 1},
		{ID: "profile", Kind: backend.ControlRange, Value: 0.9, Min: 0, Max: 1},
	}})
	if len(p.Accessories) != 2 {
		t.Fatalf("accessories = %+v, want 2", p.Accessories)
	}
	if p.Accessories[0].Control != 0 {
		t.Errorf("first accessory control = %d, want the anomalous level (0)", p.Accessories[0].Control)
	}
}

// TestAccessoriesHonourPinsAndExpanded checks both explicit hints.
func TestAccessoriesHonourPinsAndExpanded(t *testing.T) {
	pinned := Compose(backend.Item{Title: "T", Controls: []backend.Control{
		{ID: "a", Kind: backend.ControlRange, Value: 0.9, Min: 0, Max: 1},
		{ID: "b", Kind: backend.ControlToggle, Bool: true, Display: backend.DisplayCard},
		{ID: "c", Kind: backend.ControlRange, Value: 0.9, Min: 0, Max: 1},
	}})
	if len(pinned.Accessories) == 0 || pinned.Accessories[0].Control != 1 {
		t.Errorf("accessories = %+v, want the pinned control first", pinned.Accessories)
	}

	hidden := Compose(backend.Item{Title: "T", Controls: []backend.Control{
		{ID: "a", Kind: backend.ControlToggle, Bool: true, Display: backend.DisplayExpanded, Primary: true},
	}})
	if len(hidden.Accessories) != 0 || hidden.Overflow != 0 {
		t.Errorf("accessories = %+v overflow = %d, want none", hidden.Accessories, hidden.Overflow)
	}
}

// TestAccessoriesNeverRenderTextOrUnpinnedChoice checks the eligibility filter.
func TestAccessoriesNeverRenderTextOrUnpinnedChoice(t *testing.T) {
	p := Compose(backend.Item{Title: "Wi-Fi", Controls: []backend.Control{
		{ID: "passphrase", Kind: backend.ControlText, Display: backend.DisplayCard, Primary: true},
		{ID: "wifi", Kind: backend.ControlChoice, Options: []string{"a", "b"}},
	}})
	if len(p.Accessories) != 0 {
		t.Errorf("accessories = %+v, want none", p.Accessories)
	}
}

// TestAccessoriesUseABarForWideRanges checks the tick rule boundary.
func TestAccessoriesUseABarForWideRanges(t *testing.T) {
	p := Compose(backend.Item{Title: "T", Controls: []backend.Control{
		{ID: "battery", Kind: backend.ControlRange, Value: 65, Min: 0, Max: 100, Step: 1, Primary: true},
	}})
	if len(p.Accessories) != 1 {
		t.Fatalf("accessories = %+v, want 1", p.Accessories)
	}
	if p.Accessories[0].Ticks != 0 {
		t.Errorf("ticks = %d, want 0 (bar) for a 0..100 range", p.Accessories[0].Ticks)
	}
	if got := p.Accessories[0].Ratio; got < 0.64 || got > 0.66 {
		t.Errorf("ratio = %v, want 0.65", got)
	}
}
