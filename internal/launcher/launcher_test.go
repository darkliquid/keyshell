package launcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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
	mu            sync.Mutex
	id            string
	trigger       string
	nonSearchable bool
	items         []backend.Item
	query         backend.Query
	control       []backend.Change
}

func (b *listBackend) Info() backend.Info {
	return backend.Info{ID: b.id, Trigger: b.trigger, Searchable: !b.nonSearchable, StableIDs: true}
}
func (b *listBackend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.query = q
	return &sliceStream{items: b.items}, nil
}
func (b *listBackend) Control(_ context.Context, c backend.Change) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.control = append(b.control, c)
	return nil
}
func (b *listBackend) ControlCalls() []backend.Change {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]backend.Change, len(b.control))
	copy(out, b.control)
	return out
}
func (b *listBackend) Close() error { return nil }

type invokingBackend struct {
	listBackend
	invocations []backend.Invoke
}

func (b *invokingBackend) Invoke(_ context.Context, inv backend.Invoke) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.invocations = append(b.invocations, inv)
	return nil
}

type testLogHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *testLogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *testLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *testLogHandler) WithGroup(string) slog.Handler      { return h }

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

func TestEnterOpensPanelForControls(t *testing.T) {
	audio := &listBackend{id: "audio", items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.5, Min: 0, Max: 1, Step: 0.01},
			{ID: "mute", Kind: backend.ControlToggle, Label: "Mute"},
		},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"audio": audio}},
		Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	l.Enter()
	if !l.PanelOpen() {
		t.Fatal("PanelOpen = false, want true after Enter on a control card")
	}
	if got := len(l.PanelRows()); got != 2 {
		t.Errorf("PanelRows len = %d, want 2 (volume, mute)", got)
	}
	if l.PanelFocus() != 0 {
		t.Errorf("PanelFocus = %d, want 0", l.PanelFocus())
	}
}

func TestEnterWhenNoResultsDoesNothing(t *testing.T) {
	audio := &listBackend{id: "audio", items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.5, Min: 0, Max: 1, Step: 0.01},
			{ID: "mute", Kind: backend.ControlToggle, Label: "Mute"},
		},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"audio": audio}},
		Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	// Search for something with no matches
	l.SetQuery("nonexistent")
	if len(l.Results()) != 0 {
		t.Fatalf("Results len = %d, want 0", len(l.Results()))
	}
	l.Enter()
	if l.PanelOpen() {
		t.Fatal("PanelOpen = true, want false when pressing Enter with no results")
	}
}

func TestEnterOpensPanelWithActionFirst(t *testing.T) {
	media := &listBackend{id: "media", items: []backend.Item{{
		ID: "song", Title: "Song", Actions: []backend.Action{{Type: "exec", Value: "play"}},
		Controls: []backend.Control{{ID: "position", Kind: backend.ControlRange, Label: "Position", Value: 0.2}},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"media": media}},
		Options{Searchable: []string{"media"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("media")
	l.Enter()
	rows := l.PanelRows()
	if len(rows) != 2 {
		t.Fatalf("PanelRows len = %d, want 2 (action, position)", len(rows))
	}
	if rows[0].Kind != "action" || rows[0].Action == nil {
		t.Errorf("rows[0] = %+v, want action row first", rows[0])
	}
	if rows[1].Kind != backend.ControlRange {
		t.Errorf("rows[1].Kind = %q, want range", rows[1].Kind)
	}
}

func TestEnterWithoutControlsStillLaunches(t *testing.T) {
	apps := &listBackend{id: "apps", items: []backend.Item{{
		ID: "app1", Title: "Calculator", Actions: []backend.Action{{Type: "exec", Value: "calc"}},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"apps": apps}},
		Options{Searchable: []string{"apps"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("apps")
	l.Enter()
	if l.PanelOpen() {
		t.Error("PanelOpen = true, want false (no controls, plain launch)")
	}
	if _, ok := l.TakeAction(); !ok {
		t.Error("TakeAction = none, want the exec action still queued")
	}
}

func TestPanelNavigateClamps(t *testing.T) {
	audio := &listBackend{id: "audio", items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume"},
			{ID: "mute", Kind: backend.ControlToggle, Label: "Mute"},
		},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"audio": audio}},
		Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	l.Enter()
	l.PanelNavigate(1)
	if got := l.PanelFocus(); got != 1 {
		t.Errorf("PanelFocus = %d, want 1", got)
	}
	l.PanelNavigate(5)
	if got := l.PanelFocus(); got != 1 {
		t.Errorf("PanelFocus = %d, want 1 (clamped upper)", got)
	}
	l.PanelNavigate(-9)
	if got := l.PanelFocus(); got != 0 {
		t.Errorf("PanelFocus = %d, want 0 (clamped lower)", got)
	}
}

func TestPanelAdjustStepsRange(t *testing.T) {
	audio := &listBackend{id: "audio", items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.5, Min: 0, Max: 1, Step: 0.1},
		},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"audio": audio}},
		Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}, SettleMs: 20 * time.Millisecond})
	l.Open()
	l.SetMode("audio")
	l.Enter()
	l.PanelAdjust(-1)
	time.Sleep(80 * time.Millisecond)
	calls := audio.ControlCalls()
	if len(calls) != 1 {
		t.Fatalf("control calls = %d, want 1", len(calls))
	}
	if calls[0].Control != "volume" || calls[0].Value < 0.39 || calls[0].Value > 0.41 {
		t.Errorf("control = %+v, want volume 0.4", calls[0])
	}
	// Verify in-place update of card control in launcher results.
	results := l.Results()
	if len(results) == 0 || len(results[0].Item.Controls) == 0 {
		t.Fatalf("results missing controls: %+v", results)
	}
	if v := results[0].Item.Controls[0].Value; v < 0.39 || v > 0.41 {
		t.Errorf("card results[0] volume = %v, want 0.4", v)
	}
}

