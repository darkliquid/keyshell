// Package render draws the overlay with gg into a buffer, unpremultiplies it,
// and presents it on layer surfaces via gogpu. It owns the visual vocabulary:
// the slab background, the band, reel cards and control widgets. The state
// comes from the launcher; the drawing is stateless per frame.
package render

import (
	"image"
	"image/color"

	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// View is the slice of launcher state the renderer reads.
type View interface {
	Results() []rank.Match
	Selected() int
	Query() string
}

// DrawFrame paints the current launcher state into a gg context.
func DrawFrame(dc *gg.Context, l View, th *theme.Theme) {
	dc.ClearWithColor(th.Slab)

	titleFace := th.Face(44)
	dc.SetFont(titleFace)
	dc.SetColor(th.Ink)
	dc.DrawString("KEYSHELL", 40, 60)

	// The reel of results.
	results := l.Results()
	x := 40.0
	y := 110.0
	for i, m := range results {
		if i >= 12 {
			break
		}
		drawCard(dc, th, x, y, m, i == l.Selected(), l.Query())
		x += 300
	}

	// Status line.
	subFace := th.Face(18)
	dc.SetFont(subFace)
	dc.SetColor(th.Muted)
	dc.DrawString("Enter: open  Tab: mode  Escape: dismiss", 40, 780)
}

func drawCard(dc *gg.Context, th *theme.Theme, x, y float64, m rank.Match, selected bool, query string) {
	const cardW, cardH = 280.0, 200.0

	// Card surface with the solid offset shadow.
	if selected {
		dc.SetColor(th.Accent)
		dc.DrawRectangle(x+10, y+10, cardW, cardH)
		_ = dc.Fill()
	}
	dc.SetColor(th.SurfaceAlt)
	dc.DrawRectangle(x, y, cardW, cardH)
	_ = dc.Fill()

	// Title.
	titleFace := th.Face(28)
	dc.SetFont(titleFace)
	dc.SetColor(th.Ink)
	dc.DrawString(clip(m.Item.Title, 18), x+16, y+50)

	// Subtitle.
	subFace := th.Face(16)
	dc.SetFont(subFace)
	dc.SetColor(th.Muted)
	dc.DrawString(clip(m.Item.Subtitle, 26), x+16, y+78)

	// Selected accent border stays solid; non-selected is thin.
	if selected {
		dc.SetColor(th.AccentAlt)
		dc.SetLineWidth(3)
		dc.DrawRectangle(x, y, cardW, cardH)
		_ = dc.Stroke()
	} else {
		dc.SetColor(th.Muted)
		dc.SetLineWidth(1)
		dc.DrawRectangle(x, y, cardW, cardH)
		_ = dc.Stroke()
	}
	_ = query
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

// gamma uses the standard image/color conversion so the sample tests can find
// exact ink values.
var _ = color.NRGBA{}
