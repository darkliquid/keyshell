package render

import (
	"image"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// paintImage draws the Image node: the cached bitmap or the placeholder,
// fitted per `fit` and tinted per `tint`.
func paintImage(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	r := n.Rect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	src, ok := n.Props["source"].(eval.Str)
	if !ok {
		return
	}
	if deps.Images == nil {
		return
	}
	img, ready := deps.Images.Get(string(src), int(r.W), int(r.H))
	if !ready {
		img = deps.Images.Placeholder()
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