func TestPanelActivateToggles(t *testing.T) {
	audio := &listBackend{id: "audio", items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "mute", Kind: backend.ControlToggle, Label: "Mute", Bool: false},
		},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"audio": audio}},
		Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	l.Enter()
	l.PanelActivate()
	calls := audio.ControlCalls()
	if len(calls) != 1 || calls[0].Control != "mute" || !calls[0].Bool {
		t.Errorf("control calls = %+v, want mute=true", calls)
	}
	// Verify in-place update of toggle in launcher results.
	results := l.Results()
	if len(results) == 0 || len(results[0].Item.Controls) == 0 {
		t.Fatalf("results missing controls: %+v", results)
	}
	if !results[0].Item.Controls[0].Bool {
		t.Errorf("card results[0] mute = %v, want true", results[0].Item.Controls[0].Bool)
	}
}

func TestPanelActivateRunsAction(t *testing.T) {
	media := &listBackend{id: "media", items: []backend.Item{{
		ID: "song", Title: "Song", Actions: []backend.Action{{Type: "exec", Value: "play"}},
		Controls: []backend.Control{{ID: "volume", Kind: backend.ControlRange, Label: "Volume"}},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"media": media}},
		Options{Searchable: []string{"media"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("media")
	l.Enter()
	if rows := l.PanelRows(); len(rows) < 2 || rows[0].Kind != "action" {
		t.Fatalf("rows = %+v, want action row present", rows)
	}
	l.PanelActivate()
	if a, ok := l.TakeAction(); !ok || a.Value != "play" {
		t.Errorf("TakeAction = %+v, %v; want play action", a, ok)
	}
}

func TestEscapeClosesPanelFirst(t *testing.T) {
	audio := &listBackend{id: "audio", items: []backend.Item{{
		ID: "sink", Title: "Speakers", Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Label: "Volume"},
		},
	}}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"audio": audio}},
		Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("audio")
	l.Enter()
	if !l.PanelOpen() {
		t.Fatal("panel not open")
	}
	l.Escape()
	if l.PanelOpen() {
		t.Error("Escape left the panel open, want it closed first")
	}
	if !l.IsOpen() {
		t.Error("Escape dismissed the overlay, want only the panel closed")
	}
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
	calls := apps.ControlCalls()
	if len(calls) != 1 {
		t.Errorf("control calls = %d, want 1 (coalesced)", len(calls))
	}
	if len(calls) > 0 && calls[0].Value != 4 {
		t.Errorf("value = %v, want 4 (last settled)", calls[0].Value)
	}
}

