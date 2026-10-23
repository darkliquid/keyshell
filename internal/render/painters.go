package render

import (
	"math"
	"strconv"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/icons"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// Painter draws one resolved node.
type Painter func(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps)

// PainterDeps carries per-frame host state painters need.
type PainterDeps struct {
	Reveal      Reveal
	CursorAlpha float64
	CardCfg     config.Reveal
	Images      *ImageCache
	Icons       *icons.Resolver
	ClipRect    *eval.RectF
}

func painterFor(name string) Painter {
	switch name {
	case "Window", "Group", "Row", "Column", "Grid", "Stack":
		return paintThrough
	case "Box":
		return paintBox
	case "Text":
		return paintText
	case "Path":
		return paintPath
	case "Image", "Icon":
		return paintImage
	case "Pill":
		return paintPill
	case "Meter":
		return paintMeter
	case "Cursor":
		return paintCursor
	case "Card":
		return paintCard
	case "List":
		return paintList
	default:
		return paintThrough
	}
}

// paintList paints a List container. When overflow is "clamp" or "hidden" (or
// clip is explicitly true), it clips its viewport so overflowing items never
// paint outside the list container.
func paintList(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	r := n.Rect
	shouldClip := false
	if v, ok := n.Props["clip"].(eval.Bool); ok {
		shouldClip = bool(v)
	} else {
		flow := kwVal(n.Props["overflow"])
		if flow == "clamp" || flow == "hidden" {
			shouldClip = true
		}
	}

	if shouldClip && r.W > 0 && r.H > 0 {
		dc.Push()
		dc.ClipRect(r.X, r.Y, r.W, r.H)
		childDeps := *deps
		childDeps.ClipRect = &eval.RectF{X: r.X, Y: r.Y, W: r.W, H: r.H}
		paintChildren(dc, th, n, &childDeps)
		dc.Pop()
		return
	}
	paintChildren(dc, th, n, deps)
}

// paintThrough paints children in declaration order (the painter sorts
// by z within siblings; z is carried on each node).
func paintThrough(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	if n.Name == "Group" {
		dc.Push()
		r := n.Rect
		// Map the group's local frame to the surface: translate the
		// group's centre to its surface position, rotate about it.
		dc.Translate(r.X+r.W/2, r.Y+r.H/2)
		if v, ok := n.Props["rotate"].(eval.Length); ok {
			dc.Rotate(v.V * math.Pi / 180)
		}
		dc.Translate(-r.W/2, -r.H/2)
	}
	paintChildren(dc, th, n, deps)
	if n.Name == "Group" {
		dc.Pop()
	}
}

func paintChildren(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	for _, c := range n.Children {
		if deps != nil && deps.ClipRect != nil {
			cr := deps.ClipRect
			if c.Rect.W > 0 && c.Rect.H > 0 {
				if c.Rect.X+c.Rect.W <= cr.X || c.Rect.X >= cr.X+cr.W ||
					c.Rect.Y+c.Rect.H <= cr.Y || c.Rect.Y >= cr.Y+cr.H {
					continue
				}
			}
		}
		painterFor(c.Name)(dc, th, c, deps)
	}
}

func evalColorToGG(v eval.Value, def gg.RGBA) gg.RGBA {
	if c, ok := v.(eval.Color); ok {
		return gg.RGBA{R: float64(c.R) / 255, G: float64(c.G) / 255, B: float64(c.B) / 255, A: float64(c.A) / 255}
	}
	return def
}

func opacityOf(props map[string]eval.Value) float64 {
	if v, ok := props["opacity"].(eval.Num); ok {
		return float64(v)
	}
	return 1
}

func paintBox(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	applyEffects(dc, th, n, deps)
	r := n.Rect
	if deps != nil && deps.ClipRect != nil {
		cr := deps.ClipRect
		if r.X+r.W <= cr.X || r.X >= cr.X+cr.W || r.Y+r.H <= cr.Y || r.Y >= cr.Y+cr.H {
			return
		}
	}
	fill, hasFill := n.Props["fill"]
	// a FuncVal fill is an effect (gradient) already handled by applyEffects.
	effectFill := false
	if _, ok := fill.(eval.FuncVal); ok {
		effectFill = true
	}
	var col gg.RGBA
	radius := 0.0
	border := 0.0
	var borderCol gg.RGBA
	if hasFill && !effectFill {
		col = evalColorToGG(fill, th.Slab)
	}
	if v, ok := n.Props["radius"].(eval.Num); ok {
		radius = float64(v)
	}
	if v, ok := n.Props["border"].(eval.Color); ok {
		border = 1.5
		borderCol = evalColorToGG(v, th.Muted)
	}
	op := opacityOf(n.Props)
	if op <= 0 {
		return
	}
	if hasFill && !effectFill && col.A*op > 0 {
		dc.SetColor(gg.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * op})
		if radius > 0 {
			dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
		} else {
			dc.DrawRectangle(r.X, r.Y, r.W, r.H)
		}
		_ = dc.Fill()
	}
	if border > 0 {
		if radius > 0 {
			dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
		} else {
			dc.DrawRectangle(r.X, r.Y, r.W, r.H)
		}
		dc.SetLineWidth(border)
		dc.SetColor(borderCol)
		_ = dc.Stroke()
	}
	paintChildren(dc, th, n, deps)
}

