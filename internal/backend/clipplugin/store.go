// Package clipplugin is the clipboard plugin: a hot, non-reaped instance that
// keeps a bounded history of copied text. It ports the rules that lived in the
// QML version's clipboard-watch.py: dedupe of consecutive copies, a cap, TTL
// pruning, and an atomic store. The policy is explicit controls only — pause,
// clear, per-entry delete — and deliberately no secret detection.
package clipplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Entry is one clipboard history entry.
type Entry struct {
	Text     string `json:"text"`
	CopiedAt int64  `json:"copiedAt"`
}

// StoreConfig is the store's behaviour knobs.
type StoreConfig struct {
	Path string
	Max  int
	TTL  time.Duration
}

// Store is the in-memory history with an atomic Save.
type Store struct {
	mu      sync.Mutex
	cfg     StoreConfig
	entries []Entry
}

// NewStore builds an empty store.
func NewStore(cfg StoreConfig) *Store {
	if cfg.Max == 0 {
		cfg.Max = 50
	}
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour
	}
	return &Store{cfg: cfg}
}

// Load reads the store from disk; a missing or unreadable store is not an
// error (it starts empty).
func (s *Store) Load() {
	data, err := os.ReadFile(s.cfg.Path)
	if err != nil {
		return
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	s.mu.Lock()
	s.entries = entries
	s.mu.Unlock()
	s.prune()
}

// Add inserts a copy at the front, deduping an identical consecutive entry.
func (s *Store) Add(text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) > 0 && s.entries[0].Text == text {
		s.entries[0].CopiedAt = time.Now().UnixMilli()
		return
	}
	s.entries = append([]Entry{{Text: text, CopiedAt: time.Now().UnixMilli()}}, s.entries...)
	if len(s.entries) > s.cfg.Max {
		s.entries = s.entries[:s.cfg.Max]
	}
}

// Entries returns a copy ordered newest first.
func (s *Store) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Delete removes the entry at index.
func (s *Store) Delete(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.entries) {
		return
	}
	s.entries = append(s.entries[:index], s.entries[index+1:]...)
}

// Clear empties the store.
func (s *Store) Clear() {
	s.mu.Lock()
	s.entries = nil
	s.mu.Unlock()
}

// Save writes the store atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.Path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s.entries)
	if err != nil {
		return err
	}
	tmp := s.cfg.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.cfg.Path)
}

// prune drops entries older than the TTL.
func (s *Store) prune() {
	cutoff := time.Now().Add(-s.cfg.TTL).UnixMilli()
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.entries[:0]
	for _, e := range s.entries {
		if e.CopiedAt >= cutoff {
			kept = append(kept, e)
		}
	}
	s.entries = kept
}

// sortEntries keeps tests deterministic when needed.
func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].CopiedAt > entries[j].CopiedAt
	})
}
