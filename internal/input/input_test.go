package input

import (
	"context"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/gogpu/gpucontext"
)

// stubEvents satisfies gpucontext.EventSource with no-ops.
type stubEvents struct{}

func (stubEvents) OnKeyPress(func(gpucontext.Key, gpucontext.Modifiers))         {}
func (stubEvents) OnKeyRelease(func(gpucontext.Key, gpucontext.Modifiers))       {}
func (stubEvents) OnTextInput(func(string))                                      {}
func (stubEvents) OnMouseMove(func(float64, float64))                            {}
func (stubEvents) OnMousePress(func(gpucontext.MouseButton, float64, float64))   {}
func (stubEvents) OnMouseRelease(func(gpucontext.MouseButton, float64, float64)) {}
func (stubEvents) OnScroll(func(float64, float64))                               {}
func (stubEvents) OnResize(func(int, int))                                       {}
func (stubEvents) OnFocus(func(bool))                                            {}
func (stubEvents) OnIMECompositionStart(func())                                  {}
func (stubEvents) OnIMECompositionUpdate(func(gpucontext.IMEState))              {}
func (stubEvents) OnIMECompositionEnd(func(string))                              {}

type fakeBackend struct {
	items []backend.Item
}

func (b *fakeBackend) Info() backend.Info {
	return backend.Info{ID: "audio", Searchable: true, StableIDs: true}
}
func (b *fakeBackend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	return &sliceStream{items: b.items}, nil
}
func (b *fakeBackend) Control(_ context.Context, _ backend.Change) error { return nil }
func (b *fakeBackend) Close() error                                      { return nil }

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

type fakeProvider struct {
	backends map[string]backend.Backend
}

func (p *fakeProvider) Get(id string) (backend.Backend, error) {
	if b, ok := p.backends[id]; ok {
		return b, nil
	}
	return nil, nil
}

func newPanelAdapter() (*Adapter, *launcher.Launcher) {
	audio := &fakeBackend{items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.5, Min: 0, Max: 1, Step: 0.01},
			{ID: "mute", Kind: backend.ControlToggle, Label: "Mute"},
		},
	}}}
	prov := &fakeProvider{backends: map[string]backend.Backend{"audio": audio}}
	l := launcher.New(prov, launcher.Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	l.Enter()
	return New(l, stubEvents{}), l
}

func TestPanelRoutingRoutesArrowsToNavigate(t *testing.T) {
	a, l := newPanelAdapter()
	if !l.PanelOpen() {
		t.Fatal("panel not open")
	}
	wantFocus := l.PanelFocus() + 1
	a.onKeyPress(gpucontext.KeyDown, 0)
	if got := l.PanelFocus(); got != wantFocus {
		t.Errorf("PanelFocus after Down = %d, want %d", got, wantFocus)
	}
}

func TestPanelRoutingRoutesLeftRightToAdjust(t *testing.T) {
	a, l := newPanelAdapter()
	a.onKeyPress(gpucontext.KeyRight, 0)
	if l.PanelFocus() != 0 {
		t.Errorf("PanelFocus = %d, want 0 (right adjusts, not navigate)", l.PanelFocus())
	}
}

func TestPanelRoutingRoutesEnterToActivate(t *testing.T) {
	a, l := newPanelAdapter()
	a.onKeyPress(gpucontext.KeyDown, 0)
	a.onKeyPress(gpucontext.KeyEnter, 0)
	if l.PanelFocus() != 1 {
		t.Errorf("PanelFocus = %d, want 1", l.PanelFocus())
	}
}

func TestClosedRoutingKeepsSelect(t *testing.T) {
	audio := &fakeBackend{items: []backend.Item{
		{ID: "a", Title: "A", Controls: []backend.Control{{ID: "volume", Kind: backend.ControlRange, Label: "Volume"}}},
		{ID: "b", Title: "B", Controls: []backend.Control{{ID: "volume", Kind: backend.ControlRange, Label: "Volume"}}},
	}}
	prov := &fakeProvider{backends: map[string]backend.Backend{"audio": audio}}
	l := launcher.New(prov, launcher.Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	a := New(l, stubEvents{})
	before := l.Selected()
	a.onKeyPress(gpucontext.KeyRight, 0)
	if got := l.Selected(); got != before+1 {
		t.Errorf("Selected after Right = %d, want %d (closed -> select)", got, before+1)
	}
}

func TestTabCyclesModes(t *testing.T) {
	prov := &fakeProvider{backends: map[string]backend.Backend{"audio": &fakeBackend{}}}
	l := launcher.New(prov, launcher.Options{
		Searchable: []string{"audio"},
		RankOpts:   rank.Options{Limit: 10},
	})
	l.Open()
	a := New(l, stubEvents{})

	if got := l.Mode(); got != launcher.MergedMode {
		t.Fatalf("Mode = %q, want %q", got, launcher.MergedMode)
	}
	a.onKeyPress(gpucontext.KeyTab, 0)
	if got := l.Mode(); got != "audio" {
		t.Errorf("Mode after Tab = %q, want audio", got)
	}
	a.onKeyPress(gpucontext.KeyTab, gpucontext.ModShift)
	if got := l.Mode(); got != launcher.MergedMode {
		t.Errorf("Mode after Shift+Tab = %q, want %q", got, launcher.MergedMode)
	}
}

func TestTabDoesNotCycleWhilePanelOpen(t *testing.T) {
	a, l := newPanelAdapter()
	if !l.PanelOpen() {
		t.Fatal("panel not open")
	}
	before := l.Mode()
	a.onKeyPress(gpucontext.KeyTab, 0)
	if got := l.Mode(); got != before {
		t.Errorf("Mode after Tab with panel open = %q, want %q unchanged", got, before)
	}
}