func paintText(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	var rawTxt string
	switch t := n.Props["text"].(type) {
	case eval.Str:
		rawTxt = string(t)
	case eval.Num:
		rawTxt = strconv.FormatFloat(float64(t), 'g', -1, 64)
	case eval.Length:
		rawTxt = strconv.FormatFloat(t.V, 'g', -1, 64) + t.Unit
	case eval.Kw:
		rawTxt = string(t)
	default:
		return
	}
	size := 22.0
	if v, ok := n.Props["size"].(eval.Length); ok && (v.Unit == "" || v.Unit == "px") {
		size = v.V
	} else if v, ok := n.Props["size"].(eval.Num); ok {
		size = float64(v)
	}
	if v, ok := n.Props["uppercase"].(eval.Bool); ok && bool(v) {
		rawTxt = strings.ToUpper(rawTxt)
	}
	col := evalColorToGG(n.Props["color"], th.Ink)
	face := th.Face(size)
	dc.SetFont(face)
	dc.SetColor(gg.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * opacityOf(n.Props)})
	r := n.Rect
	if deps != nil && deps.ClipRect != nil {
		cr := deps.ClipRect
		if r.X+r.W <= cr.X || r.X >= cr.X+cr.W || r.Y+r.H <= cr.Y || r.Y >= cr.Y+cr.H {
			return
		}
	}
	ax := 0.0
	x := r.X
	if align, ok := n.Props["align"].(eval.Kw); ok {
		switch string(align) {
		case "center":
			ax = 0.5
			x = r.X + r.W/2
		case "end", "right":
			ax = 1.0
			x = r.X + r.W
		}
	}
	dc.DrawStringAnchored(rawTxt, x, r.Y+r.H/2, ax, 0.5)
}

func paintPath(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	d, ok := n.Props["d"].(eval.Str)
	if !ok {
		return
	}
	fill, ok := n.Props["fill"].(eval.Color)
	if !ok {
		return
	}
	parsePath(dc, string(d))
	dc.SetColor(evalColorToGG(fill, th.Slab))
	_ = dc.Fill()
}

// parsePath applies SVG-like path commands to the context.
func parsePath(dc *gg.Context, d string) {
	fs := strings.Fields(d)
	dc.NewSubPath()
	i := 0
	for i < len(fs) {
		cmd := fs[i]
		i++
		switch cmd {
		case "M", "m":
			if i+1 < len(fs) {
				x, _ := strconv.ParseFloat(fs[i], 64)
				y, _ := strconv.ParseFloat(fs[i+1], 64)
				dc.MoveTo(x, y)
				i += 2
			}
		case "L", "l":
			if i+1 < len(fs) {
				x, _ := strconv.ParseFloat(fs[i], 64)
				y, _ := strconv.ParseFloat(fs[i+1], 64)
				dc.LineTo(x, y)
				i += 2
			}
		case "Z", "z":
			dc.ClosePath()
		}
	}
}

