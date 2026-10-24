package backend

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// ProviderRegistry manages all registered providers (in-proc and subprocess).
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Backend
	order     []string
}

// NewRegistry creates a new empty backend registry.
func NewRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Backend),
	}
}

// Register registers an in-proc plugin wrapped in an InProcPluginAdapter.
func (r *ProviderRegistry) Register(p sdk.Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info := p.Info()
	if _, exists := r.providers[info.ID]; !exists {
		r.order = append(r.order, info.ID)
	}
	r.providers[info.ID] = NewInProcPluginAdapter(p)
}

// RegisterBackend registers a Backend implementation directly by ID.
func (r *ProviderRegistry) RegisterBackend(id string, b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[id]; !exists {
		r.order = append(r.order, id)
	}
	r.providers[id] = b
}

// Get resolves a backend by ID, satisfying BackendProvider.
func (r *ProviderRegistry) Get(id string) (Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", id)
	}
	return b, nil
}

// SearchableIDs returns registered IDs where Info().Searchable is true, in registered order.
func (r *ProviderRegistry) SearchableIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []string{}
	for _, id := range r.order {
		if b, ok := r.providers[id]; ok && b.Info().Searchable {
			out = append(out, id)
		}
	}
	return out
}

// Modes returns [""] (MergedMode), followed by searchable IDs, followed by non-searchable IDs.
func (r *ProviderRegistry) Modes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	modes := []string{""} // MergedMode first
	var nonSearchable []string
	for _, id := range r.order {
		if b, ok := r.providers[id]; ok {
			if b.Info().Searchable {
				modes = append(modes, id)
			} else {
				nonSearchable = append(nonSearchable, id)
			}
		}
	}
	return append(modes, nonSearchable...)
}

// InProcPluginAdapter wraps sdk.Plugin as backend.Backend.
type InProcPluginAdapter struct {
	p sdk.Plugin
}

// NewInProcPluginAdapter wraps an sdk.Plugin as a Backend.
func NewInProcPluginAdapter(p sdk.Plugin) Backend {
	return &InProcPluginAdapter{p: p}
}

func (a *InProcPluginAdapter) Info() Info {
	info := a.p.Info()
	return Info{
		ID:         info.ID,
		Name:       info.Name,
		Badge:      info.Badge,
		Trigger:    info.Trigger,
		Searchable: info.Searchable,
		Trackable:  info.StableIDs,
		StableIDs:  info.StableIDs,
	}
}

func (a *InProcPluginAdapter) Query(ctx context.Context, q Query) (Stream, error) {
	s, err := a.p.Query(ctx, sdk.Query{
		Text:   q.Text,
		Limit:  q.Limit,
		Merged: q.Merged,
		Scope:  q.Scope,
	})
	if err != nil {
		return nil, err
	}
	return &sdkStreamAdapter{s: s}, nil
}

func (a *InProcPluginAdapter) Control(ctx context.Context, c Change) error {
	return a.p.Control(ctx, sdk.Change{
		Item:    string(c.Item),
		Control: c.Control,
		Value:   c.Value,
		Bool:    c.Bool,
		Text:    c.Text,
	})
}

func (a *InProcPluginAdapter) Invoke(ctx context.Context, i Invoke) error {
	if inv, ok := a.p.(sdk.Invoker); ok {
		return inv.Invoke(ctx, sdk.Invoke{
			Item:   string(i.Item),
			Action: i.Action,
			Value:  i.Value,
		})
	}
	return nil
}

func (a *InProcPluginAdapter) Close() error {
	return nil
}

type sdkStreamAdapter struct {
	s sdk.Stream
}

func (sa *sdkStreamAdapter) Next() (Item, bool, error) {
	it, ok, err := sa.s.Next()
	if err != nil {
		return Item{}, false, err
	}
	if !ok {
		return Item{}, false, nil
	}
	return itemFromSDK(it), true, nil
}

func (sa *sdkStreamAdapter) Close() error {
	return sa.s.Close()
}

func itemFromSDK(it sdk.Item) Item {
	actions := make([]Action, 0, len(it.Actions))
	for _, a := range it.Actions {
		actions = append(actions, Action{Type: a.Type, Value: a.Value})
	}
	controls := make([]Control, 0, len(it.Controls))
	for _, c := range it.Controls {
		controls = append(controls, Control{
			ID:      c.ID,
			Kind:    ControlKind(c.Kind),
			Label:   c.Label,
			Value:   c.Value,
			Min:     c.Min,
			Max:     c.Max,
			Step:    c.Step,
			Bool:    c.Bool,
			Text:    c.Text,
			Options: c.Options,
			Display: Display(c.Display),
			Primary: c.Primary,
		})
	}
	var cardParts []CardPart
	if len(it.Card) > 0 {
		cardParts = make([]CardPart, 0, len(it.Card))
		for _, cp := range it.Card {
			cardParts = append(cardParts, CardPart{
				Kind:    PartKind(cp.Kind),
				Control: cp.Control,
				Text:    cp.Text,
				Tone:    Tone(cp.Tone),
			})
		}
	}
	return Item{
		ID:          ResultID(it.ID),
		Title:       it.Title,
		Subtitle:    it.Subtitle,
		Glyph:       it.Glyph,
		Badge:       it.Badge,
		Destructive: it.Destructive,
		Actions:     actions,
		Controls:    controls,
		HasChildren: it.HasChildren,
		Expandable:  it.Expandable,
		Icon:        it.Icon,
		Image:       it.Image,
		Preview:     it.Preview,
		Status:      it.Status,
		StatusTone:  Tone(it.StatusTone),
		Detail:      it.Detail,
		Hint:        it.Hint,
		Card:        cardParts,
	}
}
