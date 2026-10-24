package plugin

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/plugin/proto"
)

// fakePluginPath resolves the python fake plugin relative to this test file.
func fakePluginPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "fake_plugin.py")
}

func newFakeHost(t *testing.T) *Host {
	t.Helper()
	h := &Host{cfg: Launch{Instance: "fake", Command: []string{"python3", fakePluginPath(t)}}}
	if err := h.Start(); err != nil {
		t.Fatalf("Start = err %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// TestHostHandshake verifies hello is read and the instance reports it.
func TestHostHandshake(t *testing.T) {
	h := newFakeHost(t)
	info := h.Info()
	if info.ID != "fake" {
		t.Errorf("Info().ID = %q, want fake", info.ID)
	}
	if !info.StableIDs {
		t.Error("Info().StableIDs = false, want true (fake reports stable ids)")
	}
}

// TestHostQueryStreamsItems verifies a query returns items then terminates.
func TestHostQueryStreamsItems(t *testing.T) {
	h := newFakeHost(t)
	stream, err := h.Query(context.Background(), backend.Query{Text: "hi", Limit: 10})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	var titles []string
	for {
		it, more, err := stream.Next()
		if err != nil {
			t.Fatalf("Next = err %v", err)
		}
		if !more {
			break
		}
		titles = append(titles, it.Title)
	}
	if !reflect.DeepEqual(titles, []string{"hi"}) {
		t.Errorf("titles = %v, want [hi]", titles)
	}
}

// TestHostControlRoundTrips sends a control and expects no error.
func TestHostControlRoundTrips(t *testing.T) {
	h := newFakeHost(t)
	err := h.Control(context.Background(), backend.Change{
		Item: "x", Control: "volume", Value: 0.4,
	})
	if err != nil {
		t.Fatalf("Control = err %v", err)
	}
}

// TestHostCloseIsClean verifies Close returns without hanging.
func TestHostCloseIsClean(t *testing.T) {
	h := newFakeHost(t)
	if err := h.Close(); err != nil {
		t.Errorf("Close = err %v", err)
	}
}

// TestHostQueryAfterCloseFails verifies the host refuses work once stopped.
func TestHostQueryAfterCloseFails(t *testing.T) {
	h := newFakeHost(t)
	_ = h.Close()
	if _, err := h.Query(context.Background(), backend.Query{Text: "x"}); err == nil {
		t.Fatal("Query after Close = nil error, want error")
	}
}

// TestHostSlowHandshake gives the handshake a bounded window.
func TestHostSlowHandshake(t *testing.T) {
	h := &Host{cfg: Launch{Instance: "fake", Command: []string{"python3", fakePluginPath(t)}}}
	done := make(chan error, 1)
	go func() { done <- h.Start() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start = err %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start timed out")
	}
	_ = h.Close()
}

// TestFromItemCarriesCardFields checks the host copies every card field off the
// wire, including the control hints and the override list.
func TestFromItemCarriesCardFields(t *testing.T) {
	got := fromItem(proto.Item{
		ID: "i", Title: "Speakers", Status: "DEFAULT", StatusTone: "positive",
		Icon: "audio", Image: "img", Preview: "preview", Detail: "body", Hint: "ENTER",
		Controls: []proto.ControlSpec{{ID: "volume", Kind: "progress", Display: "card", Primary: true}},
		Card:     []proto.CardPartSpec{{Kind: "meter", Control: "volume", Text: "VOL", Tone: "info"}},
	})
	if got.Status != "DEFAULT" || got.StatusTone != backend.TonePositive {
		t.Errorf("status = %q/%q", got.Status, got.StatusTone)
	}
	if got.Icon != "audio" || got.Image != "img" || got.Preview != "preview" {
		t.Errorf("visual fields = %q/%q/%q", got.Icon, got.Image, got.Preview)
	}
	if got.Detail != "body" || got.Hint != "ENTER" {
		t.Errorf("text fields = %q/%q", got.Detail, got.Hint)
	}
	if len(got.Controls) != 1 || got.Controls[0].Display != backend.DisplayCard || !got.Controls[0].Primary {
		t.Errorf("controls = %+v", got.Controls)
	}
	if len(got.Card) != 1 || got.Card[0].Kind != backend.PartMeter || got.Card[0].Control != "volume" {
		t.Errorf("card = %+v", got.Card)
	}
}

// TestHostAliveTracksClose checks the provider can tell a live plugin from a
// finished one before deciding to reuse it.
func TestHostAliveTracksClose(t *testing.T) {
	h := newFakeHost(t)
	if !h.Alive() {
		t.Fatal("Alive = false on a started host")
	}
	if err := h.Close(); err != nil {
		t.Fatalf("Close = err %v", err)
	}
	if h.Alive() {
		t.Error("Alive = true after Close")
	}
}

// TestHostSearchableDefault checks that Searchable defaults to true when omitted.
func TestHostSearchableDefault(t *testing.T) {
	h := newFakeHost(t)
	info := h.Info()
	if !info.Searchable {
		t.Errorf("Info().Searchable = false, want true by default")
	}
}

// TestHostSearchableExplicitFalse checks that Searchable is false when hello.searchable is false.
func TestHostSearchableExplicitFalse(t *testing.T) {
	script := `
import json, sys
sys.stdout.write(json.dumps({"hello": {"protocol": 1, "id": "unsearchable", "name": "Unsearchable", "searchable": False}}) + "\n")
sys.stdout.flush()
for line in sys.stdin:
    m = json.loads(line)
    if "init" in m:
        sys.stdout.write(json.dumps({"ready": {}}) + "\n")
        sys.stdout.flush()
`
	h := &Host{cfg: Launch{Instance: "unsearchable", Command: []string{"python3", "-c", script}}}
	if err := h.Start(); err != nil {
		t.Fatalf("Start = err %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	info := h.Info()
	if info.Searchable {
		t.Errorf("Info().Searchable = true, want false")
	}
}

// TestHostLaunchConfigForwarding verifies Launch.Config is forwarded in the init message.
func TestHostLaunchConfigForwarding(t *testing.T) {
	script := `
import json, sys
sys.stdout.write(json.dumps({"hello": {"protocol": 1, "id": "cfg_test", "name": "Config Test"}}) + "\n")
sys.stdout.flush()
for line in sys.stdin:
    m = json.loads(line)
    if "init" in m:
        cfg = m["init"].get("config")
        if cfg and cfg.get("key") == "val":
            sys.stdout.write(json.dumps({"ready": {}}) + "\n")
            sys.stdout.flush()
        else:
            sys.stderr.write(f"bad config: {cfg}\n")
            sys.exit(1)
`
	h := &Host{cfg: Launch{
		Instance: "cfg_test",
		Command:  []string{"python3", "-c", script},
		Config:   json.RawMessage(`{"key":"val"}`),
	}}
	if err := h.Start(); err != nil {
		t.Fatalf("Start = err %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	if !h.Alive() {
		t.Fatal("Alive = false on a started host with config")
	}
}
