package render

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/icons"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// TestDrawFrameProducesImage renders one frame and checks the slab background
// and that the ink title text is present.
func TestDrawFrameProducesImage(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	view := &fakeView{open: true, results: []rank.Match{
		{Item: backend.Item{Title: "Ghostty", Subtitle: "Terminal"}},
		{Item: backend.Item{Title: "Firefox", Subtitle: "Web Browser"}},
	}}
	dc := gg.NewContext(1600, 900)
	New().DrawFrame(dc, view, th, config.Default().Reveal)
	img := dc.Image().(*image.RGBA)

	// Outside the band the surface is transparent, as in the QML overlay.
	if got := img.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent outside the band)", got.A)
	}
	// Inside the band the slab or a card is painted.
	if got := img.RGBAAt(800, 450); got.A == 0 {
		t.Error("centre is transparent, want the band painted")
	}
	// Ink title present somewhere near the top-left.
	if !containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Errorf("ink title pixels missing")
	}
}

type fakeView struct {
	open      bool
	results   []rank.Match
	query     string
	mode      string
	selected  int
	searching bool

	panelOpen  bool
	panelRows  []backend.PanelRow
	panelFocus int
	panelDirty bool
}

func (v *fakeView) Results() []rank.Match {
	if v.results != nil {
		return v.results
	}
	return nil
}
func (v *fakeView) Poll() {}
func (v *fakeView) Selected() int {
	if v.selected != 0 || v.results == nil {
		return v.selected
	}
	return 0
}
func (v *fakeView) Query() string {
	if v.query != "" {
		return v.query
	}
	return ""
}
func (v *fakeView) IsOpen() bool { return v.open }
func (v *fakeView) Searching() bool {
	return v.searching
}
func (v *fakeView) ModeBadge() string {
	if v.mode != "" {
		return v.mode
	}
	return "SEARCH"
}
func (v *fakeView) PanelOpen() bool        { return v.panelOpen }
func (v *fakeView) PanelRows() []backend.PanelRow { return v.panelRows }
func (v *fakeView) PanelFocus() int        { return v.panelFocus }
func (v *fakeView) PanelDirty() bool       { return v.panelDirty }

// TestDrawFrameSearchingIndicator proves an in-flight query paints the busy
// strip: pixels just under the search row differ between an idle and a
// searching frame.
func TestDrawFrameSearchingIndicator(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	results := []rank.Match{{Item: backend.Item{Title: "Ghostty", Subtitle: "Terminal"}}}
	draw := func(searching bool) *image.RGBA {
		view := &fakeView{open: true, query: "fire", mode: "apps", results: results, searching: searching}
		dc := gg.NewContext(1600, 900)
		New().DrawFrame(dc, view, th, config.Default().Reveal)
		return dc.Image().(*image.RGBA)
	}
	idle, busy := draw(false), draw(true)

	// The strip is a 300x4 accent bar centred in the content frame just below
	// the search row; scan a window around its surface position and require
	// a solid block of changed pixels, so the blinking cursor (72px, outside
	// the window) cannot satisfy this.
	// In an idle frame, the trailing element of the search field is the
	// results count text. In a searching frame, it is the 20x20 spinner ring
	// with an orbiting accent dot (around x: 1380..1415, y: 430..460).
	diff := 0
	for y := 430; y <= 460; y++ {
		for x := 1380; x <= 1415; x++ {
			if idle.RGBAAt(x, y) != busy.RGBAAt(x, y) {
				diff++
			}
		}
	}
	if diff < 20 {
		t.Errorf("spinner region diff = %d pixels, want >= 20 (searching indicator missing)", diff)
	}
	// The spinner's accent dot must be painted in the busy frame.
	if !containsColor(busy, 0xFF, 0x2E, 0x63) {
		t.Errorf("searching indicator accent dot missing")
	}
}

// TestDrawFramePanelPaintsRows proves an open panel renders control rows: the
// frame with PanelOpen differs from the closed frame in the region below the
// card.
func TestDrawFramePanelPaintsRows(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	results := []rank.Match{{Item: backend.Item{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.5, Min: 0, Max: 1, Step: 0.01},
			{ID: "mute", Kind: backend.ControlToggle, Label: "Mute"},
		},
	}}}
	rows := []backend.PanelRow{
		{Kind: backend.ControlRange, Label: "Volume", Control: &results[0].Item.Controls[0]},
		{Kind: backend.ControlToggle, Label: "Mute", Control: &results[0].Item.Controls[1]},
	}
	// Reproduce the presenter pipeline: draw the scene (which includes the
	// panel as a scene node), apply the band mask, then paint the panel
	// overlay on top so it survives the mask outside the band.
	draw := func(open bool) *image.RGBA {
		view := &fakeView{open: true, mode: "audio", results: results, selected: 0,
			panelOpen: open, panelRows: rows, panelFocus: 0}
		r := New()
		dc := gg.NewContext(1600, 900)
		r.DrawFrame(dc, view, th, config.Default().Reveal)
		r.MaskFrame(dc.Image().(*image.RGBA))
		if r.scene != nil {
			r.scene.DrawPanelOverlay(dc, th)
		}
		return dc.Image().(*image.RGBA)
	}
	closed, opened := draw(false), draw(true)
	diff := 0
	// The panel renders below the band (surface y ~645+), outside the band
	// trapezoid. Scan that region (reel centre ~800).
	for y := 650; y <= 800; y++ {
		for x := 600; x <= 1000; x++ {
			if closed.RGBAAt(x, y) != opened.RGBAAt(x, y) {
				diff++
			}
		}
	}
	if diff < 200 {
		t.Errorf("panel region diff = %d pixels, want >= 200 (panel rows not painted)", diff)
	}
}

