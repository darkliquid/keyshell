package render

import (
	"math"
	"strconv"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
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
	case "Image":
		return paintImage
	case "Pill":
		return paintPill
	case "Meter":
		return paintMeter
	case "Cursor":
		return paintCursor
	case "Card":
		return paintCard
	default:
		return paintThrough
	}
}

// paintThrough paints children in declaration order (the painter sorts
// by z within siblings; z is carried on each node).
func paintThrough(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	if n.Name == "Group" {
		dc.Push()
		if v, ok := n.Props["rotate"].(eval.Length); ok {
			dc.Rotate(v.V * math.Pi / 180)
		}
	}
	paintChildren(dc, th, n, deps)
	if n.Name == "Group" {
		dc.Pop()
	}
}

func paintChildren(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	for _, c := range n.Children {
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
	r := n.Rect
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
}

func paintText(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	txt, ok := n.Props["text"].(eval.Str)
	if !ok {
		return
	}
	size := 22.0
	if v, ok := n.Props["size"].(eval.Length); ok && (v.Unit == "" || v.Unit == "px") {
		size = v.V
	} else if v, ok := n.Props["size"].(eval.Num); ok {
		size = float64(v)
	}
	if v, ok := n.Props["uppercase"].(eval.Bool); ok && bool(v) {
		txt = eval.Str(strings.ToUpper(string(txt)))
	}
	col := evalColorToGG(n.Props["color"], th.Ink)
	face := th.Face(size)
	dc.SetFont(face)
	dc.SetColor(gg.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * opacityOf(n.Props)})
	r := n.Rect
	dc.DrawStringAnchored(string(txt), r.X, r.Y, 0, 0)
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
	txt, ok := n.Props["text"].(eval.Str)
	if !ok {
		return
	}
	tone := ""
	if v, ok := n.Props["tone"].(eval.Kw); ok {
		tone = string(v)
	}
	fill, ink := chipColours(th, backend.Tone(tone), false)
	r := n.Rect
	face := th.Face(12)
	dc.SetFont(face)
	label := strings.ToUpper(string(txt))
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, 2)
	_ = dc.Fill()
	dc.SetColor(ink)
	dc.DrawStringAnchored(label, r.X+r.W/2, r.Y+r.H/2, 0.5, 0.5)
}

func paintMeter(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	ratio := 0.0
	if v, ok := n.Props["ratio"].(eval.Num); ok {
		ratio = float64(v)
	}
	variant := kwVal(n.Props["variant"])
	r := n.Rect
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
	col.A *= deps.CursorAlpha
	r := n.Rect
	dc.SetColor(col)
	dc.DrawRectangle(r.X, r.Y, r.W, r.H)
	_ = dc.Fill()
}
