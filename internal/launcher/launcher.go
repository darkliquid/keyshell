// Package launcher is the state machine of the overlay: mode, query,
// selection, the navigation stack, and the action/control routing. It has no
// knowledge of rendering or input plumbing; it only mutates state and emits
// change notifications.
package launcher

import (
	"context"
	"log/slog"
	"strings"
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
	jobs        chan Job
	resultsCh   chan Result
	cancelQuery context.CancelFunc

	debounce *time.Timer
	settleMs time.Duration

	// panel is the open control panel, nil when closed.
	panel *panelState

	// panelDebounce coalesces rapid range adjustments, mirroring debounce.
	panelDebounce *time.Timer
}

// panelState is the launcher-owned control panel. rows is the ordered list of
// interactive rows (action row first when present, then controls, progress
// excluded); focus indexes into it; edit is the live text buffer of the
// focused text row while it is being edited.
type panelState struct {
	item backend.Item
	source string
	rows   []backend.PanelRow
	focus  int
	edit   string
}

// Options is the launcher configuration.
type Options struct {
	SettleMs time.Duration
	RankOpts rank.Options
	// Searchable is the set of backend ids that participate in the merged
	// search. calc/run/clipboard are intentionally absent.
	Searchable []string
	// Modes is the Tab cycle list: the merged search first, then every mode
	// a keypress can enter directly. Empty derives it from Searchable.
	Modes []string
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
	if l.cancelQuery != nil {
		l.cancelQuery()
		l.cancelQuery = nil
	}
	l.closePanelLocked()
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

// Modes returns the mode ids Tab cycles through, merged search first.
func (l *Launcher) Modes() []string {
	if len(l.opts.Modes) > 0 {
		return l.opts.Modes
	}
	return append([]string{MergedMode}, l.searchableIDs()...)
}

// CycleMode switches to the next (delta 1) or previous (delta -1) mode in the
// cycle, keeping the typed query, and starts a search in the new mode.
func (l *Launcher) CycleMode(delta int) {
	modes := l.Modes()
	if len(modes) < 2 {
		return
	}
	l.mu.RLock()
	cur := l.mode
	l.mu.RUnlock()
	idx := 0
	for i, m := range modes {
		if m == cur {
			idx = i
			break
		}
	}
	l.SetMode(modes[(idx+delta+len(modes))%len(modes)])
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
	if _, rem, ok := l.tryTriggerLocked(text); ok {
		text = rem
	}
	l.query = text
	if level := l.currentLevelLocked(); level != nil {
		level.Selected = 0
	}
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
	if _, rem, ok := l.tryTriggerLocked(text); ok {
		text = rem
	}
	l.query = text
	if level := l.currentLevelLocked(); level != nil {
		level.Selected = 0
	}
	l.mu.Unlock()
	l.changed()
}

// SetQueryContext updates the query and starts a search whose trace joins the
// given context, letting an input event carry its trace through to the query
// and the plugin that answers it.
func (l *Launcher) SetQueryContext(ctx context.Context, text string) {
	l.mu.Lock()
	if _, rem, ok := l.tryTriggerLocked(text); ok {
		text = rem
	}
	l.query = text
	if level := l.currentLevelLocked(); level != nil {
		level.Selected = 0
	}
	job := l.jobLocked()
	if ctx != nil {
		cctx, cancel := context.WithCancel(ctx)
		l.cancelQuery = cancel
		job.ctx = cctx
	}
	l.mu.Unlock()
	l.changed()
	l.submit(job)
}

func (l *Launcher) tryTriggerLocked(text string) (newMode string, query string, matched bool) {
	if len(l.levels) > 1 {
		return "", text, false
	}
	var bestTrigger, bestID, bestBadge string
	for _, m := range l.Modes() {
		if m == MergedMode || m == "" {
			continue
		}
		b, err := l.provider.Get(m)
		if err != nil {
			continue
		}
		info := b.Info()
		trig := info.Trigger
		if trig != "" && strings.HasPrefix(text, trig) {
			if len(trig) > len(bestTrigger) {
				bestTrigger = trig
				bestID = info.ID
				bestBadge = info.Badge
			}
		}
	}
	if bestTrigger != "" {
		rem := strings.TrimPrefix(text, bestTrigger)
		rem = strings.TrimLeft(rem, " ")
		if bestID != l.mode {
			l.mode = bestID
			if bestBadge == "" {
				bestBadge = strings.ToUpper(bestID)
			}
			l.badge = bestBadge
			l.resetToRootLocked()
		}
		return bestID, rem, true
	}
	return "", text, false
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

// SelectedItem returns the currently selected item, its source backend, and
// whether a valid item is selected.
func (l *Launcher) SelectedItem() (backend.Item, string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	level := l.currentLevelLocked()
	return l.selectedItemLocked(level)
}

func (l *Launcher) countLocked(level *Level) int {
	if level == nil {
		return 0
	}
	return len(level.Matches)
}

func (l *Launcher) selectedItemLocked(level *Level) (backend.Item, string, bool) {
	if level == nil {
		return backend.Item{}, "", false
	}
	idx := level.Selected
	if len(level.Matches) == 0 || idx < 0 || idx >= len(level.Matches) {
		return backend.Item{}, "", false
	}
	m := level.Matches[idx]
	source := m.Source
	if source == "" {
		source = level.Source
	}
	if source == "" {
		source = l.mode
	}
	return m.Item, source, true
}

// Select moves the selection by delta within the current level's results.
func (l *Launcher) Select(delta int) {
	l.mu.Lock()
	level := l.currentLevelLocked()
	n := l.countLocked(level)
	if n == 0 {
		l.mu.Unlock()
		return
	}
	prev := level.Selected
	level.Selected = (level.Selected + delta + n) % n
	selected := level.Selected
	item, source, ok := l.selectedItemLocked(level)
	l.mu.Unlock()
	l.changed()
	if ok {
		slog.Debug("selection changed", "index", selected, "prev", prev, "id", item.ID, "title", item.Title, "source", source)
	}
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
	l.closePanelLocked()
}

// Enter routes the primary action of the selected item.
func (l *Launcher) Enter() {
	// currentLevelLocked may create the root level, so this takes the write
	// lock rather than the read lock.
	l.mu.Lock()
	level := l.currentLevelLocked()
	idx := 0
	if level != nil {
		idx = level.Selected
	}
	item, source, ok := l.selectedItemLocked(level)
	l.mu.Unlock()
	if !ok {
		return
	}

	slog.Debug("enter", "index", idx, "id", item.ID, "title", item.Title, "source", source, "actions", len(item.Actions))
	if hasInteractiveControls(item) {
		if l.openPanel(item, source) {
			return
		}
	}
	if item.HasChildren {
		l.descend(item)
		return
	}
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
}

// performAction handles host actions and routes domain ("plugin") actions to
// the backend that produced the item. An action execution (other than internal
// set-query) closes the overlay.
func (l *Launcher) performAction(a backend.Action, item backend.Item, source string) {
	switch a.Type {
	case "set-query":
		l.SetQuery(a.Value)
	case "plugin":
		l.invokeDomain(a, item, source)
		l.Close()
	case "copy", "open-url", "exec", "exec-terminal", "focus-window":
		// Queue for the host capability broker, then dismiss: the launched
		// or focused window needs the keyboard back.
		slog.Debug("host action queued", "type", a.Type, "value", a.Value)
		l.mu.Lock()
		l.hostActions = append(l.hostActions, a)
		l.mu.Unlock()
		l.Close()
	default:
		slog.Debug("host action queued (generic)", "type", a.Type, "value", a.Value)
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

// Back pops a level, or closes the panel first when one is open.
func (l *Launcher) Back() {
	l.mu.Lock()
	if l.panel != nil {
		l.closePanelLocked()
		l.mu.Unlock()
		l.changed()
		return
	}
	l.mu.Unlock()
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

// Escape closes the panel first, then pops a level or dismisses at the root.
func (l *Launcher) Escape() {
	l.mu.Lock()
	if l.panel != nil {
		l.closePanelLocked()
		l.mu.Unlock()
		l.changed()
		return
	}
	l.mu.Unlock()
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
	ch := backend.Change{
		Item:    backend.ResultID(itemID),
		Control: controlID,
		Value:   value,
	}
	l.mu.Lock()
	l.updateControlInPlaceLocked(backend.ResultID(itemID), ch)
	mode := l.mode
	l.mu.Unlock()
	l.changed()

	b, err := l.provider.Get(mode)
	if err != nil {
		return
	}
	_ = b.Control(context.Background(), ch)
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

// hasInteractiveControls reports whether an item exposes any editable control
// (anything but a read-only progress indicator).
func hasInteractiveControls(item backend.Item) bool {
	for _, c := range item.Controls {
		if c.Kind != backend.ControlProgress {
			return true
		}
	}
	return false
}

// panelRowsFor builds the ordered panel rows for an item: the action row
// first when the item has a primary action, then every non-progress control.
func panelRowsFor(item backend.Item) []backend.PanelRow {
	var rows []backend.PanelRow
	if len(item.Actions) > 0 {
		a := item.Actions[0]
		rows = append(rows, backend.PanelRow{Kind: backend.ControlKind("action"), Label: actionLabel(a), Action: &a})
	}
	for i := range item.Controls {
		c := item.Controls[i]
		if c.Kind == backend.ControlProgress {
			continue
		}
		cc := c
		rows = append(rows, backend.PanelRow{Kind: c.Kind, Label: c.Label, Control: &cc})
	}
	return rows
}

func actionLabel(a backend.Action) string {
	switch a.Type {
	case "copy":
		return "COPY"
	case "exec", "exec-terminal":
		return "RUN"
	case "open-url":
		return "OPEN"
	case "focus-window":
		return "FOCUS"
	case "plugin":
		return "ACTIVATE"
	case "set-query":
		return "SEARCH"
	}
	return "SELECT"
}

// openPanel opens the control panel for an item. It reports whether a panel
// opened (the item has interactive rows).
func (l *Launcher) openPanel(item backend.Item, source string) bool {
	l.mu.Lock()
	if l.panel != nil {
		l.mu.Unlock()
		return true
	}
	rows := panelRowsFor(item)
	if len(rows) == 0 {
		l.mu.Unlock()
		return false
	}
	l.panel = &panelState{item: item, source: source, rows: rows}
	l.mu.Unlock()
	l.changed()
	return true
}

// closePanelLocked clears the panel; callers must hold the write lock and call
// changed() after unlocking.
func (l *Launcher) closePanelLocked() {
	if l.panel == nil {
		return
	}
	l.panel = nil
	if l.panelDebounce != nil {
		l.panelDebounce.Stop()
		l.panelDebounce = nil
	}
}

// PanelOpen reports whether the control panel is open.
func (l *Launcher) PanelOpen() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.panel != nil
}

// PanelRows returns the ordered interactive rows of the open panel.
func (l *Launcher) PanelRows() []backend.PanelRow {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.panel == nil {
		return nil
	}
	out := make([]backend.PanelRow, len(l.panel.rows))
	copy(out, l.panel.rows)
	return out
}

// PanelFocus returns the focused row index, or 0 when closed.
func (l *Launcher) PanelFocus() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.panel == nil {
		return 0
	}
	return l.panel.focus
}

// PanelDirty reports whether the focused text row is being edited.
func (l *Launcher) PanelDirty() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.panel != nil && l.panel.edit != ""
}

// PanelNavigate moves focus by delta within the panel's rows, clamping.
func (l *Launcher) PanelNavigate(delta int) {
	l.mu.Lock()
	p := l.panel
	if p == nil || len(p.rows) == 0 {
		l.mu.Unlock()
		return
	}
	p.focus += delta
	if p.focus < 0 {
		p.focus = 0
	}
	if p.focus >= len(p.rows) {
		p.focus = len(p.rows) - 1
	}
	l.mu.Unlock()
	l.changed()
}

// PanelAdjust manipulates the focused control: a range steps its value, a
// choice cycles its options. Progress/action rows are no-ops.
func (l *Launcher) PanelAdjust(delta int) {
	l.mu.Lock()
	p := l.panel
	if p == nil || p.focus >= len(p.rows) {
		l.mu.Unlock()
		return
	}
	row := &p.rows[p.focus]
	if row.Control == nil {
		l.mu.Unlock()
		return
	}
	c := row.Control
	var change backend.Change
	switch c.Kind {
	case backend.ControlRange:
		step := c.Step
		if step <= 0 {
			step = 1
		}
		v := c.Value + float64(delta)*step
		if v < c.Min {
			v = c.Min
		}
		if v > c.Max {
			v = c.Max
		}
		c.Value = v
		change = backend.Change{Item: p.item.ID, Control: c.ID, Value: v}
	case backend.ControlChoice:
		if len(c.Options) == 0 {
			l.mu.Unlock()
			return
		}
		cur := 0
		for i, o := range c.Options {
			if o == c.Text {
				cur = i
				break
			}
		}
		next := (cur + delta) % len(c.Options)
		if next < 0 {
			next += len(c.Options)
		}
		c.Text = c.Options[next]
		change = backend.Change{Item: p.item.ID, Control: c.ID, Text: c.Text}
	}
	if change.Control != "" {
		l.updateControlInPlaceLocked(p.item.ID, change)
	}
	source := p.source
	settle := c.Kind == backend.ControlRange
	l.mu.Unlock()
	if settle {
		l.debouncePanelControl(source, change)
	} else {
		_ = l.ControlSource(source, change)
	}
	l.changed()
}

func (l *Launcher) debouncePanelControl(source string, change backend.Change) {
	l.mu.Lock()
	if l.panelDebounce != nil {
		l.panelDebounce.Stop()
	}
	l.panelDebounce = time.AfterFunc(l.opts.SettleMs, func() {
		_ = l.ControlSource(source, change)
	})
	l.mu.Unlock()
}

// PanelActivate activates the focused row: runs the action row, flips a
// toggle, commits a choice, or enters/commits a text field.
func (l *Launcher) PanelActivate() {
	l.mu.Lock()
	p := l.panel
	if p == nil || p.focus >= len(p.rows) {
		l.mu.Unlock()
		return
	}
	row := &p.rows[p.focus]
	if row.Action != nil {
		a := *row.Action
		item := p.item
		source := p.source
		l.mu.Unlock()
		l.performAction(a, item, source)
		return
	}
	if row.Control == nil {
		l.mu.Unlock()
		return
	}
	c := row.Control
	var change backend.Change
	switch c.Kind {
	case backend.ControlToggle:
		c.Bool = !c.Bool
		change = backend.Change{Item: p.item.ID, Control: c.ID, Bool: c.Bool}
	case backend.ControlChoice:
		change = backend.Change{Item: p.item.ID, Control: c.ID, Text: c.Text}
	case backend.ControlText:
		if p.edit == "" {
			p.edit = c.Text
			l.mu.Unlock()
			l.changed()
			return
		}
		c.Text = p.edit
		change = backend.Change{Item: p.item.ID, Control: c.ID, Text: p.edit}
		p.edit = ""
	}
	if change.Control != "" {
		l.updateControlInPlaceLocked(p.item.ID, change)
	}
	source := p.source
	l.mu.Unlock()
	if change.Control != "" {
		_ = l.ControlSource(source, change)
	}
	l.changed()
}

func (l *Launcher) updateControlInPlaceLocked(itemID backend.ResultID, change backend.Change) {
	if itemID == "" || change.Control == "" {
		return
	}
	if l.panel != nil && l.panel.item.ID == itemID {
		for i := range l.panel.item.Controls {
			if l.panel.item.Controls[i].ID == change.Control {
				applyControlChange(&l.panel.item.Controls[i], change)
			}
		}
	}
	for i := range l.results {
		if l.results[i].Item.ID == itemID {
			for j := range l.results[i].Item.Controls {
				if l.results[i].Item.Controls[j].ID == change.Control {
					applyControlChange(&l.results[i].Item.Controls[j], change)
				}
			}
		}
	}
	for _, lvl := range l.levels {
		for i := range lvl.Matches {
			if lvl.Matches[i].Item.ID == itemID {
				for j := range lvl.Matches[i].Item.Controls {
					if lvl.Matches[i].Item.Controls[j].ID == change.Control {
						applyControlChange(&lvl.Matches[i].Item.Controls[j], change)
					}
				}
			}
		}
		for i := range lvl.Items {
			if lvl.Items[i].ID == itemID {
				for j := range lvl.Items[i].Controls {
					if lvl.Items[i].Controls[j].ID == change.Control {
						applyControlChange(&lvl.Items[i].Controls[j], change)
					}
				}
			}
		}
	}
}

func applyControlChange(c *backend.Control, change backend.Change) {
	switch c.Kind {
	case backend.ControlRange, backend.ControlProgress:
		c.Value = change.Value
	case backend.ControlToggle:
		c.Bool = change.Bool
	case backend.ControlChoice, backend.ControlText:
		c.Text = change.Text
	}
}

// PanelType appends to the focused text row's edit buffer.
func (l *Launcher) PanelType(s string) {
	l.mu.Lock()
	p := l.panel
	if p == nil || p.focus >= len(p.rows) || p.rows[p.focus].Control == nil {
		l.mu.Unlock()
		return
	}
	if p.rows[p.focus].Control.Kind != backend.ControlText {
		l.mu.Unlock()
		return
	}
	p.edit += s
	l.mu.Unlock()
	l.changed()
}

// PanelBackspace deletes the last rune from the focused text row's buffer.
func (l *Launcher) PanelBackspace() {
	l.mu.Lock()
	p := l.panel
	if p == nil || p.edit == "" {
		l.mu.Unlock()
		return
	}
	r := []rune(p.edit)
	p.edit = string(r[:len(r)-1])
	l.mu.Unlock()
	l.changed()
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
