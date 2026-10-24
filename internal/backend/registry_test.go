package backend

import (
	"context"
	"errors"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

type dummyPlugin struct {
	info        sdk.BackendInfo
	items       []sdk.Item
	queryErr    error
	lastChange  sdk.Change
	lastInvoke  sdk.Invoke
	controlErr  error
	invokeErr   error
}

func (d *dummyPlugin) Info() sdk.BackendInfo { return d.info }
func (d *dummyPlugin) Query(_ context.Context, _ sdk.Query) (sdk.Stream, error) {
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return sdk.NewStream(d.items...), nil
}
func (d *dummyPlugin) Control(_ context.Context, c sdk.Change) error {
	d.lastChange = c
	return d.controlErr
}
func (d *dummyPlugin) Invoke(_ context.Context, i sdk.Invoke) error {
	d.lastInvoke = i
	return d.invokeErr
}

func TestRegistrySearchableIDsEmptyNonNil(t *testing.T) {
	reg := NewRegistry()
	ids := reg.SearchableIDs()
	if ids == nil {
		t.Fatal("SearchableIDs() returned nil for empty registry, want non-nil slice []string{}")
	}
	if len(ids) != 0 {
		t.Fatalf("SearchableIDs() len = %d, want 0", len(ids))
	}
}

func TestRegistryDynamicModesAndSearchable(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyPlugin{info: sdk.BackendInfo{ID: "apps", Searchable: true}})
	reg.Register(&dummyPlugin{info: sdk.BackendInfo{ID: "calc", Searchable: false, Trigger: "="}})
	reg.Register(&dummyPlugin{info: sdk.BackendInfo{ID: "tray", Searchable: true, Trigger: ","}})

	searchable := reg.SearchableIDs()
	if len(searchable) != 2 || searchable[0] != "apps" || searchable[1] != "tray" {
		t.Errorf("searchable = %v, want [apps tray]", searchable)
	}

	modes := reg.Modes()
	if len(modes) != 4 || modes[0] != "" || modes[1] != "apps" || modes[2] != "tray" || modes[3] != "calc" {
		t.Errorf("modes = %v, want ['' apps tray calc]", modes)
	}
}

func TestRegistryGetAndRegisterBackend(t *testing.T) {
	reg := NewRegistry()
	dp := &dummyPlugin{
		info: sdk.BackendInfo{ID: "dp", Name: "Dummy Plugin", Badge: "DP", Searchable: true, StableIDs: true},
		items: []sdk.Item{
			{
				ID:       "item1",
				Title:    "Title 1",
				Subtitle: "Subtitle 1",
				Glyph:    "glyph1",
				Badge:    "B1",
				Actions:  []sdk.Action{{Type: "exec", Value: "cmd"}},
				Controls: []sdk.Control{{ID: "vol", Kind: "range", Value: 0.5}},
				Card:     []sdk.CardPart{{Kind: "badge", Text: "text", Tone: "accent"}},
			},
		},
	}
	reg.Register(dp)

	b, err := reg.Get("dp")
	if err != nil {
		t.Fatalf("Get dp: %v", err)
	}
	info := b.Info()
	if info.ID != "dp" || info.Name != "Dummy Plugin" || info.Badge != "DP" || !info.Searchable || !info.StableIDs || !info.Trackable {
		t.Errorf("unexpected Info: %+v", info)
	}

	stream, err := b.Query(context.Background(), Query{Text: "test"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer stream.Close()

	it, more, err := stream.Next()
	if err != nil || !more {
		t.Fatalf("Next: err=%v more=%v", err, more)
	}
	if it.ID != "item1" || it.Title != "Title 1" || it.Badge != "B1" {
		t.Errorf("unexpected Item: %+v", it)
	}
	if len(it.Actions) != 1 || it.Actions[0].Type != "exec" {
		t.Errorf("unexpected Actions: %+v", it.Actions)
	}
	if len(it.Controls) != 1 || it.Controls[0].ID != "vol" || it.Controls[0].Kind != ControlRange {
		t.Errorf("unexpected Controls: %+v", it.Controls)
	}
	if len(it.Card) != 1 || it.Card[0].Kind != PartBadge || it.Card[0].Tone != ToneAccent {
		t.Errorf("unexpected Card: %+v", it.Card)
	}

	_, more, _ = stream.Next()
	if more {
		t.Errorf("expected end of stream")
	}

	if err := b.Control(context.Background(), Change{Item: "item1", Control: "vol", Value: 0.8}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if dp.lastChange.Item != "item1" || dp.lastChange.Control != "vol" || dp.lastChange.Value != 0.8 {
		t.Errorf("unexpected lastChange: %+v", dp.lastChange)
	}

	if inv, ok := b.(Invoker); ok {
		if err := inv.Invoke(context.Background(), Invoke{Item: "item1", Action: "play", Value: "track"}); err != nil {
			t.Fatalf("Invoke: %v", err)
		}
		if dp.lastInvoke.Item != "item1" || dp.lastInvoke.Action != "play" || dp.lastInvoke.Value != "track" {
			t.Errorf("unexpected lastInvoke: %+v", dp.lastInvoke)
		}
	} else {
		t.Errorf("backend does not implement Invoker")
	}

	if _, err := reg.Get("missing"); err == nil {
		t.Errorf("expected error for missing backend")
	}

	// Test re-registration doesn't duplicate in order
	reg.RegisterBackend("dp", b)
	modes := reg.Modes()
	if len(modes) != 2 || modes[0] != "" || modes[1] != "dp" {
		t.Errorf("modes after duplicate register = %v, want ['' dp]", modes)
	}
}

func TestInProcAdapterQueryError(t *testing.T) {
	expectedErr := errors.New("query failed")
	dp := &dummyPlugin{
		info:     sdk.BackendInfo{ID: "errplug"},
		queryErr: expectedErr,
	}
	adapter := NewInProcPluginAdapter(dp)
	_, err := adapter.Query(context.Background(), Query{})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}
