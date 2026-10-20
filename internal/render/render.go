// Package render draws the overlay with gg into a buffer, unpremultiplies it,
// and presents it on layer surfaces via gogpu. The composition is ported from
// the QML implementation: a tilted trapezoid band in slab colour on a
// transparent surface, a search field at the top of the rotated band content,
// and sheared cards whose selected state is a solid accent fill.
package render

import (
	"image"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/darkliquid/keyshell/internal/card"
	"github.com/darkliquid/keyshell/internal/config"
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

// cursorBlinkPeriodMs is the QML cursor's fade cycle: 400ms out, 400ms in.
const cursorBlinkPeriodMs = 800

// Renderer holds the per-summon reveal geometry, resampled on each open.
type Renderer struct {
	mu       sync.Mutex
	reveal   Reveal
	wasOpen  bool
	opened   time.Time
	surfaceW float64
	surfaceH float64
	rng      *rand.Rand
}

// New builds a renderer.
func New() *Renderer {
	return &Renderer{rng: rand.New(rand.NewSource(1))}
}

// DrawFrame paints the current launcher state at the given surface size.
func (r *Renderer) DrawFrame(dc *gg.Context, l View, th *theme.Theme, cfg config.Reveal) {
	w := float64(dc.Width())
	h := float64(dc.Height())

	if !l.IsOpen() {
		dc.ClearWithColor(gg.RGBA{})
		r.wasOpen = false
		return
	}

	r.mu.Lock()
	if !r.wasOpen {
		r.reveal = Resample(w, h, cfg, r.rng)
		r.wasOpen = true
		r.opened = time.Now()
	}
	r.surfaceW, r.surfaceH = w, h
	rev := r.reveal
	elapsed := time.Since(r.opened)
	r.mu.Unlock()

	// Outside the band is transparent: only the band itself is slab.
	dc.ClearWithColor(gg.RGBA{})
	drawBand(dc, th, rev, w)

	// Nothing may paint outside the slab. Cards are sheared more steeply than
	// the band tilts and the reel is wider than the band, so without this the
	// corners of both leak onto the desktop instead of flowing inside it.
	bandPath(dc, rev, w)
	dc.Clip()

	// Content lives in the band's rotated frame.
	cw := math.Sqrt(w*w + (rev.YRight-rev.YLeft)*(rev.YRight-rev.YLeft))
	ch := math.Min(rev.HLeft, rev.HRight) * math.Cos(rev.Angle)

	dc.Push()
	dc.Translate(w/2, rev.Yc)
	dc.Rotate(rev.Angle)
	dc.Translate(-cw/2, -ch/2)

	drawSearchField(dc, th, l, cfg, cw, cursorAlpha(elapsed))
	drawCards(dc, th, l, rev, cfg, cw, ch)

	dc.Pop()
}

// BandContains reports whether a surface point lies inside the band, which is
// what separates a click on the overlay from one on the dismiss scrim.
func (r *Renderer) BandContains(x, y float64) bool {
	r.mu.Lock()
	rev, w, h := r.reveal, r.surfaceW, r.surfaceH
	r.mu.Unlock()
	if w <= 0 || h <= 0 || x < 0 || y < 0 || x > w || y > h {
		return false
	}
	quad := [4][2]float64{
		{0, rev.YLeft - rev.HLeft/2},
		{w, rev.YRight - rev.HRight/2},
		{w, rev.YRight + rev.HRight/2},
		{0, rev.YLeft + rev.HLeft/2},
	}
	return insideConvex(quad, x, y)
}

// bandPath lays the tilted trapezoid down as the current path.
func bandPath(dc *gg.Context, rev Reveal, w float64) {
	halfL := rev.HLeft / 2
	halfR := rev.HRight / 2

	dc.NewSubPath()
	dc.MoveTo(0, rev.YLeft-halfL)
	dc.LineTo(w, rev.YRight-halfR)
	dc.LineTo(w, rev.YRight+halfR)
	dc.LineTo(0, rev.YLeft+halfL)
	dc.ClosePath()
}

// drawBand fills the tilted trapezoid in slab colour.
func drawBand(dc *gg.Context, th *theme.Theme, rev Reveal, w float64) {
	dc.SetColor(th.Slab)
	bandPath(dc, rev, w)
	_ = dc.Fill()
}

// drawSearchField draws the mode badge, the uppercased query with its cursor
// bar, and the result count, centred at the top of the band.
// cursorAlpha fades the cursor in and out on the QML cycle.
func cursorAlpha(elapsed time.Duration) float64 {
	phase := float64(elapsed.Milliseconds()%cursorBlinkPeriodMs) / cursorBlinkPeriodMs
	// 0..1..0 triangle: full at the start of each half cycle.
	if phase < 0.5 {
		return 1 - phase*2
	}
	return (phase - 0.5) * 2
}

func drawSearchField(dc *gg.Context, th *theme.Theme, l View, cfg config.Reveal, cw float64, cursorA float64) {
	fieldW := math.Min(cw*0.8, 1200)
	fieldX := (cw - fieldW) / 2
	fieldTop := cfg.ContentPaddingPx
	midY := fieldTop + cfg.SearchHeightPx/2

	badge := l.ModeBadge()
	if badge == "" {
		badge = "SEARCH"
	}
	badgeFace := th.Face(12)
	badgeW := badgeFace.Advance(badge) + 16

	// Badge plate: accent fill, slab text.
	badgeH := 28.0
	dc.SetColor(th.Accent)
	dc.DrawRectangle(fieldX, midY-badgeH/2, badgeW, badgeH)
	_ = dc.Fill()

	dc.SetFont(badgeFace)
	dc.SetColor(th.Slab)
	dc.DrawString(badge, fieldX+8, midY+4)

	// Query text, uppercased, in ink.
	queryFace := th.Face(22)
	dc.SetFont(queryFace)
	dc.SetColor(th.Ink)
	query := strings.ToUpper(l.Query())
	queryX := fieldX + badgeW + 12
	dc.DrawString(query, queryX, midY+8)

	// Cursor bar after the query.
	cursorX := queryX + queryFace.Advance(query) + 2
	cursor := th.Accent
	cursor.A = cursorA
	dc.SetColor(cursor)
	dc.DrawRectangle(cursorX, midY-12, 3, 24)
	_ = dc.Fill()

	// Match count on the right of the field.
	countFace := th.Face(14)
	dc.SetFont(countFace)
	dc.SetColor(th.Muted)
	count := itoa(len(l.Results()))
	dc.DrawString(count, fieldX+fieldW-countFace.Advance(count), midY+4)
}

// drawCards lays the reel across the band content.
func drawCards(dc *gg.Context, th *theme.Theme, l View, rev Reveal, cfg config.Reveal, cw, ch float64) {
	cardTop := cfg.ContentPaddingPx + cfg.SearchHeightPx + 8
	cardH := ch - cardTop - cfg.ContentPaddingPx
	if cardH <= 0 {
		return
	}
	cardW := math.Min(cfg.CardWidthMaxPx, cw/3)
	results := l.Results()
	selected := l.Selected()

	// Centre the reel around the selected card.
	visible := int(cw/(cardW+8)) + 2
	first := selected - visible/2
	if first < 0 {
		first = 0
	}
	if first+visible > len(results) {
		first = len(results) - visible
		if first < 0 {
			first = 0
		}
	}

	x := 0.0
	for i := first; i < len(results) && i < first+visible; i++ {
		drawCard(dc, th, rev, cfg, x, cardTop, cardW, cardH, card.Compose(results[i].Item), i == selected)
		x += cardW + 8
	}
}

// drawCard reproduces the QML Card: a sheared slab (or accent when selected)
// with a 3px border, a cyan offset shadow behind unselected cards, and the
// composed plan painted in an axis-aligned content box inset past the shear.
func drawCard(dc *gg.Context, th *theme.Theme, rev Reveal, cfg config.Reveal, x, y, w, h float64, p card.Plan, selected bool) {
	if !selected {
		shadow := th.AccentAlt
		shadow.A = 0.7
		cardQuad(dc, rev, x+4, y+4, w, h, shadow, th.AccentAlt, 0)
	}

	fill := th.Slab
	border := th.Muted
	if selected {
		fill = th.Accent
		border = th.Accent
	}
	cardQuad(dc, rev, x, y, w, h, fill, border, 3)

	inset := math.Ceil(math.Abs(rev.Shear)*h/2) + 4 + 10
	drawCardContent(dc, th, p, selected, x+inset, y+10, w-2*inset, h-20)
}

// cardQuad fills and strokes a parallelogram matching the QML shear matrix:
// y' = y + shear*x, x' = x - shear*h/2.
func cardQuad(dc *gg.Context, rev Reveal, x, y, w, h float64, fill, border gg.RGBA, borderWidth float64) {
	shear := rev.Shear
	hx := shear * h / 2
	tl := [2]float64{x - hx, y}
	tr := [2]float64{x - hx + w, y + shear*w}
	br := [2]float64{x - hx + w, y + shear*w + h}
	bl := [2]float64{x - hx, y + h}

	if fill.A > 0 {
		dc.SetColor(fill)
		dc.NewSubPath()
		dc.MoveTo(tl[0], tl[1])
		dc.LineTo(tr[0], tr[1])
		dc.LineTo(br[0], br[1])
		dc.LineTo(bl[0], bl[1])
		dc.ClosePath()
		_ = dc.Fill()
	}
	if borderWidth > 0 {
		dc.SetColor(border)
		dc.SetLineWidth(borderWidth)
		dc.NewSubPath()
		dc.MoveTo(tl[0], tl[1])
		dc.LineTo(tr[0], tr[1])
		dc.LineTo(br[0], br[1])
		dc.LineTo(bl[0], bl[1])
		dc.ClosePath()
		_ = dc.Stroke()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
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