func TestEnterDescendsWhenItemHasChildren(t *testing.T) {
	tray := &listBackend{id: "tray", items: []backend.Item{{
		ID: "applet-1", Title: "Applet", HasChildren: true,
		Actions: []backend.Action{{Type: "plugin", Value: "activate"}},
	}}}
	prov := &fakeProvider{backends: map[string]backend.Backend{"tray": tray}}
	l := New(prov, Options{Searchable: []string{"tray"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("tray")

	if len(l.Results()) != 1 {
		t.Fatalf("results len = %d, want 1", len(l.Results()))
	}
	l.Enter()

	// l.levels should now have depth 2 with scope containing applet-1
	if depth := len(l.levels); depth != 2 {
		t.Fatalf("levels depth = %d, want 2 (descended into child level)", depth)
	}
	if scope := l.levels[1].Scope; len(scope) != 1 || scope[0] != "applet-1" {
		t.Errorf("scope = %v, want [applet-1]", scope)
	}
}

func TestEnterActivatesWhenItemHasNoChildren(t *testing.T) {
	tray := &listBackend{id: "tray", items: []backend.Item{{
		ID: "applet-1", Title: "Applet", HasChildren: false,
		Actions: []backend.Action{{Type: "exec", Value: "echo hello"}},
	}}}
	prov := &fakeProvider{backends: map[string]backend.Backend{"tray": tray}}
	l := New(prov, Options{Searchable: []string{"tray"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("tray")

	l.Enter()
	action, ok := l.TakeAction()
	if !ok || action.Value != "echo hello" {
		t.Errorf("action = (%+v, %v), want echo hello", action, ok)
	}
}

// blockingBackend answers only once its gate is released, standing in for a
// plugin that shells out and takes a while.
type blockingBackend struct {
	id    string
	items []backend.Item
	gate  chan struct{}
}

func (b *blockingBackend) Info() backend.Info {
	return backend.Info{ID: b.id, Searchable: true, StableIDs: true}
}
func (b *blockingBackend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	<-b.gate
	return &sliceStream{items: b.items}, nil
}
func (b *blockingBackend) Control(context.Context, backend.Change) error { return nil }
func (b *blockingBackend) Close() error                                  { return nil }

// TestSearchRunsOffTheCallingThread checks a slow query does not block the
// caller once the worker is started: typing must never wait for a plugin.
func TestSearchRunsOffTheCallingThread(t *testing.T) {
	gate := make(chan struct{})
	b := &blockingBackend{id: "slow", items: []backend.Item{{ID: "x", Title: "x"}}, gate: gate}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"slow": b}},
		Options{Searchable: []string{"slow"}, RankOpts: rank.Options{Limit: 10}})
	l.Start()
	// Deliberately no Open: it would submit its own query and consume the one
	// gate release this test provides.
	done := make(chan struct{})
	go func() {
		l.SetQuery("x")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SetQuery blocked on the backend query")
	}

	if got := l.Results(); len(got) != 0 {
		t.Fatalf("results = %v before the worker finished, want none", got)
	}
	gate <- struct{}{}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		l.Poll()
		if len(l.Results()) == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("worker results never arrived through Poll")
}

// TestStaleResultsAreDropped checks a superseded query cannot overwrite newer
// results, which is what stops a slow plugin from publishing behind the field.
func TestStaleResultsAreDropped(t *testing.T) {
	l := New(&fakeProvider{backends: map[string]backend.Backend{}}, Options{})
	l.mu.Lock()
	l.levels = []*Level{{}}
	l.gen = 7
	l.mu.Unlock()

	if l.apply(Result{gen: 6, depth: 1, matches: []rank.Match{{Source: "old"}}}) {
		t.Fatal("apply accepted a stale result")
	}
	if !l.apply(Result{gen: 7, depth: 1, matches: []rank.Match{{Source: "new"}}}) {
		t.Fatal("apply rejected the current result")
	}
	if got := l.Results(); len(got) != 1 || got[0].Source != "new" {
		t.Errorf("results = %v, want the current generation", got)
	}
}

// TestPollWithoutWorkerIsANoOp keeps the inline path safe for callers that
// never start the worker.
func TestPollWithoutWorkerIsANoOp(t *testing.T) {
	l := New(&fakeProvider{backends: map[string]backend.Backend{}}, Options{})
	l.Poll()
	if got := l.Results(); len(got) != 0 {
		t.Errorf("results = %v, want none", got)
	}
}

// TestEnterQueuesHostAction proves Enter hands the selected item's host
// action to the host: the launcher queues it for the capability broker and
// TakeAction drains it exactly once. Before this, Enter silently dropped
// host actions in the GUI host.
func TestEnterQueuesHostAction(t *testing.T) {
	l, _ := newTestLauncher()
	l.Open()
	l.SetMode("apps")
	if len(l.Results()) == 0 {
		t.Fatal("no results in apps mode")
	}
	wantItem := l.Results()[0].Item
	l.Enter()
	a, ok := l.TakeAction()
	if !ok {
		t.Fatal("TakeAction after Enter = none, want the exec action")
	}
	if a.Type != "exec" || a.Value != wantItem.Actions[0].Value {
		t.Errorf("action = %v %q, want exec %q", a.Type, a.Value, wantItem.Actions[0].Value)
	}
	if _, ok := l.TakeAction(); ok {
		t.Error("TakeAction drained the same action twice")
	}
}

// TestEnterTriggersHighlightedRankedItem proves Enter triggers the action of
// the highlighted item from the ranked results, NOT the unranked raw items.
func TestEnterTriggersHighlightedRankedItem(t *testing.T) {
	apps := &listBackend{id: "apps", items: []backend.Item{
		{ID: "app1", Title: "Calculator", Actions: []backend.Action{{Type: "exec", Value: "calc"}}},
		{ID: "app2", Title: "Firefox", Actions: []backend.Action{{Type: "exec", Value: "firefox"}}},
	}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"apps": apps}},
		Options{Searchable: []string{"apps"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("apps")
	// Search for "fire" so Firefox is ranked #1 (index 0).
	l.SetQuery("fire")
	if len(l.Results()) == 0 || l.Results()[0].Item.Title != "Firefox" {
		t.Fatalf("top result = %+v, want Firefox", l.Results())
	}
	l.Enter()
	a, ok := l.TakeAction()
	if !ok {
		t.Fatal("TakeAction after Enter = none")
	}
	if a.Value != "firefox" {
		t.Errorf("action.Value = %q, want firefox (triggered wrong item!)", a.Value)
	}
}

// TestEnterTriggersSelectedItemAfterSelect proves that moving the selection
// with Select(1) triggers the newly selected item, not the first item.
func TestEnterTriggersSelectedItemAfterSelect(t *testing.T) {
	apps := &listBackend{id: "apps", items: []backend.Item{
		{ID: "app1", Title: "Alacritty", Actions: []backend.Action{{Type: "exec", Value: "alacritty"}}},
		{ID: "app2", Title: "Btop", Actions: []backend.Action{{Type: "exec", Value: "btop"}}},
	}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"apps": apps}},
		Options{Searchable: []string{"apps"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("apps")
	if len(l.Results()) < 2 {
		t.Fatalf("results count = %d, want at least 2", len(l.Results()))
	}
	secondItem := l.Results()[1].Item
	l.Select(1)
	if l.Selected() != 1 {
		t.Fatalf("l.Selected() = %d, want 1", l.Selected())
	}
	l.Enter()
	a, ok := l.TakeAction()
	if !ok {
		t.Fatal("TakeAction after Enter = none")
	}
	if a.Value != secondItem.Actions[0].Value {
		t.Errorf("action.Value = %q, want %q for second item", a.Value, secondItem.Actions[0].Value)
	}
}

// TestEnterCarriesMatchSourceInMergedMode proves Enter routes plugin actions
// to the item's own backend source even when running in merged mode.
func TestEnterCarriesMatchSourceInMergedMode(t *testing.T) {
	audio := &invokingBackend{listBackend: listBackend{
		id: "audio",
		items: []backend.Item{
			{ID: "vol", Title: "Volume", Actions: []backend.Action{{Type: "plugin", Value: "mute"}}},
		},
	}}
	prov := &fakeProvider{backends: map[string]backend.Backend{
		"audio": audio,
	}}
	l := New(prov, Options{Searchable: []string{"audio"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("") // merged mode
	if len(l.Results()) == 0 {
		t.Fatal("no results in merged mode")
	}
	l.Enter()
	if len(audio.invocations) != 1 {
		t.Fatalf("audio invocations = %d, want 1", len(audio.invocations))
	}
	if audio.invocations[0].Action != "mute" {
		t.Errorf("invocation = %+v, want mute", audio.invocations[0])
	}
}

// TestSelectLogsSelectionChange proves selection changes emit a debug log with
// item details (index, id, title, source).
func TestSelectLogsSelectionChange(t *testing.T) {
	th := &testLogHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(th))
	t.Cleanup(func() { slog.SetDefault(old) })

	apps := &listBackend{id: "apps", items: []backend.Item{
		{ID: "app1", Title: "Alacritty", Actions: []backend.Action{{Type: "exec", Value: "alacritty"}}},
		{ID: "app2", Title: "Btop", Actions: []backend.Action{{Type: "exec", Value: "btop"}}},
	}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"apps": apps}},
		Options{Searchable: []string{"apps"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("apps")
	l.Select(1)

	th.mu.Lock()
	defer th.mu.Unlock()
	found := false
	for _, rec := range th.records {
		if rec.Message == "selection changed" {
			found = true
			var idx int64
			var title, id string
			rec.Attrs(func(a slog.Attr) bool {
				switch a.Key {
				case "index":
					idx = a.Value.Int64()
				case "title":
					title = a.Value.String()
				case "id":
					id = a.Value.String()
				}
				return true
			})
			if idx != 1 {
				t.Errorf("logged index = %d, want 1", idx)
			}
			wantItem := l.Results()[1].Item
			if title != wantItem.Title {
				t.Errorf("logged title = %q, want %q", title, wantItem.Title)
			}
			if id != string(wantItem.ID) {
				t.Errorf("logged id = %q, want %q", id, wantItem.ID)
			}
			break
		}
	}
	if !found {
		t.Error("expected 'selection changed' debug log not found")
	}
}

// TestSetQueryResetsSelection proves that typing a new query resets the selection
// back to index 0 so the top result is highlighted.
func TestSetQueryResetsSelection(t *testing.T) {
	apps := &listBackend{id: "apps", items: []backend.Item{
		{ID: "app1", Title: "Alacritty", Actions: []backend.Action{{Type: "exec", Value: "alacritty"}}},
		{ID: "app2", Title: "Btop", Actions: []backend.Action{{Type: "exec", Value: "btop"}}},
	}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"apps": apps}},
		Options{Searchable: []string{"apps"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("apps")
	l.Select(1)
	if l.Selected() != 1 {
		t.Fatalf("l.Selected() = %d, want 1", l.Selected())
	}
	l.SetQuery("ala")
	if l.Selected() != 0 {
		t.Errorf("l.Selected() after SetQuery = %d, want 0", l.Selected())
	}
}

// TestEnterHostActionClosesOverlay checks activating a host action dismisses
// the overlay: the launched or focused window needs the keyboard back.
func TestEnterHostActionClosesOverlay(t *testing.T) {
	l, _ := newTestLauncher()
	l.Open()
	l.SetMode("apps")
	l.Enter()
	if l.IsOpen() {
		t.Error("overlay still open after a host action")
	}
}

// TestEnterPluginActionClosesOverlay checks activating a plugin domain action
// (like a systray leaf item or applet trigger) dismisses the overlay.
func TestEnterPluginActionClosesOverlay(t *testing.T) {
	tray := &invokingBackend{listBackend: listBackend{
		id: "tray",
		items: []backend.Item{
			{ID: "quit", Title: "Quit", Actions: []backend.Action{{Type: "plugin", Value: "invoke-menu"}}},
		},
	}}
	prov := &fakeProvider{backends: map[string]backend.Backend{"tray": tray}}
	l := New(prov, Options{Searchable: []string{"tray"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("tray")
	if len(l.Results()) != 1 {
		t.Fatalf("results len = %d, want 1", len(l.Results()))
	}
	l.Enter()
	if len(tray.invocations) != 1 {
		t.Fatalf("tray invocations = %d, want 1", len(tray.invocations))
	}
	if l.IsOpen() {
		t.Error("overlay still open after a plugin action, want it closed")
	}
}

// TestEnterDescendKeepsOverlayOpen proves that activating an item with sub-items
// descends instead of closing the overlay.
func TestEnterDescendKeepsOverlayOpen(t *testing.T) {
	tray := &listBackend{id: "tray", items: []backend.Item{
		{ID: "applet", Title: "Applet", HasChildren: true},
	}}
	prov := &fakeProvider{backends: map[string]backend.Backend{"tray": tray}}
	l := New(prov, Options{Searchable: []string{"tray"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("tray")
	l.Enter()
	if !l.IsOpen() {
		t.Error("overlay closed when descending into sub-items, want it open")
	}
	if depth := len(l.levels); depth != 2 {
		t.Errorf("levels depth = %d, want 2", depth)
	}
}

// TestPanelActionClosesOverlay proves activating an action row from a control
// panel runs the action and closes the overlay.
func TestPanelActionClosesOverlay(t *testing.T) {
	l, prov := newTestLauncher()
	prov.backends["apps"] = &listBackend{id: "apps", items: []backend.Item{
		{
			ID: "tool", Title: "Tool",
			Actions:  []backend.Action{{Type: "exec", Value: "tool run"}},
			Controls: []backend.Control{{ID: "c1", Kind: backend.ControlRange, Value: 0.5}},
		},
	}}
	l.Open()
	l.SetMode("apps")
	l.Enter() // Opens the control panel because it has interactive controls
	if !l.PanelOpen() {
		t.Fatal("control panel should be open")
	}
	l.PanelActivate() // Activates row 0 (the action row)
	if l.IsOpen() {
		t.Error("overlay still open after panel action activation, want it closed")
	}
	if l.PanelOpen() {
		t.Error("panel still open after action activation, want it closed")
	}
}

// TestSetQueryActionStaysInternal guards the action queue: set-query changes
// the query in place and must never reach the capability broker.
func TestSetQueryActionStaysInternal(t *testing.T) {
	apps := &listBackend{id: "apps", items: []backend.Item{
		{ID: "a", Title: "Again", Actions: []backend.Action{{Type: "set-query", Value: "next"}}},
	}}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"apps": apps}},
		Options{Searchable: []string{"apps"}, RankOpts: rank.Options{Limit: 10}})
	l.Open()
	l.SetMode("apps")
	l.Enter()
	if _, ok := l.TakeAction(); ok {
		t.Error("set-query leaked into the host action queue")
	}
	if l.Query() != "next" {
		t.Errorf("query = %q, want next", l.Query())
	}
	if !l.IsOpen() {
		t.Error("set-query closed the overlay")
	}
}

// TestSearchingTracksPendingWork proves Searching is true from submit until
// the current generation's results land, so the UI can show a loading
// indicator while the merged search is in flight.
func TestSearchingTracksPendingWork(t *testing.T) {
	gate := make(chan struct{})
	b := &blockingBackend{id: "slow", items: []backend.Item{{ID: "x", Title: "x"}}, gate: gate}
	l := New(&fakeProvider{backends: map[string]backend.Backend{"slow": b}},
		Options{Searchable: []string{"slow"}, RankOpts: rank.Options{Limit: 10}})
	l.Start()

	l.SetQuery("x")
	if !l.Searching() {
		t.Fatal("Searching = false while the query is in flight")
	}
	gate <- struct{}{}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		l.Poll()
		if !l.Searching() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("Searching stayed true after results published")
}

// TestSearchingStaysUntilNewestGeneration keeps the indicator up when a
// superseded result is dropped: only the newest generation's landing marks
// the search as finished.
func TestSearchingStaysUntilNewestGeneration(t *testing.T) {
	l := New(&fakeProvider{backends: map[string]backend.Backend{}}, Options{})
	l.mu.Lock()
	l.levels = []*Level{{}}
	l.gen = 7
	l.searching = true
	l.mu.Unlock()

	if l.apply(Result{gen: 6, depth: 1, matches: []rank.Match{{Source: "old"}}}) {
		t.Fatal("apply accepted a stale result")
	}
	if !l.Searching() {
		t.Error("Searching cleared while the newest search is still pending")
	}
	if !l.apply(Result{gen: 7, depth: 1, matches: []rank.Match{{Source: "new"}}}) {
		t.Fatal("apply rejected the current result")
	}
	if l.Searching() {
		t.Error("Searching stayed true after the current result landed")
	}
}

// TestQuerySyncLeavesStateAlone proves the ad-hoc query path ranks results
// without touching launcher state.
func TestQuerySyncLeavesStateAlone(t *testing.T) {
	l, _ := newTestLauncher()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	matches, err := l.QuerySync(ctx, "apps", "ghost", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Item.Title != "Ghostty" || matches[0].Source != "apps" {
		t.Fatalf("matches = %+v, want Ghostty from apps", matches)
	}
	if l.Query() != "" || l.IsOpen() {
		t.Error("QuerySync mutated launcher state")
	}
	if _, err := l.QuerySync(ctx, "nope", "x", 10); err == nil {
		t.Error("unknown backend must error")
	}
}

func TestCalcModeDisplaysComputedResult(t *testing.T) {
	calcB := &listBackend{
		id:            "calc",
		nonSearchable: true,
		items:         []backend.Item{{ID: "result", Title: "4", Subtitle: "Calculator"}},
	}
	prov := &fakeProvider{backends: map[string]backend.Backend{
		"calc": calcB,
	}}
	l := New(prov, Options{
		Modes:    []string{MergedMode, "calc"},
		RankOpts: rank.Options{Limit: 10},
	})
	l.Open()
	l.SetMode("calc")
	l.SetQuery("2 + 2")
	results := l.Results()
	if len(results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(results))
	}
	if results[0].Item.Title != "4" {
		t.Errorf("Title = %q, want 4", results[0].Item.Title)
	}
}

// TestTriggerPrefixSwitchesMode proves typing a registered prefix trigger like "=" switches mode.
func TestTriggerPrefixSwitchesMode(t *testing.T) {
	calcB := &listBackend{
		id:            "calc",
		trigger:       "=",
		nonSearchable: true,
		items:         []backend.Item{{ID: "result", Title: "4", Subtitle: "Calculator"}},
	}
	prov := &fakeProvider{backends: map[string]backend.Backend{
		"calc": calcB,
	}}
	l := New(prov, Options{
		Modes:    []string{MergedMode, "calc"},
		RankOpts: rank.Options{Limit: 10},
	})
	l.Open()
	if l.Mode() != MergedMode {
		t.Fatalf("initial mode = %q, want %q", l.Mode(), MergedMode)
	}
	l.SetQuery("= 2 + 2")
	if l.Mode() != "calc" {
		t.Errorf("Mode after trigger = %q, want calc", l.Mode())
	}
	if l.Query() != "2 + 2" {
		t.Errorf("Query after trigger = %q, want 2 + 2", l.Query())
	}
}

// TestCycleModeWrapsAndKeepsQuery proves Tab cycling walks the configured
// mode list, wraps at both ends, and preserves the typed query.
func TestCycleModeWrapsAndKeepsQuery(t *testing.T) {
	prov := &fakeProvider{backends: map[string]backend.Backend{
		"apps": &listBackend{id: "apps", items: []backend.Item{{ID: "a", Title: "Ghostty"}}},
		"calc": &listBackend{id: "calc", items: []backend.Item{{ID: "c", Title: "42"}}},
	}}
	l := New(prov, Options{
		Modes:    []string{MergedMode, "apps", "calc"},
		SettleMs: 20 * time.Millisecond,
		RankOpts: rank.Options{Limit: 10},
	})
	l.Open()
	l.SetQuery("ghost")

	if got := l.Mode(); got != MergedMode {
		t.Fatalf("Mode = %q, want %q", got, MergedMode)
	}
	l.CycleMode(1)
	if got := l.Mode(); got != "apps" {
		t.Fatalf("CycleMode(+1) from merged: Mode = %q, want apps", got)
	}
	l.CycleMode(1)
	if got := l.Mode(); got != "calc" {
		t.Fatalf("CycleMode(+1): Mode = %q, want calc", got)
	}
	l.CycleMode(1)
	if got := l.Mode(); got != MergedMode {
		t.Fatalf("CycleMode past the end: Mode = %q, want %q", got, MergedMode)
	}
	if got := l.ModeBadge(); got != "SEARCH" {
		t.Errorf("ModeBadge = %q, want SEARCH in merged mode", got)
	}
	l.CycleMode(-1)
	if got := l.Mode(); got != "calc" {
		t.Fatalf("CycleMode(-1) before the start: Mode = %q, want calc", got)
	}
	if got := l.Query(); got != "ghost" {
		t.Errorf("Query = %q after cycling, want %q preserved", got, "ghost")
	}
}

// TestModesDefaultsToMergedPlusSearchable pins the fallback cycle list.
func TestModesDefaultsToMergedPlusSearchable(t *testing.T) {
	l, _ := newTestLauncher()
	got := l.Modes()
	want := []string{MergedMode, "apps", "files", "windows"}
	if len(got) != len(want) {
		t.Fatalf("Modes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Modes = %v, want %v", got, want)
		}
	}
}

// barrierBackend blocks its Query until the partner backend is inside its own
// Query, so a sequential merged fan-out would deadlock and trip the deadline.
type barrierBackend struct {
	id      string
	entered chan<- struct{}
	proceed <-chan struct{}
}

func (b *barrierBackend) Info() backend.Info {
	return backend.Info{ID: b.id, Searchable: true, StableIDs: true}
}
func (b *barrierBackend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	b.entered <- struct{}{}
	<-b.proceed
	return &sliceStream{items: []backend.Item{{ID: backend.ResultID(b.id + "-1"), Title: b.id + " item"}}}, nil
}
func (b *barrierBackend) Control(context.Context, backend.Change) error { return nil }
func (b *barrierBackend) Close() error                                  { return nil }

// TestMergedQueryRunsBackendsConcurrently proves the merged fan-out does not
// run backends one after another: two mutually-blocking backends must both
// complete within the deadline.
func TestMergedQueryRunsBackendsConcurrently(t *testing.T) {
	aEntered := make(chan struct{}, 1)
	bEntered := make(chan struct{}, 1)
	a := &barrierBackend{id: "a", entered: aEntered, proceed: bEntered}
	b := &barrierBackend{id: "b", entered: bEntered, proceed: aEntered}
	prov := &fakeProvider{backends: map[string]backend.Backend{"a": a, "b": b}}
	l := New(prov, Options{
		Searchable: []string{"a", "b"},
		SettleMs:   20 * time.Millisecond,
		RankOpts:   rank.Options{Limit: 10},
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		l.Open()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("merged query deadlocked: backends ran sequentially")
	}
	// Both backends answered and their groups merged in list order.
	if got := len(l.Results()); got != 2 {
		t.Fatalf("Results len = %d, want 2", got)
	}
}

func TestMergedQueryStreamsProgressiveResults(t *testing.T) {
	fast := &listBackend{id: "fast", items: []backend.Item{{ID: "fast-1", Title: "Fast Result"}}}
	slowProceed := make(chan struct{})
	slowEntered := make(chan struct{}, 1)
	slow := &barrierBackend{id: "slow", entered: slowEntered, proceed: slowProceed}

	prov := &fakeProvider{backends: map[string]backend.Backend{"fast": fast, "slow": slow}}
	l := New(prov, Options{
		Searchable: []string{"fast", "slow"},
		SettleMs:   20 * time.Millisecond,
		RankOpts:   rank.Options{Limit: 10},
	})
	l.Start()
	l.Open()

	// Wait until slow backend enters Query
	select {
	case <-slowEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("slow backend never entered query")
	}

	// Poll launcher: fast results should already be published while slow backend is blocked
	deadline := time.Now().Add(1 * time.Second)
	foundFast := false
	for time.Now().Before(deadline) {
		l.Poll()
		res := l.Results()
		if len(res) == 1 && res[0].Item.ID == "fast-1" {
			foundFast = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !foundFast {
		t.Fatalf("fast backend results were not streamed while slow backend was blocked: got %+v", l.Results())
	}
	if !l.Searching() {
		t.Error("Searching = false while slow backend is still in flight")
	}

	// Release slow backend
	close(slowProceed)

	// Wait for slow backend to complete and land in Poll
	foundBoth := false
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		l.Poll()
		if len(l.Results()) == 2 && !l.Searching() {
			foundBoth = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !foundBoth {
		t.Fatalf("results never completed with both backends: got %d results, searching=%v", len(l.Results()), l.Searching())
	}
	// Verify selection stayed on fast-1
	selectedItem, _, ok := l.SelectedItem()
	if !ok || selectedItem.ID != "fast-1" {
		t.Errorf("SelectedItem = %+v, want fast-1 preserved after progressive update", selectedItem)
	}
}

func TestLauncherExternalPluginNonSearchable(t *testing.T) {
	// Represents an external plugin declaring Searchable: false and Trigger: "="
	extPlugin := &listBackend{
		id:            "extcalc",
		trigger:       "=",
		nonSearchable: true,
		items:         []backend.Item{{ID: "res", Title: "42", Subtitle: "External Calculator"}},
	}
	appsB := &listBackend{
		id:    "apps",
		items: []backend.Item{{ID: "app1", Title: "Terminal"}},
	}

	prov := &fakeProvider{backends: map[string]backend.Backend{
		"extcalc": extPlugin,
		"apps":    appsB,
	}}

	// In keyshell, non-searchable plugins are in Modes but NOT Searchable
	l := New(prov, Options{
		Searchable: []string{"apps"},
		Modes:      []string{MergedMode, "apps", "extcalc"},
		RankOpts:   rank.Options{Limit: 10},
		SettleMs:   20 * time.Millisecond,
	})
	l.Open()

	// 1. In merged search, querying should NOT query extcalc or return its items
	l.SetQuery("42")
	if extPlugin.query.Text == "42" {
		t.Fatalf("extcalc backend was queried during merged search")
	}
	results := l.Results()
	for _, m := range results {
		if m.Source == "extcalc" || m.Item.ID == "res" {
			t.Fatalf("merged search returned result from non-searchable plugin: %+v", m)
		}
	}

	// 2. Typing the trigger prefix "=" should switch mode to extcalc and return its items
	l.SetQuery("= 21 * 2")
	if l.Mode() != "extcalc" {
		t.Fatalf("Mode after trigger prefix = %q, want extcalc", l.Mode())
	}
	if l.Query() != "21 * 2" {
		t.Errorf("Query after trigger = %q, want 21 * 2", l.Query())
	}
	calcResults := l.Results()
	if len(calcResults) == 0 {
		t.Fatal("expected results after trigger prefix, got none")
	}
	if calcResults[0].Item.Title != "42" {
		t.Errorf("expected Title = 42, got %q", calcResults[0].Item.Title)
	}
}
