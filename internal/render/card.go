package render

import (
	"image"
	"math"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/card"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
)

// drawCardContent paints a plan inside the axis-aligned content box, inset from
// the sheared quad the way the QML Loader clipped its delegate.
func drawCardContent(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64, deps *PainterDeps) {
	muted := th.Muted
	if selected {
		muted = th.Slab
	}

	top := y
	if p.Badge.Kind != "" || p.Status.Kind != "" {
		if p.Badge.Kind != "" {
			drawChip(dc, th, x, top, w/2, p.Badge.Text, p.Badge.Tone, selected)
		}
		if p.Status.Kind != "" {
			drawChip(dc, th, x+w/2, top, w/2, p.Status.Text, p.Status.Tone, selected)
		}
		top += 18 + 6
	}

	bottom := y + h
	if text := footerText(p); text != "" {
		bottom -= 16
		face := th.Face(9)
		dc.SetFont(face)
		dc.SetColor(muted)
		dc.DrawStringAnchored(elide(face, text, w), x+w/2, bottom+8, 0.5, 0.5)
	}

	if bottom-top < 20 {
		return
	}
	switch p.Layout {
	case card.LayoutHero:
		drawHero(dc, th, p, selected, x, top, w, bottom-top, deps)
	case card.LayoutBody:
		drawBody(dc, th, p, selected, x, top, w, bottom-top)
	default:
		drawRow(dc, th, p, selected, x, top, w, bottom-top, deps)
	}
}

// footerText folds the overflow count into the hint line.
func footerText(p card.Plan) string {
	text := ""
	if p.Footer.Kind != "" {
		text = p.Footer.Text
	}
	if p.Overflow > 0 {
		marker := "+" + itoa(p.Overflow)
		if text == "" {
			return marker
		}
		return text + "  //  " + marker
	}
	return text
}

// drawHero paints the app card: a centred visual over a wrapped title.
func drawHero(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64, deps *PainterDeps) {
	size := math.Max(36, math.Min(52, h*0.40))
	drawVisual(dc, th, p.Visual, selected, x+(w-size)/2, y+4, size, size, deps)
	face := th.Face(math.Max(12, math.Min(16, h*0.14)))
	drawLines(dc, th, face, p.Title.Text, selected, x, y+size+10, w, 2, true)
}

// drawBody paints the iconless card: a centred title over its detail or
// subtitle. The title uses the same treatment as every other layout, so an
// iconless card reads like a card, not a billboard.
func drawBody(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64) {
	face := th.Face(titleSize(p.Title.Style))
	max := maxLines(p.Title.Style)
	if max < 2 {
		max = 2 // the body has the width; let the title wrap once
	}
	lines := drawLines(dc, th, face, titleText(p.Title), selected, x, y+4, w, max, true)
	next := y + 4 + float64(len(lines))*face.Metrics().LineHeight()

	if p.Detail.Kind != "" {
		drawDetail(dc, th, p.Detail.Text, selected, x, next, w, h-(next-y)-4)
		return
	}
	if p.Subtitle.Kind != "" {
		sub := th.Face(9)
		dc.SetFont(sub)
		dc.SetColor(mutedOrInk(th, selected))
		dc.DrawStringAnchored(elide(sub, strings.ToUpper(p.Subtitle.Text), w), x+w/2, next+7, 0.5, 0.5)
	}
	if len(p.Accessories) > 0 {
		drawAccessories(dc, th, p.Accessories, selected, x, y+h-accessoryHeight*float64(len(p.Accessories)), w)
	}
}

// drawRow paints the standard card: a leading visual beside a text column.
func drawRow(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64, deps *PainterDeps) {
	const box = 44.0
	drawVisual(dc, th, p.Visual, selected, x, y+(h-box)/2, box, box, deps)

	colX := x + box + 10
	colW := w - box - 10
	if colW <= 0 {
		return
	}
	size := titleSize(p.Title.Style)
	face := th.Face(size)
	lines := drawLines(dc, th, face, titleText(p.Title), selected, colX, y+2, colW, maxLines(p.Title.Style), false)
	top := y + 2 + float64(len(lines))*(size+2)

	if p.Subtitle.Kind != "" {
		sub := th.Face(9)
		dc.SetFont(sub)
		dc.SetColor(mutedOrInk(th, selected))
		dc.DrawStringAnchored(elide(sub, strings.ToUpper(p.Subtitle.Text), colW), colX, top+7, 0, 0.5)
	}
	if len(p.Accessories) > 0 {
		accTop := top + 16
		if accTop+accessoryHeight <= y+h {
			drawAccessories(dc, th, p.Accessories, selected, colX, accTop, colW)
		}
	}
}

func drawDetail(dc *gg.Context, th *theme.Theme, s string, selected bool, x, y, w, h float64) {
	face := th.Face(11)
	dc.SetFont(face)
	dc.SetColor(mutedOrInk(th, selected))
	for i, line := range dc.WordWrap(s, w) {
		if i == 3 || float64(i)*14 > h {
			break
		}
		dc.DrawStringAnchored(elide(face, line, w), x+w/2, y+float64(i)*14+7, 0.5, 0.5)
	}
}

