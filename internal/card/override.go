package card

import "github.com/darkliquid/keyshell/internal/backend"

// composeOverride renders an explicit part list. It replaces the derived body
// wholesale; the header and footer still follow the derive rules, because every
// card has them.
func composeOverride(item backend.Item) Plan {
	p := Plan{
		Badge:  chip(backend.PartBadge, item.Badge, backend.ToneAccent),
		Status: chip(backend.PartStatus, item.Status, item.StatusTone),
		Footer: footer(item),
	}

	index := make(map[string]int, len(item.Controls))
	for i, c := range item.Controls {
		index[c.ID] = i
	}

	for _, cp := range item.Card {
		part := Part{Kind: cp.Kind, Control: -1, Text: cp.Text, Tone: cp.Tone}
		if cp.Control != "" {
			i, ok := index[cp.Control]
			if !ok {
				continue
			}
			c := item.Controls[i]
			part.Control = i
			part.Ratio = ratio(c)
			if part.Text == "" {
				part.Text = c.Text
			}
			if part.Label == "" {
				part.Label = c.Label
			}
			if part.Tone == "" {
				part.Tone = backend.ToneNeutral
			}
			if cp.Kind == backend.PartMeter && ratioLike(c) {
				part.Ticks = 10
			}
		}
		switch part.Kind {
		case backend.PartBadge:
			p.Badge = part
		case backend.PartStatus:
			p.Status = part
		case backend.PartVisual:
			p.Visual = part
		case backend.PartTitle:
			p.Title = part
		case backend.PartSubtitle:
			p.Subtitle = part
		case backend.PartDetail:
			p.Detail = part
		case backend.PartMeter, backend.PartTimeline, backend.PartNote:
			if len(p.Accessories) < AccessoryRows {
				p.Accessories = append(p.Accessories, part)
			} else {
				p.Overflow++
			}
		case backend.PartHold:
			p.Hold = part
		case backend.PartFooter:
			p.Footer = part
		}
	}
	if p.Title.Kind == "" {
		p.Title = Part{Kind: backend.PartTitle, Control: -1, Text: item.Title}
	}
	p.Layout = layoutFor(item, p)
	p.Title.Style = titleStyle(item, p)
	if p.Layout == LayoutHero {
		p.Footer = Part{}
	}
	return p
}
