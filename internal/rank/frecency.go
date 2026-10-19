// Package rank stores and loads the frecency usage history.
package rank

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
)

// Entry is one record in the frecency history.
type Entry struct {
	Count    int   `json:"count"`
	LastUsed int64 `json:"lastUsed"`
}

// Frecency is the in-memory usage history, saved atomically.
type Frecency struct {
	mu         sync.Mutex
	path       string
	entries    map[string]*Entry
	pruneAfter int64 // cutoff in ms for lastUsed
}

// NewFrecency builds an empty history with the given store path and max age.
func NewFrecency(path string, maxAgeDays int) *Frecency {
	return &Frecency{
		path:       path,
		entries:    map[string]*Entry{},
		pruneAfter: int64(maxAgeDays) * 86400000,
	}
}

// Load reads the store; a missing or unreadable store is not an error.
func (f *Frecency) Load() {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("frecency store unreadable, starting empty", "err", err)
		}
		f.entries = map[string]*Entry{}
		return
	}
	var parsed map[string]*Entry
	if err := json.Unmarshal(data, &parsed); err != nil || parsed == nil {
		f.entries = map[string]*Entry{}
		return
	}
	f.entries = parsed
	f.prune()
}

// Record bumps count and lastUsed for a source/id pair.
func (f *Frecency) Record(source string, id backend.ResultID) {
	if source == "" || id == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := source + ":" + string(id)
	prev := f.entries[key]
	next := &Entry{Count: 1, LastUsed: nowMs()}
	if prev != nil {
		next.Count = prev.Count + 1
		next.LastUsed = nowMs()
	}
	f.entries[key] = next
}

// Score returns the frecency score for a source/id pair.
func (f *Frecency) Score(source string, id backend.ResultID) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.entries[source+":"+string(id)]
	buckets := []Bucket{
		{MaxAgeMs: 3600000, Weight: 4},
		{MaxAgeMs: 86400000, Weight: 3},
		{MaxAgeMs: 604800000, Weight: 2},
		{MaxAgeMs: 2592000000, Weight: 1},
	}
	return FrecencyScore(entry, nowMs(), buckets, 10)
}

// Save writes the history atomically. It is a no-op when the history is clean.
func (f *Frecency) Save() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(f.entries)
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *Frecency) prune() {
	cutoff := nowMs() - f.pruneAfter
	for k, e := range f.entries {
		if e == nil || e.LastUsed < cutoff {
			delete(f.entries, k)
		}
	}
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}
