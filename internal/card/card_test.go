package card

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// TestComposeHeroForALaunchItem checks the app card: a visual and a title and
// nothing else, with the footer suppressed by the hero layout.
func TestComposeHeroForALaunchItem(t *testing.T) {
	p := Compose(backend.Item{
		Title:   "Ghostty",
		Icon:    "utilities-terminal",
		Actions: []backend.Action{{Type: "exec", Value: "ghostty"}},
	})
	if p.Layout != LayoutHero {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutHero)
	}
	if p.Title.Style != StyleHero {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleHero)
	}
	if p.Footer.Kind != "" {
		t.Errorf("footer = %+v, want none in the hero layout", p.Footer)
	}
	if p.Visual.Source != "icon" {
		t.Errorf("visual source = %q, want icon", p.Visual.Source)
	}
}

// TestComposeRowForASpeaker checks the header chips, visual and text column.
func TestComposeRowForASpeaker(t *testing.T) {
	p := Compose(backend.Item{
		Title: "Speakers", Subtitle: "alsa_output.hdmi", Icon: "audio",
		Badge: "OUTPUT", Status: "DEFAULT", StatusTone: backend.TonePositive,
		Controls: []backend.Control{{ID: "volume", Kind: backend.ControlRange, Value: 0.35, Min: 0, Max: 1, Step: 0.01}},
	})
	if p.Layout != LayoutRow {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutRow)
	}
	if p.Badge.Kind != backend.PartBadge || p.Badge.Text != "OUTPUT" || p.Badge.Tone != backend.ToneAccent {
		t.Errorf("badge = %+v", p.Badge)
	}
	if p.Status.Kind != backend.PartStatus || p.Status.Tone != backend.TonePositive {
		t.Errorf("status = %+v", p.Status)
	}
	if p.Subtitle.Kind != backend.PartSubtitle || p.Subtitle.Text != "alsa_output.hdmi" {
		t.Errorf("subtitle = %+v", p.Subtitle)
	}
	if p.Title.Style != StyleLine {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleLine)
	}
	if p.Footer.Kind != "" {
		t.Errorf("footer = %+v, want none without actions", p.Footer)
	}
}

// TestComposeBodyForACalculator checks the iconless valuation card.
func TestComposeBodyForACalculator(t *testing.T) {
	p := Compose(backend.Item{
		Title: "14", Subtitle: "2+3*4",
		Actions: []backend.Action{{Type: "copy", Value: "14"}},
	})
	if p.Layout != LayoutBody {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutBody)
	}
	if p.Title.Style != StyleValue {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleValue)
	}
	if p.Footer.Text != "ENTER COPY" {
		t.Errorf("footer = %q, want %q", p.Footer.Text, "ENTER COPY")
	}
}

// TestComposeWrapsPlainResultText checks that prose-only results get the
// wrapped title treatment rather than a single elided line.
func TestComposeWrapsPlainResultText(t *testing.T) {
	p := Compose(backend.Item{Title: "some long copied paragraph", Icon: "edit-copy"})
	if p.Layout != LayoutRow {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutRow)
	}
	if p.Title.Style != StyleWrap {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleWrap)
	}
}

// TestComposeUsesDetailOnlyWithoutAVisual keeps the prose body from competing
// with a leading visual for the same space.
func TestComposeUsesDetailOnlyWithoutAVisual(t *testing.T) {
	withVisual := Compose(backend.Item{Title: "T", Icon: "audio", Detail: "body"})
	if withVisual.Detail.Kind != "" {
		t.Errorf("detail = %+v, want none beside a visual", withVisual.Detail)
	}
	withoutVisual := Compose(backend.Item{Title: "T", Subtitle: "S", Detail: "body"})
	if withoutVisual.Detail.Kind != backend.PartDetail || withoutVisual.Detail.Text != "body" {
		t.Errorf("detail = %+v, want the body text", withoutVisual.Detail)
	}
}

// TestComposePrefersImageThenPreviewThenIcon pins the visual precedence.
func TestComposePrefersImageThenPreviewThenIcon(t *testing.T) {
	for _, tc := range []struct {
		name string
		item backend.Item
		want string
	}{
		{"image", backend.Item{Title: "T", Image: "art", Preview: "p", Icon: "i", Glyph: "g"}, "image"},
		{"preview", backend.Item{Title: "T", Preview: "p", Icon: "i", Glyph: "g"}, "preview"},
		{"icon", backend.Item{Title: "T", Icon: "i", Glyph: "g"}, "icon"},
		{"glyph", backend.Item{Title: "T", Glyph: "g"}, "glyph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compose(tc.item).Visual.Source; got != tc.want {
				t.Errorf("source = %q, want %q", got, tc.want)
			}
		})
	}
}
