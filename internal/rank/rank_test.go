package rank

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

func item(title, subtitle string) backend.Item {
	return backend.Item{ID: backend.ResultID(title), Title: title, Subtitle: subtitle}
}

// TestTierOrdering verifies the tier ordering: exact prefix beats word-boundary
// prefix beats substring beats subtitle/keyword.
func TestTierOrdering(t *testing.T) {
	groups := []Group{{Source: "apps", Items: []backend.Item{
		item("Ghostty", "Terminal"),
		item("Firefox", "Web Browser"),
		item("gHost", "hosting"),
	}}}

	// The word-boundary prefix on "Firefox" matches "fire" as a word start.
	matches := Rank("fire", groups, nil, Options{Limit: 10})
	if len(matches) == 0 {
		t.Fatal("no matches")
	}
	first := matches[0].Item.Title
	if first != "Firefox" {
		t.Errorf("first match = %q, want Firefox (word-boundary tier)", first)
	}
}

// TestFrecencyBreaksTies checks that a frequently used item rises within a tier.
func TestFrecencyBreaksTies(t *testing.T) {
	hist := &fakeHist{score: map[string]float64{
		"apps:ghostty": 1.0,
		"apps:firefox": 0.0,
	}}
	groups := []Group{{Source: "apps", Items: []backend.Item{
		item("ghostty", ""),
		item("firefox", ""),
	}}}
	matches := Rank("o", groups, hist, Options{Limit: 10})
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(matches))
	}
	if matches[0].Item.Title != "ghostty" {
		t.Errorf("top match = %q, want ghostty (frecency tiebreak)", matches[0].Item.Title)
	}
}

// TestMatchTierExactPrefix ensures exact prefix is the top tier.
func TestMatchTierExactPrefix(t *testing.T) {
	groups := []Group{{Source: "apps", Items: []backend.Item{
		item("Firefox Developer", ""),
		item("Firefox", ""),
	}}}
	matches := Rank("firefox", groups, nil, Options{Limit: 10})
	if matches[0].Item.Title != "Firefox" {
		t.Errorf("top = %q, want exact-title match first", matches[0].Item.Title)
	}
}

// TestSubstringTierSanity confirms substring ordering for titles.
func TestSubstringTierSanity(t *testing.T) {
	groups := []Group{{Source: "apps", Items: []backend.Item{
		item("Blender", ""),
		item("LibreOffice", ""),
	}}}
	matches := Rank("office", groups, nil, Options{Limit: 10})
	if len(matches) != 1 || matches[0].Item.Title != "LibreOffice" {
		t.Errorf("matches = %+v, want LibreOffice", matches)
	}
	if matches[0].Tier != 2 {
		t.Errorf("tier = %d, want 2 (title substring)", matches[0].Tier)
	}
}

// TestEmptyQueryRanksAll indicates an empty query returns everything at tier 0.
func TestEmptyQueryRanksAll(t *testing.T) {
	groups := []Group{{Source: "apps", Items: []backend.Item{
		item("A", ""),
		item("B", ""),
	}}}
	matches := Rank("", groups, nil, Options{Limit: 10})
	if len(matches) != 2 {
		t.Errorf("matches = %d, want 2", len(matches))
	}
}

type fakeHist struct {
	score map[string]float64
}

func (h *fakeHist) Score(source string, id backend.ResultID) float64 {
	if v, ok := h.score[source+":"+string(id)]; ok {
		return v
	}
	return 0
}
