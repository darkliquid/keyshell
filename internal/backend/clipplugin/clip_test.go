package clipplugin

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

func newTestBackend(t *testing.T) *Backend {
	t.Helper()
	return New(StoreConfig{Path: t.TempDir() + "/clipboard.json", Max: 5, TTL: 24 * time.Hour})
}

func TestQueryHeader(t *testing.T) {
	b := newTestBackend(t)
	b.store.Add("hello world")

	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()
	it, more, _ := stream.Next()
	if !more {
		t.Fatal("no header item")
	}
	if it.ID != "header" || it.Title != "Clipboard" {
		t.Errorf("header = %+v", it)
	}
	if len(it.Controls) != 2 {
		t.Errorf("header controls = %d, want 2 (pause + clear)", len(it.Controls))
	}
	// entry item
	entry, more, _ := stream.Next()
	if !more || entry.Title != "hello world" {
		t.Errorf("entry = %+v more=%v", entry, more)
	}
	if len(entry.Actions) != 1 || entry.Actions[0].Type != "copy" {
		t.Errorf("entry actions = %+v", entry.Actions)
	}
	if entry.Icon == "" {
		t.Errorf("entry icon = %q, want a theme icon name", entry.Icon)
	}
}

func TestQueryFilters(t *testing.T) {
	b := newTestBackend(t)
	b.store.Add("rust is fun")
	b.store.Add("go is fast")

	stream, _ := b.Query(context.Background(), sdk.Query{Text: "rust"})
	defer stream.Close()
	got := 0
	for {
		it, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		if it.ID != "header" {
			got++
		}
	}
	if got != 1 {
		t.Errorf("filtered entries = %d, want 1", got)
	}
}

func TestControlPauseAndClear(t *testing.T) {
	b := newTestBackend(t)
	b.store.Add("x")

	// The Capture toggle is false when paused; send Bool:false to pause.
	if err := b.Control(context.Background(), sdk.Change{Control: "pause", Bool: false}); err != nil {
		t.Fatal(err)
	}
	b.append("y")
	if got := len(b.store.Entries()); got != 1 {
		t.Errorf("entries while paused = %d, want 1", got)
	}

	// clear empties and saves
	if err := b.Control(context.Background(), sdk.Change{Control: "clear"}); err != nil {
		t.Fatal(err)
	}
	if got := len(b.store.Entries()); got != 0 {
		t.Errorf("entries after clear = %d, want 0", got)
	}
}

func TestCaptureAppends(t *testing.T) {
	b := newTestBackend(t)
	b.Capture(strings.NewReader("first\nsecond\n"))
	if got := len(b.store.Entries()); got != 2 {
		t.Fatalf("entries after capture = %d, want 2", got)
	}
	if b.store.Entries()[0].Text != "second" {
		t.Errorf("newest = %q, want second", b.store.Entries()[0].Text)
	}
}
