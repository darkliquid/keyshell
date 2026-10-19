package trayplugin

import (
	"context"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// fakeWatcher serves a fixed item list without D-Bus.
type fakeWatcher struct {
	items []string
}

func (f *fakeWatcher) Items() []string { return f.items }

// newFakeBackend builds a tray backend over a nil connection, so titles fall
// back to service names (the query path is what we assert).
func newFakeBackend(items ...string) *Backend {
	return &Backend{watcher: &fakeWatcher{items: items}}
}

func TestQueryListsApplets(t *testing.T) {
	b := newFakeBackend("org.kde.StatusNotifierItem-1-1", "org.kde.StatusNotifierItem-2-1")
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	ids := []string{}
	for {
		it, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		ids = append(ids, it.ID)
	}
	if len(ids) != 2 {
		t.Errorf("ids = %v, want 2", ids)
	}
	if ids[0] != "org.kde.StatusNotifierItem-1-1" {
		t.Errorf("first id = %q", ids[0])
	}
}

func TestQueryFilters(t *testing.T) {
	b := newFakeBackend("org.kde.StatusNotifierItem-audio", "org.kde.StatusNotifierItem-other")
	stream, _ := b.Query(context.Background(), sdk.Query{Text: "audio"})
	defer stream.Close()
	got := 0
	for {
		_, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		got++
	}
	if got != 1 {
		t.Errorf("filtered = %d, want 1", got)
	}
}

func TestContainsFold(t *testing.T) {
	if !containsFold("My Applet", "APPLET") {
		t.Error("containsFold should match case-insensitively")
	}
	if containsFold("My Applet", "nope") {
		t.Error("containsFold matched a non-substring")
	}
}

func TestParseMenuItemID(t *testing.T) {
	svc, id, ok := parseMenuItemID("menu:org.kde.StatusNotifierItem-1-1:42")
	if !ok || svc != "org.kde.StatusNotifierItem-1-1" || id != 42 {
		t.Errorf("parse = (%q,%d,%v)", svc, id, ok)
	}
	if _, _, ok := parseMenuItemID("bogus"); ok {
		t.Error("parse(bogus) should fail")
	}
	if _, _, ok := parseMenuItemID("menu:svc:notanumber"); ok {
		t.Error("parse(bad id) should fail")
	}
}
