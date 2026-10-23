// Package card composes the generic result card from item data. It is pure:
// no theme, no measurement and no drawing, so the budget rules are testable
// without a graphics context.
package card

import "github.com/darkliquid/keyshell/internal/backend"

// Layout is the card arrangement derived from the item's parts.
type Layout string

const (
	LayoutHero Layout = "hero"
	LayoutRow  Layout = "row"
	LayoutBody Layout = "body"
)

// Style selects the title treatment.
type Style string

const (
	StyleHero Style = "hero"
	StyleLine Style = "line"
	StyleWrap Style = "wrap"
)

// Part is one resolved card widget. A zero Kind means the part is absent.
type Part struct {
	Kind    backend.PartKind
	Control int
	Text    string
	Tone    backend.Tone
	Style   Style
	Source  string
	Ratio   float64
	Ticks   int
	Label   string
	Value   string
}

// Plan is the composed card: what to draw, in which zones.
type Plan struct {
	Layout      Layout
	Badge       Part
	Status      Part
	Visual      Part
	Title       Part
	Subtitle    Part
	Detail      Part
	Accessories []Part
	Hold        Part
	Footer      Part
	Overflow    int
}

// Compose derives a card from item data.
func Compose(item backend.Item) Plan {
	if len(item.Card) > 0 {
		return composeOverride(item)
	}
	p := Plan{
		Badge:    chip(backend.PartBadge, item.Badge, backend.ToneAccent),
		Status:   chip(backend.PartStatus, item.Status, item.StatusTone),
		Visual:   visual(item),
		Title:    Part{Kind: backend.PartTitle, Control: -1, Text: item.Title},
		Subtitle: chip(backend.PartSubtitle, item.Subtitle, ""),
		Footer:   footer(item),
	}
	if item.Detail != "" && p.Visual.Kind == "" {
		p.Detail = Part{Kind: backend.PartDetail, Control: -1, Text: item.Detail}
	}
	p.Accessories, p.Overflow = accessories(item)
	p.Layout = layoutFor(item, p)
	p.Title.Style = titleStyle(item, p)
	if p.Layout == LayoutHero {
		p.Footer = Part{}
	}
	return p
}

// chip builds a text chip part. An empty string means no part.
func chip(kind backend.PartKind, text string, tone backend.Tone) Part {
	if text == "" {
		return Part{}
	}
	if tone == "" {
		tone = backend.ToneNeutral
	}
	return Part{Kind: kind, Control: -1, Text: text, Tone: tone}
}

// visual picks the leading artwork, most specific source first.
func visual(item backend.Item) Part {
	switch {
	case item.Image != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Image, Source: "image"}
	case item.Preview != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Preview, Source: "preview"}
	case item.Icon != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Icon, Source: "icon"}
	case item.Glyph != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Glyph, Source: "glyph"}
	}
	return Part{}
}

// layoutFor chooses the arrangement from what the card actually has.
func layoutFor(item backend.Item, p Plan) Layout {
	if p.Visual.Kind != "" && p.Badge.Kind == "" && p.Status.Kind == "" &&
		p.Subtitle.Kind == "" && p.Detail.Kind == "" && len(p.Accessories) == 0 &&
		len(item.Controls) == 0 && !item.HasChildren && launches(item) {
		return LayoutHero
	}
	if p.Visual.Kind == "" && (p.Subtitle.Kind != "" || p.Detail.Kind != "") {
		return LayoutBody
	}
	return LayoutRow
}

// launches reports whether the item's primary action starts something, which is
// what makes a card a hero card rather than a result with a leading mark.
func launches(item backend.Item) bool {
	if len(item.Actions) == 0 {
		return false
	}
	switch item.Actions[0].Type {
	case "exec", "exec-terminal", "open-url":
		return true
	}
	return false
}

// titleStyle picks the title treatment for the chosen layout. Every layout
// shares the line/wrap treatment so an iconless card reads like any other;
// only hero launch tiles get the larger display size.
func titleStyle(item backend.Item, p Plan) Style {
	if p.Layout == LayoutHero {
		return StyleHero
	}
	if p.Badge.Kind == "" && p.Status.Kind == "" && len(item.Controls) == 0 {
		return StyleWrap
	}
	return StyleLine
}

// footer is the hint line: the item's own hint, else one derived from the
// primary action.
func footer(item backend.Item) Part {
	if item.Hint != "" {
		return Part{Kind: backend.PartFooter, Control: -1, Text: item.Hint}
	}
	if len(item.Actions) == 0 {
		return Part{}
	}
	return Part{Kind: backend.PartFooter, Control: -1, Text: defaultHint(item.Actions[0], item.Destructive)}
}

func defaultHint(a backend.Action, destructive bool) string {
	if destructive {
		return "HOLD ENTER"
	}
	switch a.Type {
	case "copy":
		return "ENTER COPY"
	case "exec", "exec-terminal":
		return "ENTER RUN"
	case "open-url":
		return "ENTER OPEN"
	case "focus-window":
		return "ENTER FOCUS"
	case "plugin":
		return "ENTER ACTIVATE"
	case "set-query":
		return "ENTER SEARCH"
	}
	return "ENTER SELECT"
}
