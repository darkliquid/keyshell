// Package backend defines the seam every result source crosses, whether it is
// linked into the host (an in-process backend) or a separate plugin process.
// The launcher, ranking and UI know only these types.
package backend

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkliquid/keyshell/internal/config"
)

// ResultID identifies a result. When a backend reports StableIDs it guarantees
// the id survives a restart, which is what makes frecency meaningful for it.
type ResultID string

// ControlKind is the semantic shape of an editable value. The host renders a
// widget per kind; the plugin owns the meaning.
type ControlKind string

const (
	ControlRange    ControlKind = "range"
	ControlToggle   ControlKind = "toggle"
	ControlChoice   ControlKind = "choice"
	ControlText     ControlKind = "text"
	ControlProgress ControlKind = "progress"
)

// Tone is the semantic colour of a chip. The theme maps each tone to colours.
type Tone string

const (
	ToneAccent    Tone = "accent"
	ToneNeutral   Tone = "neutral"
	ToneInfo      Tone = "info"
	TonePositive  Tone = "positive"
	ToneWarn      Tone = "warn"
	ToneAlert     Tone = "alert"
	ToneAttention Tone = "attention"
)

// Display steers card composition for one control.
type Display string

const (
	DisplayAuto     Display = ""
	DisplayCard     Display = "card"
	DisplayExpanded Display = "expanded"
)

// PartKind names one display widget a card can use.
type PartKind string

const (
	PartBadge    PartKind = "badge"
	PartStatus   PartKind = "status"
	PartVisual   PartKind = "visual"
	PartTitle    PartKind = "title"
	PartSubtitle PartKind = "subtitle"
	PartDetail   PartKind = "detail"
	PartMeter    PartKind = "meter"
	PartTimeline PartKind = "timeline"
	PartNote     PartKind = "note"
	PartHold     PartKind = "hold"
	PartFooter   PartKind = "footer"
)

// CardPart is one explicit card part. A CardPart list on an item overrides the
// derived body; the header and footer still come from the derive rules.
type CardPart struct {
	Kind    PartKind
	Control string
	Text    string
	Tone    Tone
}

// action is a declarative action. Host-executed types are open-url, copy, exec,
// exec-terminal and set-query; a plugin-domain action carries type "plugin" and
// is forwarded to the backend that produced the item.
type Action struct {
	Type  string
	Value string
}

// Control is an editable value shown on a card.
type Control struct {
	ID      string
	Kind    ControlKind
	Label   string
	Value   float64
	Min     float64
	Max     float64
	Step    float64
	Bool    bool
	Text    string
	Options []string
	Display Display
	Primary bool
}

// PanelRow is one interactive row in an item's control panel. It is either
// the action row (Action non-nil, Kind "action") or a control row (Control
// non-nil, Kind is the control's kind). The launcher hands the renderer an
// ordered panel row list so the scene can render one widget per row.
type PanelRow struct {
	Kind    ControlKind
	Label   string
	Action  *Action
	Control *Control
}

// Item is one selectable result.
type Item struct {
	ID          ResultID
	Title       string
	Subtitle    string
	Glyph       string
	Badge       string
	Destructive bool
	Actions     []Action
	Controls    []Control
	HasChildren bool
	Expandable  bool
	Icon        string
	Image       string
	Preview     string
	Status      string
	StatusTone  Tone
	Detail      string
	Hint        string
	Card        []CardPart
}

// Query asks a backend for items. Scope carries the navigation path for a
// scoped sub-level (an item's children).
type Query struct {
	Text   string
	Limit  int
	Merged bool
	Scope  []string
}

// Change is a value change for one control, already settled by the host.
type Change struct {
	Item    ResultID
	Control string
	Value   float64
	Bool    bool
	Text    string
}

// Info is the static identity of a backend.
type Info struct {
	ID         string
	Name       string
	Badge      string
	Trigger    string
	Searchable bool
	Trackable  bool
	StableIDs  bool
}

// Stream is the result of one query. It yields items until it is exhausted.
type Stream interface {
	Next() (Item, bool, error)
	Close() error
}

// Invoke is a domain action forwarded to the backend that produced an item.
type Invoke struct {
	Item   ResultID
	Action string
	Value  string
}

// Invoker is implemented by backends that accept domain actions (plugin
// actions not executed by the host).
type Invoker interface {
	Invoke(ctx context.Context, i Invoke) error
}

// Backend is a source of results. It also accepts control changes.
type Backend interface {
	Info() Info
	Query(ctx context.Context, q Query) (Stream, error)
	Control(ctx context.Context, c Change) error
	Close() error
}

// BackendProvider resolves a backend by id. In this step the provider is backed
// by in-process factories; later it routes to plugin processes.
type BackendProvider interface {
	Get(id string) (Backend, error)
}

// NewInProc returns a provider that builds backends from a factory. The
// factory is called once per id and memory memoised, mirroring the plugin
// provider that arrives in a later step.
func NewInProc(factory func(id string, cfg config.Config) (Backend, error)) BackendProvider {
	return &inProcProvider{factory: factory, built: map[string]Backend{}}
}

type inProcProvider struct {
	factory func(id string, cfg config.Config) (Backend, error)

	// The query worker and a host-side prewarm call Get from different
	// goroutines, so the memo map is guarded.
	mu    sync.Mutex
	built map[string]Backend
}

func (p *inProcProvider) Get(id string) (Backend, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if b, ok := p.built[id]; ok {
		return b, nil
	}
	b, err := p.factory(id, config.Default())
	if err != nil {
		return nil, fmt.Errorf("build backend %q: %w", id, err)
	}
	p.built[id] = b
	return b, nil
}
