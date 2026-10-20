// Package clipplugin contains the clipboard plugin backend and its capture mode.
package clipplugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// Backend serves the clipboard history over the SDK.
type Backend struct {
	store  *Store
	paused atomic.Bool
}

// DefaultStorePath resolves the clipboard history store location.
func DefaultStorePath() string {
	if p := os.Getenv("KEYSHELL_CLIPBOARD_STORE"); p != "" {
		return p
	}
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, _ := os.UserHomeDir()
		state = home + "/.local/state"
	}
	return state + "/keyshell/clipboard.json"
}

// New builds the clipboard backend, loading the existing store.
func New(cfg StoreConfig) *Backend {
	s := NewStore(cfg)
	s.Load()
	return &Backend{store: s}
}

// Info implements sdk.Backend. The clipboard is hot and never reaped: capture
// must run for the shell's whole lifetime.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:        "clipboard",
		Name:      "Clipboard",
		Badge:     "CLIPBOARD",
		Trigger:   ",cb",
		Controls:  []string{"toggle", "choice"},
		Hot:       true,
		StableIDs: true,
	}
}

// Query returns a header item with the pause toggle and one item per entry.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	entries := b.store.Entries()
	lower := strings.ToLower(q.Text)

	items := []sdk.Item{
		{
			ID: "header", Title: "Clipboard",
			Subtitle: fmt.Sprintf("%d entries", len(entries)),
			Icon:     "edit-copy",
			Controls: []sdk.Control{
				{ID: "pause", Kind: "toggle", Label: "Capture", Bool: !b.paused.Load()},
				{ID: "clear", Kind: "choice", Label: "Clear", Options: []string{"clear"}},
			},
		},
	}
	for i, e := range entries {
		if lower != "" && !strings.Contains(strings.ToLower(e.Text), lower) {
			continue
		}
		items = append(items, sdk.Item{
			ID: fmt.Sprintf("%d", i), Title: firstLine(e.Text),
			Subtitle: ageString(e.CopiedAt),
			Icon:     "edit-copy",
			Actions:  []sdk.Action{{Type: "copy", Value: e.Text}},
		})
	}
	return sdk.NewStream(items...), nil
}

// Control handles pause and clear.
func (b *Backend) Control(_ context.Context, c sdk.Change) error {
	switch c.Control {
	case "pause":
		b.paused.Store(!c.Bool)
		return nil
	case "clear":
		b.store.Clear()
		return b.store.Save()
	}
	return fmt.Errorf("clipboard: no control %q", c.Control)
}

// Capture consumes new selections from a `wl-paste --watch` pipe and appends
// them to the store. Selections are newline-delimited; a single read may carry
// several, so the buffer is split line by line.
func (b *Backend) Capture(r io.Reader) error {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		for {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				break
			}
			line := strings.TrimRight(string(buf[:idx]), "\x00")
			b.append(strings.TrimSpace(line))
			buf = buf[idx+1:]
		}
		if err != nil {
			if len(buf) > 0 {
				b.append(strings.TrimSpace(string(buf)))
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (b *Backend) append(text string) {
	if b.paused.Load() || text == "" {
		return
	}
	b.store.Add(text)
	_ = b.store.Save()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

func ageString(ms int64) string {
	age := time.Since(time.UnixMilli(ms))
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}
