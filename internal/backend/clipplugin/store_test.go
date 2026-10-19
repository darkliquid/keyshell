package clipplugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(StoreConfig{
		Path: filepath.Join(t.TempDir(), "clipboard.json"),
		Max:  5,
		TTL:  24 * time.Hour,
	})
}

func TestAddAndEntries(t *testing.T) {
	s := newTestStore(t)
	s.Add("one")
	s.Add("two")
	entries := s.Entries()
	if len(entries) != 2 || entries[0].Text != "two" {
		t.Errorf("entries = %+v, want [two one]", entries)
	}
}

func TestDedupeConsecutive(t *testing.T) {
	s := newTestStore(t)
	s.Add("same")
	s.Add("same")
	if got := len(s.Entries()); got != 1 {
		t.Errorf("entries = %d, want 1 (consecutive dup collapsed)", got)
	}
}

func TestCap(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		s.Add(string(rune('a' + i)))
	}
	if got := len(s.Entries()); got != 5 {
		t.Errorf("entries = %d, want cap 5", got)
	}
}

func TestDeleteAndClear(t *testing.T) {
	s := newTestStore(t)
	s.Add("a")
	s.Add("b")
	s.Delete(0)
	if got := s.Entries(); len(got) != 1 || got[0].Text != "a" {
		t.Errorf("after delete entries = %+v", got)
	}
	s.Clear()
	if got := len(s.Entries()); got != 0 {
		t.Errorf("after clear entries = %d", got)
	}
}

func TestSaveLoadAtomic(t *testing.T) {
	s := newTestStore(t)
	s.Add("hello")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	// A second store reading the same path sees the entry, proving the write
	// is durable and parseable.
	s2 := NewStore(s.cfg)
	s2.Load()
	if got := s2.Entries(); len(got) != 1 || got[0].Text != "hello" {
		t.Errorf("reloaded entries = %+v", got)
	}
	// The temp file must not linger.
	if _, err := os.Stat(s.cfg.Path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file left behind after Save")
	}
}

func TestPruneByTTL(t *testing.T) {
	s := NewStore(StoreConfig{Max: 5, TTL: time.Millisecond})
	s.Add("old")
	time.Sleep(5 * time.Millisecond)
	s.prune()
	if got := len(s.Entries()); got != 0 {
		t.Errorf("entries after prune = %d, want 0", got)
	}
}
