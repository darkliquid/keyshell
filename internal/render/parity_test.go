package render

// This file pins visual parity between the legacy (pre-KSL) renderer
// and the scene-driven renderer for the default scene.

import (
	"image"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

func renderPair(t *testing.T) (*image.RGBA, *image.RGBA) {
	t.Helper()
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	view := &fakeView{open: true, query: "fire", mode: "apps",
		results: []rank.Match{
			{Item: backend.Item{ID: "a", Title: "Firefox", Subtitle: "Web Browser", Badge: "APP"}},
			{Item: backend.Item{ID: "b", Title: "Ghostty", Subtitle: "Terminal", Badge: "DEV"}},
			{Item: backend.Item{ID: "c", Title: "VLC", Subtitle: "Media", Badge: "MEDIA"}},
		},
		selected: 1}
	cfg := config.Default()
	leg := New()
	lg := gg.NewContext(1600, 900)
	legacyDrawFrame(leg, lg, view, th, cfg.Reveal)
	scn := New()
	sg := gg.NewContext(1600, 900)
	scn.DrawFrame(sg, view, th, cfg.Reveal)
	return lg.Image().(*image.RGBA), sg.Image().(*image.RGBA)
}

func regionInk(t *testing.T, img *image.RGBA, x0, y0, x1, y1 int) int {
	t.Helper()
	n := 0
	for y := y0; y < y1; y += 2 {
		for x := x0; x < x1; x += 2 {
			p := img.RGBAAt(x, y)
			if p.R > 200 && p.G > 200 && p.B > 200 {
				n++
			}
		}
	}
	return n
}

func accentIn(t *testing.T, img *image.RGBA, x0, y0, x1, y1 int) int {
	t.Helper()
	n := 0
	for y := y0; y < y1; y += 2 {
		for x := x0; x < x1; x += 2 {
			p := img.RGBAAt(x, y)
			if p.R > 0xC0 && p.G < 0x40 && p.B < 0x80 {
				n++
			}
		}
	}
	return n
}

func TestParitySearchRow(t *testing.T) {
	leg, scn := renderPair(t)
	// ink in the search zone (upper band) must exist in both
	legInk := regionInk(t, leg, 200, 330, 1400, 420)
	scnInk := regionInk(t, scn, 200, 330, 1400, 420)
	t.Logf("search ink legacy=%d scene=%d", legInk, scnInk)
	if scnInk < 10 {
		t.Errorf("scene search zone has almost no text ink: %d", scnInk)
	}
	// accent (mode badge plate) must appear in the upper band
	legAcc := accentIn(t, leg, 0, 300, 1600, 420)
	scnAcc := accentIn(t, scn, 0, 300, 1600, 420)
	t.Logf("accent legacy=%d scene=%d", legAcc, scnAcc)
	if scnAcc < 20 {
		t.Errorf("scene mode badge accent missing: %d", scnAcc)
	}
}

func TestParityCards(t *testing.T) {
	leg, scn := renderPair(t)
	// slab card bodies in the lower band region
	slab := func(img *image.RGBA) int {
		n := 0
		for y := 430; y < 650; y += 2 {
			for x := 300; x < 1300; x += 2 {
				p := img.RGBAAt(x, y)
				if p.A != 0 && p.R < 0x20 && p.G < 0x20 && p.B < 0x30 {
					n++
				}
			}
		}
		return n
	}
	l, s := slab(leg), slab(scn)
	t.Logf("card slab legacy=%d scene=%d", l, s)
	if s < l/2 {
		t.Errorf("scene card region too sparse: %d vs %d", s, l)
	}
}
