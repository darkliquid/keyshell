package plugin

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
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