// drawLines draws up to max wrapped lines and returns them.
func drawLines(dc *gg.Context, th *theme.Theme, face text.Face, s string, selected bool, x, y, w float64, max int, centred bool) []string {
	dc.SetFont(face)
	dc.SetColor(inkOrSlab(th, selected))
	lines := dc.WordWrap(s, w)
	if len(lines) > max {
		lines = lines[:max]
	}
	ax := 0.0
	if centred {
		ax = 0.5
	}
	lh := face.Metrics().LineHeight()
	for i, line := range lines {
		dc.DrawStringAnchored(line, x+w*ax, y+lh*(float64(i)+0.5), ax, 0.5)
	}
	return lines
}

func titleSize(s card.Style) float64 {
	if s == card.StyleHero {
		return 16
	}
	return 13
}

func titleText(p card.Part) string {
	if p.Style == card.StyleHero {
		return p.Text
	}
	return strings.ToUpper(p.Text)
}

func maxLines(s card.Style) int {
	switch s {
	case card.StyleWrap:
		return 3
	case card.StyleHero:
		return 2
	}
	return 1
}

func inkOrSlab(th *theme.Theme, selected bool) gg.RGBA {
	if selected {
		return th.Slab
	}
	return th.Ink
}

func mutedOrInk(th *theme.Theme, selected bool) gg.RGBA {
	if selected {
		return th.Slab
	}
	return th.Muted
}

// drawChip paints a small rounded chip and returns its width.
func drawChip(dc *gg.Context, th *theme.Theme, x, y, maxW float64, s string, tone backend.Tone, selected bool) float64 {
	face := th.Face(9)
	dc.SetFont(face)
	label := elide(face, strings.ToUpper(s), maxW-10)
	w := math.Min(face.Advance(label)+10, maxW)
	fill, ink := chipColours(th, tone, selected)
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, y, w, 18, 2)
	_ = dc.Fill()
	dc.SetFont(face)
	dc.SetColor(ink)
	dc.DrawStringAnchored(label, x+w/2, y+9, 0.5, 0.5)
	return w
}

// chipColours maps a tone to its fill and text colour in both states.
func chipColours(th *theme.Theme, tone backend.Tone, selected bool) (gg.RGBA, gg.RGBA) {
	if selected {
		switch tone {
		case backend.ToneAccent, backend.ToneInfo, backend.TonePositive, backend.ToneAttention:
			return th.Slab, th.AccentAlt
		case backend.ToneWarn, backend.ToneAlert:
			return th.Slab, th.Accent
		}
		return th.Slab, th.Ink
	}
	switch tone {
	case backend.ToneAccent, backend.ToneInfo, backend.TonePositive, backend.ToneAttention:
		return th.AccentAlt, th.Slab
	case backend.ToneWarn, backend.ToneAlert:
		return th.Accent, th.Slab
	}
	return th.SurfaceAlt, th.Muted
}

// drawVisual paints the leading artwork. Icons resolve through the theme
// resolver and decode off the frame thread; until the decode lands, and for
// glyphs and captures, the slot shows a monogram.
func drawVisual(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w, h float64, deps *PainterDeps) {
	if p.Kind == "" {
		return
	}
	if p.Source == "glyph" {
		face := th.Face(math.Min(28, h*0.6))
		dc.SetFont(face)
		dc.SetColor(inkOrSlab(th, selected))
		dc.DrawStringAnchored(p.Text, x+w/2, y+h/2, 0.5, 0.5)
		return
	}

	if img, ok := visualImage(deps, p, int(w), int(h)); ok {
		drawVisualImage(dc, img, x, y, w, h)
		return
	}

	fill, border := th.SurfaceAlt, th.AccentAlt
	if selected {
		fill, border = th.Slab, th.Slab
	}
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, y, w, h, 4)
	_ = dc.Fill()
	dc.NewSubPath()
	dc.SetColor(border)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(x, y, w, h, 4)
	_ = dc.Stroke()

	face := th.Face(math.Min(22, h*0.5))
	dc.SetFont(face)
	dc.SetColor(inkOrSlab(th, selected))
	dc.DrawStringAnchored(monogram(p.Text), x+w/2, y+h/2, 0.5, 0.5)
}

// visualImage resolves the part's artwork to a decoded image: icon names go
// through the theme resolver first; image sources are paths already. It
// reports not-ready while the decode is in flight.
func visualImage(deps *PainterDeps, p card.Part, w, h int) (image.Image, bool) {
	if deps == nil || deps.Images == nil || p.Text == "" {
		return nil, false
	}
	path := p.Text
	switch p.Source {
	case "icon":
		if deps.Icons == nil {
			return nil, false
		}
		// Icon resolution never blocks the frame: a cold name reports
		// not-ready and resolves in the background for a later frame.
		resolved, ok, ready := deps.Icons.LookupAsync(path, w)
		if !ready || !ok {
			return nil, false
		}
		path = resolved
	case "image":
		// path as-is
	default:
		return nil, false
	}
	return deps.Images.Get(path, w, h)
}

