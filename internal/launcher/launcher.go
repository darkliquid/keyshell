// Package launcher is the state machine of the overlay: mode, query,
// selection, the navigation stack, and the action/control routing. It has no
// knowledge of rendering or input plumbing; it only mutates state and emits
// change notifications.
package launcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/rank"
)

// MergedMode is the mode id for the merged search across every searchable
// backend. It is the launcher's default mode.
const MergedMode = "all"

// Mode is a named result source (a backend id) or the merged search.
type Mode struct {
	ID string
}

// Level is one level of the navigation stack.
type Level struct {
	Scope    []string // scope path for scoped queries
	Source   string   // backend id
	Items    []backend.Item
	Matches  []rank.Match
	Query    string
	Selected int
}

// Launcher is the overlay state. State changes may arrive from the UI thread,
// the IPC goroutine and the query worker, so every field below mu is guarded
// by it. Queries never run under the lock: they take a snapshot, run off the
// UI thread, and publish through a channel.
type Launcher struct {
	provider backend.BackendProvider
	opts     Options

	mu      sync.RWMutex
	open    bool
	mode    string
	badge   string
	query   string
	levels  []*Level
	results []rank.Match
	gen     uint64

	// lastQuerySC carries the trace context of the most recently published
	// query, so the frame that paints the results can join the same trace.
	lastQuerySC trace.SpanContext

	// hostActions holds host-executed actions queued by Enter, awaiting the
	// host's capability broker. The launcher never performs effects itself;
	// it queues and the host drains.
	hostActions []backend.Action

	// searching is true while a query job is in flight, so the UI can paint
	// a loading indicator instead of an empty-looking card row.
	searching bool

	// change subscriptions
	listeners []func()

	// query worker, nil until Start
	jobs      chan Job
	resultsCh chan Result

	debounce *time.Timer
	settleMs time.Duration
}

// Options is the launcher configuration.
type Options struct {
	SettleMs time.Duration
	RankOpts rank.Options
	// Searchable is the set of backend ids that participate in the merged
	// search. calc/run/clipboard are intentionally absent.
	Searchable []string
}

// New builds a launcher over a backend provider.
func New(provider backend.BackendProvider, opts Options) *Launcher {
	if opts.SettleMs == 0 {
		opts.SettleMs = 80 * time.Millisecond
	}
	return &Launcher{provider: provider, opts: opts, mode: MergedMode, badge: "SEARCH"}
}

// OnChange registers a callback invoked after any state change. Several
// listeners may be registered (rendering, focus handling, ...).
func (l *Launcher) OnChange(fn func()) {
	if fn == nil {
		return
	}
	l.mu.Lock()
	l.listeners = append(l.listeners, fn)
	l.mu.Unlock()
}

func (l *Launcher) changed() {
	l.mu.RLock()
	fns := append([]func(){}, l.listeners...)
	l.mu.RUnlock()
	for _, fn := range fns {
		fn()
	}
}

// Open shows the overlay and starts the initial query. It returns before the
// query finishes: the overlay takes focus and paints immediately, and results
// arrive through Poll.
func (l *Launcher) Open() {
	l.mu.Lock()
	l.open = true
	l.resetToRootLocked()
	job := l.jobLocked()
	l.mu.Unlock()
	l.changed()
	l.submit(job)
}

// Close hides the overlay.
func (l *Launcher) Close() {
	l.mu.Lock()
	l.open = false
	l.mu.Unlock()
	l.changed()
}

// Toggle flips visibility.
func (l *Launcher) Toggle() {
	if l.IsOpen() {
		l.Close()
	} else {
		l.Open()
	}
}

// IsOpen reports visibility.
func (l *Launcher) IsOpen() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.open
}

// Mode returns the current backend mode id.
func (l *Launcher) Mode() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.mode
}

// ModeBadge returns the badge for the active mode (or SEARCH when merged).
func (l *Launcher) ModeBadge() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.badge
}

// Searching reports whether a query is in flight. It is set when a job is
// submitted to the worker and cleared when the current generation's results
// are published.
func (l *Launcher) Searching() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.searching
}

