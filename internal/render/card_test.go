package render

import (
	"image"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/card"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// TestDrawCardPaintsChipsAndTitle checks a row card draws its header chips and
// that its text is visible.
func TestDrawCardPaintsChipsAndTitle(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	plan := card.Compose(backend.Item{
		Title: "Speakers", Subtitle: "hdmi", Icon: "audio",
		Badge: "OUTPUT", Status: "DEFAULT", StatusTone: backend.TonePositive,
	})
	dc := gg.NewContext(400, 260)
	drawCard(dc, th, Reveal{}, config.Default().Reveal, 10, 10, 340, 220, plan, false)
	img := dc.Image().(*image.RGBA)

	if !containsColor(img, 0x08, 0xF7, 0xFE) {
		t.Error("cyan chip pixels missing")
	}
	if !containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Error("ink text pixels missing")
	}
}

// TestDrawCardSelectedUsesSlabText checks the slide-to-accent state does not
// paint ink on the accent fill.
func TestDrawCardSelectedUsesSlabText(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	plan := card.Compose(backend.Item{Title: "Ghostty", Icon: "terminal", Actions: []backend.Action{{Type: "exec", Value: "ghostty"}}})
	dc := gg.NewContext(400, 260)
	drawCard(dc, th, Reveal{}, config.Default().Reveal, 10, 10, 340, 220, plan, true)
	img := dc.Image().(*image.RGBA)

	if containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Error("ink pixels on a selected card, want slab text only")
	}
	if !containsColorIn(img, 0x0B, 0x0B, 0x0F, 0x30) {
		t.Error("slab text pixels missing on the selected card")
	}
}

// TestElideFits checks the helper trims to the available width.
func TestElideFits(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	face := th.Face(12)
	got := elide(face, strings.Repeat("WIDE ", 40), 80)
	if face.Advance(got) > 80 {
		t.Errorf("elided %q advances %v, want <= 80", got, face.Advance(got))
	}
	if elide(face, "ok", 80) != "ok" {
		t.Error("short text was changed")
	}
}

// containsColorIn allows a colour-range match, used where antialiasing means an
// exact pixel may not exist.
func containsColorIn(img *image.RGBA, r, g, b, tolerance uint8) bool {
	for y := 0; y < img.Bounds().Dy(); y += 2 {
		for x := 0; x < img.Bounds().Dx(); x += 2 {
			px := img.RGBAAt(x, y)
			if absDiff(px.R, r) <= int(tolerance) && absDiff(px.G, g) <= int(tolerance) && absDiff(px.B, b) <= int(tolerance) {
				return true
			}
		}
	}
	return false
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