// drawVisualImage fits the decoded image into the slot, centred.
func drawVisualImage(dc *gg.Context, img image.Image, x, y, w, h float64) {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}
	s := math.Min(w/float64(b.Dx()), h/float64(b.Dy()))
	dw, dh := float64(b.Dx())*s, float64(b.Dy())*s
	dc.DrawImage(gg.ImageBufFromImage(img), x+(w-dw)/2, y+(h-dh)/2)
}

// monogram is the fallback mark for an icon or artwork source: its first
// letter, upper case.
func monogram(s string) string {
	for _, r := range s {
		if r != ' ' && r != '-' && r != '_' {
			return strings.ToUpper(string(r))
		}
	}
	return "?"
}

// elide trims s until it fits width, marking the cut with a full stop.
func elide(face text.Face, s string, width float64) string {
	if width <= 0 {
		return ""
	}
	if face.Advance(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		if face.Advance(string(r)+".") <= width {
			return string(r) + "."
		}
	}
	return ""
}

// accessoryHeight is the row height of one accessory plus its gap.
const accessoryHeight = 18.0

// drawAccessories paints the meter, timeline and note rows under the text
// column, one row each.
func drawAccessories(dc *gg.Context, th *theme.Theme, parts []card.Part, selected bool, x, y, w float64) {
	for _, p := range parts {
		switch p.Kind {
		case backend.PartMeter:
			drawMeter(dc, th, p, selected, x, y, w)
		case backend.PartTimeline:
			drawTimeline(dc, th, p, selected, x, y, w)
		case backend.PartNote:
			drawNote(dc, th, p, selected, x, y, w)
		}
		y += accessoryHeight
	}
}

// drawMeter paints either ten sheared ticks or a progress bar.
func drawMeter(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w float64) {
	if p.Ticks > 0 {
		drawTicks(dc, th, p.Ratio, p.Ticks, selected, x, y, w)
		return
	}
	track, fill := th.SurfaceAlt, th.Accent
	if selected {
		track, fill = th.Slab, th.AccentAlt
	}
	barY := y + 4
	dc.SetColor(track)
	dc.DrawRoundedRectangle(x, barY, w, 5, 1)
	_ = dc.Fill()
	dc.NewSubPath()
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, barY, math.Max(0, w*p.Ratio), 5, 1)
	_ = dc.Fill()
}

// drawTicks reproduces the QML meter: n bars sheared 0.25, filled by ratio.
func drawTicks(dc *gg.Context, th *theme.Theme, ratio float64, n int, selected bool, x, y, w float64) {
	if n <= 0 {
		return
	}
	gap := 2.0
	tickW := math.Max(2, (w-gap*float64(n-1))/float64(n))
	filled := math.Round(ratio * float64(n))
	on, off := th.Accent, th.SurfaceAlt
	if selected {
		on, off = th.Slab, th.SurfaceAlt
	}
	for i := 0; i < n; i++ {
		tx := x + float64(i)*(tickW+gap)
		dc.SetColor(off)
		if float64(i) < filled {
			dc.SetColor(on)
		}
		dc.Push()
		dc.Translate(tx+tickW/2, y+8)
		dc.Shear(-0.25, 0)
		dc.DrawRectangle(-tickW/2, -3.5, tickW, 7)
		_ = dc.Fill()
		dc.Pop()
	}
}

// drawTimeline paints the read-only position row with its elapsed slash total
// label on the right.
func drawTimeline(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w float64) {
	face := th.Face(8)
	dc.SetFont(face)
	label := p.Label
	if p.Value != "" {
		label = p.Label + "/" + p.Value
	}
	dc.SetColor(mutedOrInk(th, selected))
	labelW := face.Advance(label)
	dc.DrawStringAnchored(label, x+w, y+8, 1, 0.5)

	track, fill := th.SurfaceAlt, th.Accent
	if selected {
		track, fill = th.Slab, th.AccentAlt
	}
	barW := w - labelW - 6
	if barW <= 0 {
		return
	}
	dc.SetColor(track)
	dc.DrawRoundedRectangle(x, y+5.5, barW, 5, 1)
	_ = dc.Fill()
	dc.NewSubPath()
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, y+5.5, math.Max(0, barW*p.Ratio), 5, 1)
	_ = dc.Fill()
}

// drawNote paints an inline state pill such as "MUTED".
func drawNote(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w float64) {
	text := p.Text
	if text == "" {
		text = p.Label
	}
	if text == "" {
		text = "ON"
	}
	drawChip(dc, th, x, y, w, text, p.Tone, selected)
}