// SetMode switches the active backend and starts a query.
func (l *Launcher) SetMode(id string) {
	badge := "SEARCH"
	if id != "" && id != MergedMode {
		if b, err := l.provider.Get(id); err == nil {
			badge = b.Info().Badge
		}
	}
	l.mu.Lock()
	l.mode = id
	l.badge = badge
	l.resetToRootLocked()
	job := l.jobLocked()
	l.mu.Unlock()
	l.changed()
	l.submit(job)
}

// Query returns the current query text.
func (l *Launcher) Query() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.query
}

// SetQuery updates the query and starts a search. Without a worker the search
// runs inline, which keeps tests and one-shot callers synchronous; with a
// worker it returns immediately and Poll publishes the results.
func (l *Launcher) SetQuery(text string) {
	l.mu.Lock()
	l.query = text
	job := l.jobLocked()
	l.mu.Unlock()
	l.changed()
	l.submit(job)
}

// SetQueryInput updates the query text without searching, so the field repaints
// on the keystroke itself. Typing calls this and schedules SetQuery for when
// the keys settle.
func (l *Launcher) SetQueryInput(text string) {
	l.mu.Lock()
	l.query = text
	l.mu.Unlock()
	l.changed()
}

// SetQueryContext updates the query and starts a search whose trace joins the
// given context, letting an input event carry its trace through to the query
// and the plugin that answers it.
func (l *Launcher) SetQueryContext(ctx context.Context, text string) {
	l.mu.Lock()
	l.query = text
	job := l.jobLocked()
	job.ctx = ctx
	l.mu.Unlock()
	l.changed()
	l.submit(job)
}

// Results returns the current ranked results.
func (l *Launcher) Results() []rank.Match {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.results
}

// Selected returns the selected index within the current level.
func (l *Launcher) Selected() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.levels) == 0 {
		return 0
	}
	return l.levels[len(l.levels)-1].Selected
}

// Select moves the selection by delta.
func (l *Launcher) Select(delta int) {
	l.mu.Lock()
	level := l.currentLevelLocked()
	if level == nil || len(level.Items) == 0 {
		l.mu.Unlock()
		return
	}
	n := len(level.Items)
	level.Selected = (level.Selected + delta + n) % n
	l.mu.Unlock()
	l.changed()
}

func (l *Launcher) searchableIDs() []string {
	if len(l.opts.Searchable) > 0 {
		return l.opts.Searchable
	}
	return []string{"apps", "files", "windows"}
}

func (l *Launcher) currentLevelLocked() *Level {
	if len(l.levels) == 0 {
		l.levels = append(l.levels, &Level{Source: l.mode})
	}
	return l.levels[len(l.levels)-1]
}

func (l *Launcher) resetToRootLocked() {
	l.levels = []*Level{{Source: l.mode}}
}

// Enter routes the primary action of the selected item.
func (l *Launcher) Enter() {
	// currentLevelLocked may create the root level, so this takes the write
	// lock rather than the read lock.
	l.mu.Lock()
	level := l.currentLevelLocked()
	if level == nil || len(level.Items) == 0 {
		l.mu.Unlock()
		return
	}
	idx := level.Selected
	if idx < 0 || idx >= len(level.Items) {
		l.mu.Unlock()
		return
	}
	item := level.Items[idx]
	source := l.mode
	if level.Source != "" {
		source = level.Source
	}
	l.mu.Unlock()

	slog.Debug("enter", "item", item.ID, "title", item.Title, "source", source, "actions", len(item.Actions))
	if len(item.Actions) > 0 {
		if item.Destructive {
			// Gating is checked by the input layer; here we route it anyway.
			// The caller decides whether to perform or require confirmation.
			l.changed()
			return
		}
		l.performAction(item.Actions[0], item, source)
		return
	}
	if item.Expandable {
		l.openFocusView(item)
		return
	}
	if item.HasChildren {
		l.descend(item)
	}
}