// TestDrawFramePaintsResolvedIcon proves a card icon name resolves through
// the theme resolver and the decoded image reaches the card once the async
// decode lands.
func TestDrawFramePaintsResolvedIcon(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	// A one-theme tree with a pure-red 48px icon.
	root := t.TempDir()
	dir := filepath.Join(root, "icons", "themeA", "48x48", "apps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "icons", "themeA", "index.theme"),
		[]byte("[Icon Theme]\nName=themeA\nDirectories=48x48/apps\n\n[48x48/apps]\nSize=48\nType=Fixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := range 48 {
		for x := range 48 {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, "firefox.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	r := New()
	r.icons = icons.NewResolver("themeA", filepath.Join(root, "icons"))

	view := &fakeView{open: true, results: []rank.Match{
		{Item: backend.Item{Title: "Firefox", Icon: "firefox"}},
	}}
	dc := gg.NewContext(1600, 900)
	deadline := time.Now().Add(2 * time.Second)
	for {
		r.DrawFrame(dc, view, th, config.Default().Reveal)
		out := dc.Image().(*image.RGBA)
		if containsColor(out, 0xFF, 0, 0) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the resolved icon never reached the card")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func closeTo(img *image.RGBA, x, y int, r, g, b uint8) bool {
	px := img.RGBAAt(x, y)
	return px.R == r && px.G == g && px.B == b
}

func containsColor(img *image.RGBA, r, g, b uint8) bool {
	for y := 0; y < img.Bounds().Dy(); y += 4 {
		for x := 0; x < img.Bounds().Dx(); x += 4 {
			if closeTo(img, x, y, r, g, b) {
				return true
			}
		}
	}
	return false
}

// TestMaskFrameKeepsTheBand checks the mask leaves pixels inside the band
// untouched and clears everything outside it.
func TestMaskFrameKeepsTheBandContent(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 3))
	src.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	src.SetRGBA(0, 1, color.RGBA{R: 128, G: 128, B: 128, A: 128})
	src.SetRGBA(0, 2, color.RGBA{})

	r := New()
	r.wasOpen = true
	r.surfaceW = 1
	r.reveal = Reveal{YLeft: 75, YRight: 75, HLeft: 400, HRight: 400, Yc: 75}

	r.MaskFrame(src)
	if got := src.RGBAAt(0, 0); got.R != 255 || got.G != 255 || got.B != 255 || got.A != 255 {
		t.Errorf("opaque = %+v, want white at full alpha", got)
	}
	if got := src.RGBAAt(0, 1); got.R != 128 || got.A != 128 {
		t.Errorf("half = %+v, want the premultiplied pixel untouched", got)
	}
	if got := src.RGBAAt(0, 2); got.A != 0 {
		t.Errorf("transparent = %+v, want alpha 0", got)
	}

	// The same frame with the band moved off the rows clears them entirely.
	r.reveal = Reveal{YLeft: -500, YRight: -500, HLeft: 100, HRight: 100}
	src.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	r.MaskFrame(src)
	if got := src.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("outside the band = %+v, want alpha 0", got)
	}
}

// TestDrawFrameBlanksWhenClosed proves a closed launcher renders fully
// transparent rather than the palette.
func TestDrawFrameBlanksWhenClosed(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	view := &fakeView{open: false}
	dc := gg.NewContext(1600, 900)
	New().DrawFrame(dc, view, th, config.Default().Reveal)
	img := dc.Image().(*image.RGBA)
	if got := img.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("closed frame corner alpha = %d, want 0", got.A)
	}
}

// TestDrawFrameClipsContentToTheBand proves nothing paints outside the slab.
// Cards are sheared more steeply than the band tilts, so without a clip their
// corners and the reel overflow the trapezoid and the overlay leaks onto the
// desktop.
func TestDrawFrameClipsContentToTheBand(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	rev := Reveal{
		YLeft: 260, YRight: 640,
		HLeft: 120, HRight: 320,
		Angle: math.Atan2(380, 1600),
		Shear: 0.30,
		Yc:    450,
	}
	r := New()
	r.wasOpen = true
	r.reveal = rev

	view := &fakeView{open: true, results: []rank.Match{
		{Item: backend.Item{Title: "Ghostty", Icon: "terminal", Actions: []backend.Action{{Type: "exec", Value: "ghostty"}}}},
		{Item: backend.Item{Title: "Firefox", Icon: "firefox", Actions: []backend.Action{{Type: "exec", Value: "firefox"}}}},
	}}
	dc := gg.NewContext(1600, 900)
	r.DrawFrame(dc, view, th, config.Default().Reveal)
	img := dc.Image().(*image.RGBA)

	quad := [4][2]float64{
		{0, rev.YLeft - rev.HLeft/2},
		{1600, rev.YRight - rev.HRight/2},
		{1600, rev.YRight + rev.HRight/2},
		{0, rev.YLeft + rev.HLeft/2},
	}
	before := countOutsideBand(img, quad)
	r.MaskFrame(img)
	leaks := countOutsideBand(img, quad)
	if before == 0 {
		t.Fatal("no content painted outside the band: the mask is not exercised")
	}
	if leaks != 0 {
		t.Errorf("%d painted pixels still outside the band after masking (was %d)", leaks, before)
	}
}

// countOutsideBand counts painted pixels outside the quad.
func countOutsideBand(img *image.RGBA, quad [4][2]float64) int {
	leaks := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y).A == 0 {
				continue
			}
			if insideConvex(quad, float64(x)+0.5, float64(y)+0.5) {
				continue
			}
			leaks++
		}
	}
	return leaks
}
