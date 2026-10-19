// Package render draws the overlay with gg into a buffer, unpremultiplies it,
// and presents it on layer surfaces via gogpu. It owns the visual vocabulary:
// the tilted band, sheared cards with hard offset shadows, thick accent borders,
// and the display typography. The state comes from the launcher; the drawing is
// stateless per frame.
package render

import (
	"image"

	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// View is the slice of launcher state the renderer reads.
type View interface {
	Results() []rank.Match
	Selected() int
	Query() string
	IsOpen() bool
	ModeBadge() string
}

const (
	cardW     = 300.0
	cardH     = 210.0
	gap       = 26.0
	bandY     = 96.0
	bandH     = 720.0
	shear     = 0.16 // card lean
	bandShear = 0.10 // band edges
)

// DrawFrame paints the current launcher state into a gg context. When the
// launcher is closed the frame is fully transparent, so the mapped overlay
// surface shows the desktop through it.
func DrawFrame(dc *gg.Context, l View, th *theme.Theme) {
	if !l.IsOpen() {
		dc.ClearWithColor(gg.RGBA{})
		return
	}
	dc.ClearWithColor(th.Slab)

	drawBand(dc, th)

	drawHeader(dc, th, l.ModeBadge(), l.Query())

	// Reel of cards.
	results := l.Results()
	selected := l.Selected()
	offset := 70.0
	for i, m := range results {
		if i >= 12 {
			break
		}
		x := offset + float64(i)*(cardW+gap)
		if x > 1500 {
			break
		}
		drawCard(dc, th, x, bandY+60, m, i == selected)
	}

	drawStatusLine(dc, th, l.Query())
}

// drawBand draws the tilted band: a trapezoid container with a hard shadow and
// a cyan accent spine, giving the overlay its arcade frame.
func drawBand(dc *gg.Context, th *theme.Theme) {
	w := 1700.0
	shearPx := bandH * bandShear
	left := (1920.0 - w) / 2
	top := bandY
	bottom := top + bandH

	// Hard offset shadow first (solid, zero blur).
	dc.SetColor(gg.RGBA{R: 0, G: 0, B: 0, A: 0.45})
	dc.NewSubPath()
	dc.MoveTo(left+14, top+14)
	dc.LineTo(left+w+shearPx+14, top+14)
	dc.LineTo(left+w+shearPx+14, bottom+14)
	dc.LineTo(left+14, bottom+14)
	dc.ClosePath()
	_ = dc.Fill()

	// The band surface, sheared on the right edge.
	dc.SetColor(th.SurfaceAlt)
	dc.NewSubPath()
	dc.MoveTo(left, top)
	dc.LineTo(left+w+shearPx, top)
	dc.LineTo(left+w+shearPx, bottom)
	dc.LineTo(left, bottom)
	dc.ClosePath()
	_ = dc.Fill()

	// Cyan accent spine on the left edge.
	dc.SetColor(th.AccentAlt)
	dc.DrawRectangle(left, top, 6, bandH)
	_ = dc.Fill()

	// Top "band" bar.
	dc.SetColor(th.Accent)
	dc.DrawRectangle(left-20, top-12, 40, 8)
	_ = dc.Fill()
}

// drawCard draws one card with a solid offset shadow, a sheared face, a thick
// border, and the display typography. The selected card gets the accent border
// and a highlighted title.
func drawCard(dc *gg.Context, th *theme.Theme, x, y float64, m rank.Match, selected bool) {
	// Sheared card geometry via a quad.
	skew := shear * cardH

	// Hard offset shadow.
	dc.SetColor(gg.RGBA{R: 0, G: 0, B: 0, A: 0.5})
	quad(dc, x+12, y+12, skew, cardW, cardH, th.SurfaceAlt)

	// Card face.
	quadFill := th.SurfaceAlt
	if selected {
		quadFill = gg.RGBA{R: 0x1A, G: 0x1A, B: 0x22, A: 1}
	}
	quad(dc, x, y, skew, cardW, cardH, quadFill)

	// Border: 3px accent for selected, muted otherwise.
	quadStroke := th.Muted
	if selected {
		quadStroke = th.Accent
	}
	dc.SetColor(quadStroke)
	dc.SetLineWidth(3)
	strokeQuad(dc, x, y, skew, cardW, cardH)

	// Glyph (emoji or icon name) if present.
	if m.Item.Glyph != "" {
		glyphFace := th.Face(40)
		dc.SetFont(glyphFace)
		dc.SetColor(th.AccentAlt)
		dc.DrawString(m.Item.Glyph, x+16, y+110)
	}

	// Title in the display face.
	titleFace := th.Face(30)
	dc.SetFont(titleFace)
	if selected {
		dc.SetColor(th.AccentAlt)
	} else {
		dc.SetColor(th.Ink)
	}
	dc.DrawString(clip(m.Item.Title, 16), x+18, y+48)

	// Subtitle.
	subFace := th.Face(15)
	dc.SetFont(subFace)
	dc.SetColor(th.Muted)
	dc.DrawString(clip(m.Item.Subtitle, 30), x+18, y+74)

	// Selected chevron.
	if selected {
		dc.SetFont(th.Face(24))
		dc.SetColor(th.Accent)
		dc.DrawString("▶", x+cardW-34, y+48)
	}
}

// quad fills a parallelogram: top edge shifted right by skew.
func quad(dc *gg.Context, x, y, skew, w, h float64, col gg.RGBA) {
	dc.SetColor(col)
	dc.NewSubPath()
	dc.MoveTo(x, y)
	dc.LineTo(x+w+skew, y)
	dc.LineTo(x+w+skew, y+h)
	dc.LineTo(x, y+h)
	dc.ClosePath()
	_ = dc.Fill()
}

func strokeQuad(dc *gg.Context, x, y, skew, w, h float64) {
	dc.NewSubPath()
	dc.MoveTo(x, y)
	dc.LineTo(x+w+skew, y)
	dc.LineTo(x+w+skew, y+h)
	dc.LineTo(x, y+h)
	dc.ClosePath()
	_ = dc.Stroke()
}

// drawHeader draws the mode badge and the query in the top bar.
func drawHeader(dc *gg.Context, th *theme.Theme, badge, query string) {
	if badge == "" {
		badge = "SEARCH"
	}
	// Badge plate.
	dc.SetColor(th.SurfaceAlt)
	dc.DrawRectangle(70, 36, 190, 40)
	_ = dc.Fill()
	dc.SetColor(th.Accent)
	dc.SetLineWidth(3)
	dc.DrawRectangle(70, 36, 190, 40)
	_ = dc.Stroke()

	badgeFace := th.Face(26)
	dc.SetFont(badgeFace)
	dc.SetColor(th.Accent)
	dc.DrawString(badge, 88, 66)

	// Query text.
	queryFace := th.Face(26)
	dc.SetFont(queryFace)
	dc.SetColor(th.Ink)
	prefix := "⌕ "
	dc.DrawString(prefix+query, 290, 66)
}

func drawStatusLine(dc *gg.Context, th *theme.Theme, query string) {
	face := th.Face(18)
	dc.SetFont(face)
	dc.SetColor(th.Muted)
	text := "Enter: open   Tab: mode   Escape: dismiss"
	if query != "" {
		text = "⌕ " + query
	}
	dc.DrawString(text, 70, 948)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// unpremultiply converts a premultiplied RGBA image (Go's image.RGBA
// convention, which gg produces) into straight alpha, because the upload path
// treats data as straight and would otherwise multiply alpha in twice.
func unpremultiply(img *image.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			i := img.PixOffset(x, y)
			a := img.Pix[i+3]
			if a == 0 || a == 255 {
				copy(out.Pix[i:i+4], img.Pix[i:i+4])
				continue
			}
			out.Pix[i] = uint8(min(255, int(img.Pix[i])*255/int(a)))
			out.Pix[i+1] = uint8(min(255, int(img.Pix[i+1])*255/int(a)))
			out.Pix[i+2] = uint8(min(255, int(img.Pix[i+2])*255/int(a)))
			out.Pix[i+3] = a
		}
	}
	return out
}