// performAction handles host actions and routes domain ("plugin") actions to
// the backend that produced the item.
func (l *Launcher) performAction(a backend.Action, item backend.Item, source string) {
	switch a.Type {
	case "set-query":
		l.SetQuery(a.Value)
	case "plugin":
		l.invokeDomain(a, item, source)
	case "copy", "open-url", "exec", "exec-terminal", "focus-window":
		// Queue for the host capability broker, then dismiss: the launched
		// or focused window needs the keyboard back.
		slog.Debug("host action queued", "type", a.Type, "value", a.Value)
		l.mu.Lock()
		l.hostActions = append(l.hostActions, a)
		l.mu.Unlock()
		l.Close()
	}
}

// TakeAction pops the oldest queued host action. The host drains the queue
// after each change notification and performs it through its capability
// broker.
func (l *Launcher) TakeAction() (backend.Action, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hostActions) == 0 {
		return backend.Action{}, false
	}
	a := l.hostActions[0]
	l.hostActions = l.hostActions[1:]
	return a, true
}

func (l *Launcher) invokeDomain(a backend.Action, item backend.Item, source string) {
	b, err := l.provider.Get(source)
	if err != nil {
		return
	}
	if inv, ok := b.(backend.Invoker); ok {
		slog.Debug("plugin action", "source", source, "item", item.ID, "action", a.Value)
		_ = inv.Invoke(context.Background(), backend.Invoke{
			Item: item.ID, Action: a.Value, Value: a.Value,
		})
	}
	l.changed()
}

// descend pushes a scoped level for an item's children.
func (l *Launcher) descend(item backend.Item) {
	l.mu.Lock()
	parent := l.currentLevelLocked()
	scope := append([]string{}, parent.Scope...)
	scope = append(scope, string(item.ID))
	l.levels = append(l.levels, &Level{
		Scope:  scope,
		Source: l.mode,
	})
	l.query = ""
	job := l.jobLocked()
	l.mu.Unlock()
	l.changed()
	l.submit(job)
}

// Back pops a level.
func (l *Launcher) Back() {
	l.mu.RLock()
	depth := len(l.levels)
	query := l.query
	l.mu.RUnlock()
	switch {
	case depth > 1:
		l.mu.Lock()
		l.levels = l.levels[:len(l.levels)-1]
		job := l.jobLocked()
		l.mu.Unlock()
		l.changed()
		l.submit(job)
	case query != "":
		l.SetQuery("")
	default:
		l.Close()
	}
}

// Escape pops a level or dismisses at the root.
func (l *Launcher) Escape() {
	l.mu.RLock()
	depth := len(l.levels)
	l.mu.RUnlock()
	if depth > 1 {
		l.Back()
		return
	}
	l.Close()
}

// ControlSource delivers a settled value change to a named backend, used by
// the IPC verbs to drive a plugin without opening the overlay.
func (l *Launcher) ControlSource(source string, c backend.Change) error {
	b, err := l.provider.Get(source)
	if err != nil {
		return err
	}
	return b.Control(context.Background(), c)
}

// Control delivers a settled value change to the current item's backend.
func (l *Launcher) Control(itemID string, controlID string, value float64) {
	b, err := l.provider.Get(l.Mode())
	if err != nil {
		return
	}
	_ = b.Control(context.Background(), backend.Change{
		Item:    backend.ResultID(itemID),
		Control: controlID,
		Value:   value,
	})
}

// DebouncedControl coalesces rapid control changes using the settle window.
func (l *Launcher) DebouncedControl(itemID, controlID string, value float64) {
	l.mu.Lock()
	if l.debounce != nil {
		l.debounce.Stop()
	}
	l.debounce = time.AfterFunc(l.opts.SettleMs, func() {
		l.Control(itemID, controlID, value)
	})
	l.mu.Unlock()
}

func (l *Launcher) openFocusView(item backend.Item) {
	l.changed() // UI decides to show the focus view for this item
}

func drain(s backend.Stream) []backend.Item {
	var items []backend.Item
	for {
		it, more, err := s.Next()
		if err != nil || !more {
			break
		}
		items = append(items, it)
	}
	return items
}

func flattenGroups(groups []rank.Group) []backend.Item {
	var items []backend.Item
	for _, g := range groups {
		items = append(items, g.Items...)
	}
	return items
}