func paintPill(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	var rawTxt string
	switch t := n.Props["text"].(type) {
	case eval.Str:
		rawTxt = string(t)
	case eval.Num:
		rawTxt = strconv.FormatFloat(float64(t), 'g', -1, 64)
	case eval.Kw:
		rawTxt = string(t)
	default:
		return
	}
	tone := ""
	if v, ok := n.Props["tone"].(eval.Kw); ok {
		tone = string(v)
	}
	fill, ink := chipColours(th, backend.Tone(tone), false)
	if tone == "accent" {
		fill = th.Accent
		ink = th.Slab
	}
	if v, ok := n.Props["fill"]; ok {
		fill = evalColorToGG(v, fill)
	}
	if v, ok := n.Props["color"]; ok {
		ink = evalColorToGG(v, ink)
	}
	radius := 2.0
	if v, ok := n.Props["radius"].(eval.Num); ok {
		radius = float64(v)
	}
	size := 12.0
	if v, ok := n.Props["size"].(eval.Num); ok {
		size = float64(v)
	}
	r := n.Rect
	if deps != nil && deps.ClipRect != nil {
		cr := deps.ClipRect
		if r.X+r.W <= cr.X || r.X >= cr.X+cr.W || r.Y+r.H <= cr.Y || r.Y >= cr.Y+cr.H {
			return
		}
	}
	face := th.Face(size)
	dc.SetFont(face)
	label := strings.ToUpper(rawTxt)
	dc.SetColor(fill)
	if radius > 0 {
		dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
	} else {
		dc.DrawRectangle(r.X, r.Y, r.W, r.H)
	}
	_ = dc.Fill()
	dc.SetColor(ink)
	dc.DrawStringAnchored(label, r.X+r.W/2, r.Y+r.H/2, 0.5, 0.5)
}

func paintMeter(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	r := n.Rect
	if deps != nil && deps.ClipRect != nil {
		cr := deps.ClipRect
		if r.X+r.W <= cr.X || r.X >= cr.X+cr.W || r.Y+r.H <= cr.Y || r.Y >= cr.Y+cr.H {
			return
		}
	}
	ratio := 0.0
	if v, ok := n.Props["ratio"].(eval.Num); ok {
		ratio = float64(v)
	}
	variant := kwVal(n.Props["variant"])
	selected := false
	switch variant {
	case "ticks":
		ticks := 10
		if v, ok := n.Props["ticks"].(eval.Num); ok {
			ticks = int(v)
		}
		drawTicks(dc, th, ratio, ticks, selected, r.X, r.Y, r.W)
	case "bar", "":
		// solid bar
		dc.SetColor(th.SurfaceAlt)
		dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, 3)
		_ = dc.Fill()
		dc.SetColor(th.Accent)
		fillW := r.W * ratio
		if fillW > 0 {
			dc.DrawRoundedRectangle(r.X, r.Y, fillW, r.H, 3)
			_ = dc.Fill()
		}
	}
}

func kwVal(v eval.Value) string {
	if k, ok := v.(eval.Kw); ok {
		return string(k)
	}
	return ""
}

func paintCursor(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	col := evalColorToGG(n.Props["color"], th.Accent)
	if v, ok := n.Props["opacity"].(eval.Num); ok {
		col.A *= float64(v)
	} else {
		col.A *= deps.CursorAlpha
	}
	r := n.Rect
	dc.SetColor(col)
	dc.DrawRectangle(r.X, r.Y, r.W, r.H)
	_ = dc.Fill()
}
