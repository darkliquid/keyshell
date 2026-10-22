package launcher

import (
	"context"
	"fmt"
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
	mu      sync.Mutex
	id      string
	items   []backend.Item
	query   backend.Query
	control []backend.Change
}

func (b *listBackend) Info() backend.Info {
	return backend.Info{ID: b.id, Searchable: true, StableIDs: true}
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
	calls := apps.ControlCalls()
	if len(calls) != 1 {
		t.Errorf("control calls = %d, want 1 (coalesced)", len(calls))
	}
	if len(calls) > 0 && calls[0].Value != 4 {
		t.Errorf("value = %v, want 4 (last settled)", calls[0].Value)
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
	l.Enter()
	a, ok := l.TakeAction()
	if !ok {
		t.Fatal("TakeAction after Enter = none, want the exec action")
	}
	if a.Type != "exec" || a.Value != "ghostty" {
		t.Errorf("action = %v %q, want exec ghostty", a.Type, a.Value)
	}
	if _, ok := l.TakeAction(); ok {
		t.Error("TakeAction drained the same action twice")
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
