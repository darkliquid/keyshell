package launcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/rank"
)

// fakeProvider serves backends from a map.
type fakeProvider struct {
	backends map[string]backend.Backend
}

func (p *fakeProvider) Get(id string) (backend.Backend, error) {
	if b, ok := p.backends[id]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("unknown backend %q", id)
}

// listBackend returns fixed items.
type listBackend struct {
	id      string
	items   []backend.Item
	query   backend.Query
	control []backend.Change
}

func (b *listBackend) Info() backend.Info {
	return backend.Info{ID: b.id, Searchable: true, StableIDs: true}
}
func (b *listBackend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	b.query = q
	return &sliceStream{items: b.items}, nil
}
func (b *listBackend) Control(_ context.Context, c backend.Change) error {
	b.control = append(b.control, c)
	return nil
}
func (b *listBackend) Close() error { return nil }

type sliceStream struct {
	items []backend.Item
	i     int
}

func (s *sliceStream) Next() (backend.Item, bool, error) {
	if s.i >= len(s.items) {
		return backend.Item{}, false, nil
	}
	it := s.items[s.i]
	s.i++
	return it, true, nil
}
func (s *sliceStream) Close() error { return nil }

func newTestLauncher() (*Launcher, *fakeProvider) {
	apps := &listBackend{
		id: "apps",
		items: []backend.Item{
			{ID: "a", Title: "Ghostty", Actions: []backend.Action{{Type: "exec", Value: "ghostty"}}},
			{ID: "b", Title: "Firefox", Actions: []backend.Action{{Type: "exec", Value: "firefox"}}},
		},
	}
	files := &listBackend{
		id: "files",
		items: []backend.Item{
			{ID: "/tmp/x", Title: "/tmp/x", Actions: []backend.Action{{Type: "open-url", Value: "file:///tmp/x"}}},
		},
	}
	prov := &fakeProvider{backends: map[string]backend.Backend{
		"apps": apps, "files": files, "windows": &listBackend{id: "windows"},
	}}
	l := New(prov, Options{SettleMs: 20 * time.Millisecond, RankOpts: rank.Options{Limit: 10}})
	return l, prov
}

// TestMergedSearchUsesConfiguredSearchable proves the merged fan-out honours
// Options.Searchable rather than a hardcoded list.
func TestMergedSearchUsesConfiguredSearchable(t *testing.T) {
	apps := &listBackend{
		id:    "apps",
		items: []backend.Item{{ID: "a", Title: "Alpha"}},
	}
	prov := &fakeProvider{backends: map[string]backend.Backend{"apps": apps}}
	l := New(prov, Options{SettleMs: 20 * time.Millisecond, RankOpts: rank.Options{Limit: 10}, Searchable: []string{"apps"}})
	l.Open()
	l.SetMode("all")
	l.SetQuery("")
	if len(l.Results()) != 1 {
		t.Errorf("results = %d, want 1 (only apps in Searchable)", len(l.Results()))
	}
}

// TestDefaultModeIsMerged guards the summon path: a launcher opened without an
// explicit SetMode must search every searchable backend. Starting with an empty
// mode id searched a backend called "" and returned nothing.
func TestDefaultModeIsMerged(t *testing.T) {
	l, _ := newTestLauncher()
	if got := l.Mode(); got != MergedMode {
		t.Fatalf("Mode() = %q, want %q", got, MergedMode)
	}
	l.Open()
	if len(l.Results()) == 0 {
		t.Fatal("no results after Open without SetMode")
	}
	if got := l.ModeBadge(); got != "SEARCH" {
		t.Errorf("ModeBadge() = %q, want SEARCH", got)
	}
}

func TestMergedSearchRanksAcrossBackends(t *testing.T) {
	l, _ := newTestLauncher()
	l.Open()
	l.SetMode("all")
	l.SetQuery("")

	if len(l.Results()) == 0 {
		t.Fatal("expected merged results")
	}
	// Both apps plus the file appear.
	if len(l.Results()) < 3 {
		t.Errorf("results = %d, want >= 3", len(l.Results()))
	}
}

func TestSelectMovesSelection(t *testing.T) {
	l, _ := newTestLauncher()
	l.Open()
	l.SetMode("all")
	index := l.Selected()
	l.Select(1)
	if l.Selected() != (index+1)%len(l.Results()) {
		t.Errorf("selection did not move")
	}
}

func TestBackPopsLevel(t *testing.T) {
	l, _ := newTestLauncher()
	l.Open()
	l.SetMode("apps")
	before := len(l.levels)
	// Simulate descending by pushing a level.
	l.levels = append(l.levels, &Level{Source: "apps"})
	l.Back()
	if len(l.levels) != before {
		t.Errorf("levels = %d, want %d after Back", len(l.levels), before)
	}
}

func TestEscDismissesAtRoot(t *testing.T) {
	l, _ := newTestLauncher()
	l.Open()
	if !l.IsOpen() {
		t.Fatal("expected open")
	}
	l.Escape()
	if l.IsOpen() {
		t.Error("Escape at root should dismiss")
	}
}

func TestDebouncedControlCoalesces(t *testing.T) {
	l, prov := newTestLauncher()
	l.Open()
	l.SetMode("apps")
	apps := prov.backends["apps"].(*listBackend)

	for i := 0; i < 5; i++ {
		l.DebouncedControl("a", "volume", float64(i))
	}
	// Wait past the settle window.
	time.Sleep(80 * time.Millisecond)
	if len(apps.control) != 1 {
		t.Errorf("control calls = %d, want 1 (coalesced)", len(apps.control))
	}
	if apps.control[0].Value != 4 {
		t.Errorf("value = %v, want 4 (last settled)", apps.control[0].Value)
	}
}
