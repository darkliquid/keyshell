// Package launcher is the state machine of the overlay: mode, query,
// selection, the navigation stack, and the action/control routing. It has no
// knowledge of rendering or input plumbing; it only mutates state and emits
// change notifications.
package launcher

import (
	"context"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/rank"
)

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

// Launcher is the overlay state.
type Launcher struct {
	provider backend.BackendProvider
	opts     Options

	open   bool
	mode   string
	query  string
	levels []*Level

	// per-level results for the current query
	results []rank.Match

	// change subscriptions
	onChange func()

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
	return &Launcher{provider: provider, opts: opts}
}

// OnChange registers a callback invoked after any state change.
func (l *Launcher) OnChange(fn func()) {
	l.onChange = fn
}

func (l *Launcher) changed() {
	if l.onChange != nil {
		l.onChange()
	}
}

// Open shows the overlay and runs the initial query.
func (l *Launcher) Open() {
	l.open = true
	l.resetToRoot()
	l.refresh()
	l.changed()
}

// Close hides the overlay.
func (l *Launcher) Close() {
	l.open = false
	l.changed()
}

// Toggle flips visibility.
func (l *Launcher) Toggle() {
	if l.open {
		l.Close()
	} else {
		l.Open()
	}
}

// IsOpen reports visibility.
func (l *Launcher) IsOpen() bool { return l.open }

// Mode returns the current backend mode id.
func (l *Launcher) Mode() string { return l.mode }

// SetMode switches the active backend and refreshes results.
func (l *Launcher) SetMode(id string) {
	l.mode = id
	l.resetToRoot()
	l.refresh()
	l.changed()
}

// Query returns the current query text.
func (l *Launcher) Query() string { return l.query }

// SetQuery updates the query and refreshes results.
func (l *Launcher) SetQuery(text string) {
	l.query = text
	l.refresh()
	l.changed()
}

// Results returns the current ranked results.
func (l *Launcher) Results() []rank.Match { return l.results }

// Selected returns the selected index within the current level.
func (l *Launcher) Selected() int {
	if len(l.levels) == 0 {
		return 0
	}
	return l.levels[len(l.levels)-1].Selected
}

// Select moves the selection by delta.
func (l *Launcher) Select(delta int) {
	level := l.currentLevel()
	if level == nil || len(level.Items) == 0 {
		return
	}
	n := len(level.Items)
	level.Selected = (level.Selected + delta + n) % n
	l.changed()
}

// refresh runs the query across the active backends and stores results.
func (l *Launcher) refresh() {
	ctx := context.Background()
	level := l.currentLevel()
	if level == nil {
		return
	}
	if l.mode == "all" {
		// merged search: all searchable backends
		ids := l.searchableIDs()
		groups := []rank.Group{}
		for _, id := range ids {
			b, err := l.provider.Get(id)
			if err != nil {
				continue
			}
			stream, err := b.Query(ctx, backend.Query{Text: l.query, Limit: l.opts.RankOpts.Limit, Merged: true})
			if err != nil {
				continue
			}
			items := drain(stream)
			stream.Close()
			groups = append(groups, rank.Group{Source: id, Items: items})
		}
		level.Items = flattenGroups(groups)
		level.Matches = rank.Rank(l.query, groups, nil, l.opts.RankOpts)
		l.results = level.Matches
	} else {
		b, err := l.provider.Get(l.mode)
		if err != nil {
			l.results = nil
			return
		}
		stream, err := b.Query(ctx, backend.Query{Text: l.query, Limit: l.opts.RankOpts.Limit, Merged: false})
		if err != nil {
			l.results = nil
			return
		}
		items := drain(stream)
		stream.Close()
		level.Items = items
		level.Matches = rank.RankOne(l.query, l.mode, items, nil, l.opts.RankOpts)
		l.results = level.Matches
	}
}

func (l *Launcher) searchableIDs() []string {
	if len(l.opts.Searchable) > 0 {
		return l.opts.Searchable
	}
	return []string{"apps", "files", "windows"}
}

func (l *Launcher) currentLevel() *Level {
	if len(l.levels) == 0 {
		l.levels = append(l.levels, &Level{Source: l.mode})
	}
	return l.levels[len(l.levels)-1]
}

func (l *Launcher) resetToRoot() {
	l.levels = []*Level{{Source: l.mode}}
}

// Enter routes the primary action of the selected item.
func (l *Launcher) Enter() {
	level := l.currentLevel()
	if level == nil || len(level.Items) == 0 {
		return
	}
	idx := level.Selected
	if idx < 0 || idx >= len(level.Items) {
		return
	}
	item := level.Items[idx]

	if len(item.Actions) > 0 {
		if item.Destructive {
			// Gating is checked by the input layer; here we route it anyway.
			// The caller decides whether to perform or require confirmation.
			l.changed()
			return
		}
		l.performAction(item.Actions[0], item)
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

// performAction handles the host actions for step 1. Domain/plugin actions are
// left to later steps.
func (l *Launcher) performAction(a backend.Action, item backend.Item) {
	switch a.Type {
	case "set-query":
		l.SetQuery(a.Value)
	case "copy", "open-url", "exec", "exec-terminal", "focus-window":
		l.changed() // the host capability broker picks this up
	}
}

// descend pushes a scoped level for an item's children.
func (l *Launcher) descend(item backend.Item) {
	parent := l.currentLevel()
	scope := append([]string{}, parent.Scope...)
	scope = append(scope, string(item.ID))
	l.levels = append(l.levels, &Level{
		Scope:  scope,
		Source: l.mode,
	})
	l.query = ""
	l.refresh()
	l.changed()
}

// Back pops a level.
func (l *Launcher) Back() {
	if len(l.levels) > 1 {
		l.levels = l.levels[:len(l.levels)-1]
		l.refresh()
		l.changed()
	} else if l.query != "" {
		l.SetQuery("")
	} else {
		l.Close()
	}
}

// Escape pops a level or dismisses at the root.
func (l *Launcher) Escape() {
	if len(l.levels) > 1 {
		l.Back()
	} else {
		l.Close()
	}
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
	level := l.currentLevel()
	if level == nil {
		return
	}
	b, err := l.provider.Get(l.mode)
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
	if l.debounce != nil {
		l.debounce.Stop()
	}
	l.debounce = time.AfterFunc(l.opts.SettleMs, func() {
		l.Control(itemID, controlID, value)
	})
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
