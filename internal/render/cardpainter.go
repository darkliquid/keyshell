package render

import (
	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/card"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// paintCard draws a resolved Card node through the existing card
// pipeline, so a scene's template { Card { } } reproduces today's
// cards exactly.
func paintCard(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	var item *eval.Item
	if n.Item != nil {
		item = n.Item
	} else if iv, ok := n.Props["item"].(eval.ItemVal); ok {
		item = iv.I
	}
	if item == nil {
		return
	}
	plan := card.Compose(itemToBackend(item))
	selected := n.Selected || selectedProp(n)
	drawCard(dc, th, deps.Reveal, deps.CardCfg,
		n.Rect.X, n.Rect.Y, n.Rect.W, n.Rect.H, plan, selected)
}

func selectedProp(n *eval.ResolvedNode) bool {
	if v, ok := n.Props["selected"].(eval.Bool); ok {
		return bool(v)
	}
	return false
}

// itemToBackend maps an interpreter item back to a backend item for the
// card composer. Only the fields Compose reads are populated.
func itemToBackend(it *eval.Item) backend.Item {
	if it == nil {
		return backend.Item{}
	}
	controls := make([]backend.Control, 0, len(it.Controls))
	for id, c := range it.Controls {
		controls = append(controls, backend.Control{ID: id, Text: c.Value, Value: c.Ratio, Bool: c.On})
	}
	return backend.Item{
		ID: backend.ResultID(it.ID), Title: it.Title, Subtitle: it.Subtitle,
		Badge: it.Badge, Status: it.Status, StatusTone: backend.Tone(it.StatusTone),
		Glyph: it.Glyph, Icon: it.Icon, Image: it.Image, Preview: it.Preview,
		Detail: it.Detail, Hint: it.Hint, Controls: controls,
	}
}
