package render

import (
	"image"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// paintImage draws the Image or Icon node: the cached bitmap, glyph, or fallback mark,
// fitted per `fit` and tinted per `tint`. Named icons resolve through the icon theme.
func paintImage(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	r := n.Rect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if deps != nil && deps.ClipRect != nil {
		cr := deps.ClipRect
		if r.X+r.W <= cr.X || r.X >= cr.X+cr.W || r.Y+r.H <= cr.Y || r.Y >= cr.Y+cr.H {
			return
		}
	}
	srcRaw, ok := n.Props["source"].(eval.Str)
	if !ok {
		srcRaw, ok = n.Props["name"].(eval.Str)
	}
	if !ok {
		srcRaw, ok = n.Props["icon"].(eval.Str)
	}
	path := ""
	if ok {
		path = string(srcRaw)
	}

	glyphRaw, hasGlyph := n.Props["glyph"].(eval.Str)
	glyph := ""
	if hasGlyph {
		glyph = string(glyphRaw)
	}

	if path == "" && glyph != "" {
		if utf8.RuneCountInString(glyph) <= 2 {
			drawGlyph(dc, th, n, glyph)
			return
		}
		path = mapGlyphToIcon(glyph)
	}

	if path == "" {
		return
	}

	// If source is not an absolute file path and an icon resolver exists, resolve icon name.
	if deps != nil && deps.Icons != nil && !strings.HasPrefix(path, "/") {
		resolved, ok, ready := deps.Icons.LookupAsync(path, int(r.W))
		if ready && ok {
			path = resolved
		} else if !ready || !ok {
			if glyph != "" && utf8.RuneCountInString(glyph) <= 2 {
				drawGlyph(dc, th, n, glyph)
			} else {
				drawFallbackMark(dc, th, n, path)
			}
			return
		}
	}

	if deps == nil || deps.Images == nil {
		if glyph != "" && utf8.RuneCountInString(glyph) <= 2 {
			drawGlyph(dc, th, n, glyph)
		} else {
			drawFallbackMark(dc, th, n, path)
		}
		return
	}
	img, ready := deps.Images.Get(path, int(r.W), int(r.H))
	if !ready {
		if glyph != "" && utf8.RuneCountInString(glyph) <= 2 {
			drawGlyph(dc, th, n, glyph)
		} else {
			drawFallbackMark(dc, th, n, path)
		}
		return
	}
	fit := kwVal(n.Props["fit"])
	var dx, dy, dw, dh float64
	switch fit {
	case "contain":
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		s := minf(r.W/float64(iw), r.H/float64(ih))
		dw, dh = float64(iw)*s, float64(ih)*s
		dx, dy = r.X+(r.W-dw)/2, r.Y+(r.H-dh)/2
	case "fill", "":
		dx, dy, dw, dh = r.X, r.Y, r.W, r.H
	default: // cover
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		s := maxf(r.W/float64(iw), r.H/float64(ih))
		dw, dh = float64(iw)*s, float64(ih)*s
		dx, dy = r.X+(r.W-dw)/2, r.Y+(r.H-dh)/2
	}
	ib := gg.ImageBufFromImage(img)
	dc.DrawImage(ib, dx, dy)

	if tint, ok := n.Props["tint"].(eval.Color); ok {
		tintRect(dc, image.Rect(int(dx), int(dy), int(dx+dw), int(dy+dh)), tint)
	}
}

func drawFallbackMark(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, text string) {
	r := n.Rect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	radius := 4.0
	if v, ok := n.Props["radius"].(eval.Num); ok {
		radius = float64(v)
	}
	selected := n.Selected || selectedProp(n)
	fill, border := th.SurfaceAlt, th.AccentAlt
	if selected {
		fill, border = th.Slab, th.Slab
	}
	if v, ok := n.Props["fill"]; ok {
		fill = evalColorToGG(v, fill)
	}
	dc.SetColor(fill)
	if radius > 0 {
		dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
	} else {
		dc.DrawRectangle(r.X, r.Y, r.W, r.H)
	}
	_ = dc.Fill()
	dc.NewSubPath()
	dc.SetColor(border)
	dc.SetLineWidth(1.5)
	if radius > 0 {
		dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
	} else {
		dc.DrawRectangle(r.X, r.Y, r.W, r.H)
	}
	_ = dc.Stroke()

	face := th.Face(math.Min(22, math.Max(8, r.H*0.5)))
	dc.SetFont(face)
	dc.SetColor(inkOrSlab(th, selected))
	dc.DrawStringAnchored(monogram(text), r.X+r.W/2, r.Y+r.H/2, 0.5, 0.5)
}

func drawGlyph(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, glyph string) {
	r := n.Rect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if v, ok := n.Props["fill"]; ok {
		fill := evalColorToGG(v, th.Slab)
		radius := 0.0
		if rv, ok := n.Props["radius"].(eval.Num); ok {
			radius = float64(rv)
		}
		dc.SetColor(fill)
		if radius > 0 {
			dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
		} else {
			dc.DrawRectangle(r.X, r.Y, r.W, r.H)
		}
		_ = dc.Fill()
	}
	size := math.Min(r.W, r.H) * 0.7
	if sv, ok := n.Props["size"].(eval.Num); ok && sv > 0 {
		size = float64(sv)
	}
	selected := n.Selected || selectedProp(n)
	col := inkOrSlab(th, selected)
	if cv, ok := n.Props["color"]; ok {
		col = evalColorToGG(cv, col)
	}
	dc.SetColor(col)

	if glyph == "▶" {
		triW := size * 0.65
		triH := size * 0.75
		x0 := r.X + (r.W-triW)/2
		y0 := r.Y + (r.H-triH)/2
		dc.NewSubPath()
		dc.MoveTo(x0, y0)
		dc.LineTo(x0+triW, y0+triH/2)
		dc.LineTo(x0, y0+triH)
		dc.ClosePath()
		_ = dc.Fill()
		return
	}

	if glyph == "⚙" {
		cx := r.X + r.W/2
		cy := r.Y + r.H/2
		rad := size * 0.45
		dc.DrawCircle(cx, cy, rad)
		_ = dc.Fill()
		dc.SetColor(th.Slab)
		dc.DrawCircle(cx, cy, rad*0.4)
		_ = dc.Fill()
		dc.SetColor(col)
		return
	}

	face := th.Face(size)
	dc.SetFont(face)
	dc.DrawStringAnchored(glyph, r.X+r.W/2, r.Y+r.H/2, 0.5, 0.5)
}

// mapGlyphToIcon maps common backend glyph identifiers to standard desktop icon names.
func mapGlyphToIcon(name string) string {
	switch name {
	case "command":
		return "utilities-terminal"
	case "power":
		return "system-shutdown"
	case "file":
		return "text-x-generic"
	case "folder":
		return "folder"
	case "window":
		return "preferences-system-windows"
	case "media":
		return "multimedia-audio-player"
	case "menu":
		return "open-menu"
	case "tray":
		return "preferences-system"
	default:
		return name
	}
}

// tintRect applies a tint colour over the region.
func tintRect(dc *gg.Context, r image.Rectangle, c eval.Color) {
	dc.SetColor(gg.RGBA{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255, A: float64(c.A) / 255 * 0.4})
	dc.DrawRectangle(float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()))
	_ = dc.Fill()
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// applyEffects mutates the painter's colour state for a node's effect
// props. Gradients replace the fill; shadow draws a silhouette; blur is
// an identity fallback per spec §3.
func applyEffects(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	r := n.Rect
	for key, v := range n.Props {
		if key == "fill" {
			// gradient fills live in the fill prop
		}
		fv, ok := v.(eval.FuncVal)
		if !ok {
			continue
		}
		switch fv.Name {
		case "linear-gradient":
			if len(fv.Args) >= 2 {
				paintLinearGradient(dc, r, colorOf(fv.Args[0]), colorOf(fv.Args[1]))
			}
		case "shadow":
			// draw the node's silhouette offset before the fill
			dc.SetColor(gg.RGBA{R: 0, G: 0, B: 0, A: 0.4})
			dc.DrawRectangle(r.X+4, r.Y+4, r.W, r.H)
			_ = dc.Fill()
		}
	}
}

func colorOf(v eval.Value) gg.RGBA {
	if c, ok := v.(eval.Color); ok {
		return gg.RGBA{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255, A: float64(c.A) / 255}
	}
	return gg.RGBA{}
}

// paintLinearGradient fills a rect with a vertical ramp from top to
// bottom (a software fallback for the GPU gradient).
func paintLinearGradient(dc *gg.Context, r eval.RectF, from, to gg.RGBA) {
	rows := int(r.H)
	if rows < 2 {
		rows = 2
	}
	for i := 0; i < rows; i++ {
		t := float64(i) / float64(rows-1)
		c := gg.RGBA{
			R: from.R + (to.R-from.R)*t,
			G: from.G + (to.G-from.G)*t,
			B: from.B + (to.B-from.B)*t,
			A: from.A + (to.A-from.A)*t,
		}
		y := r.Y + float64(i)
		dc.SetColor(c)
		dc.DrawRectangle(r.X, y, r.W, 1)
		_ = dc.Fill()
	}
}
