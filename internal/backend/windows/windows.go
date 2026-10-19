// Package windows provides the window backend: it reads the compositor's window
// list (niri or sway) and focuses a selected window via a host action.
package windows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// Backend reads window metadata from the compositor. The executables and
// commands to query it are injectable for tests.
type Backend struct {
	// query returns (message, parseString) for the active compositor.
	query func() (string, string, bool)
	// focus returns a command that focuses the window by id.
	focus func(id string) *exec.Cmd
}

// New detects the compositor and returns a backend for it. On, unknown
// compositor the backend is empty but valid.
func New(_ config.Config) (backend.Backend, error) {
	b := &Backend{}

	if sock := os.Getenv("NIRI_SOCKET"); sock != "" && executable("niri") {
		b.query = func() (string, string, bool) {
			out, err := exec.Command("niri", "msg", "-j", "windows").Output()
			if err != nil {
				return "", "", false
			}
			return string(out), "niri", true
		}
		b.focus = func(id string) *exec.Cmd {
			return exec.Command("niri", "msg", "action", "focus-window", id)
		}
		return b, nil
	}
	if sock := os.Getenv("SWAYSOCK"); sock != "" && executable("swaymsg") {
		b.query = func() (string, string, bool) {
			out, err := exec.Command("swaymsg", "-t", "get_tree").Output()
			if err != nil {
				return "", "", false
			}
			return string(out), "sway", true
		}
		b.focus = func(id string) *exec.Cmd {
			return exec.Command("swaymsg", "[con_id="+id+"]", "focus")
		}
		return b, nil
	}
	// No known compositor: an empty backend (documented degradation).
	b.query = func() (string, string, bool) { return "", "", false }
	b.focus = func(string) *exec.Cmd { return nil }
	return b, nil
}

func executable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Info implements backend.Backend.
func (b *Backend) Info() backend.Info {
	return backend.Info{
		ID:         "windows",
		Name:       "Windows",
		Badge:      "WINDOWS",
		Trigger:    "@",
		Searchable: true,
		Trackable:  true,
		StableIDs:  false, // window ids are not stable across runs
	}
}

// Query returns the open windows from the compositor, filtered by text.
func (b *Backend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	raw, kind, ok := b.query()
	if !ok || raw == "" {
		return &sliceStream{}, nil
	}
	var items []backend.Item
	switch kind {
	case "niri":
		items = parseNiri(raw, q.Text)
	case "sway":
		items = parseSway(raw, q.Text)
	}
	return &sliceStream{items: items}, nil
}

func (b *Backend) Control(context.Context, backend.Change) error { return nil }
func (b *Backend) Close() error                                  { return nil }

// niriWindow is the shape of niri msg -j windows entries.
type niriWindow struct {
	ID        uint64 `json:"id"`
	Title     string `json:"title"`
	AppID     string `json:"app-id"`
	IsFocused bool   `json:"is-focused"`
}

func parseNiri(raw, filter string) []backend.Item {
	var wins []niriWindow
	if err := json.Unmarshal([]byte(raw), &wins); err != nil {
		return nil
	}
	items := make([]backend.Item, 0, len(wins))
	for _, w := range wins {
		title := w.Title
		if title == "" {
			title = w.AppID
		}
		if filter != "" && !foldContains(title+" "+w.AppID, filter) {
			continue
		}
		items = append(items, backend.Item{
			ID:       backend.ResultID(fmt.Sprintf("%d", w.ID)),
			Title:    title,
			Subtitle: w.AppID,
			Glyph:    "window",
			Actions:  []backend.Action{{Type: "focus-window", Value: fmt.Sprintf("%d", w.ID)}},
		})
	}
	return items
}

// swayNode is the recursive shape of swaymsg -t get_tree.
type swayNode struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	AppID    string     `json:"app_id"`
	Focused  bool       `json:"focused"`
	Nodes    []swayNode `json:"nodes"`
	Floating []swayNode `json:"floating_nodes"`
}

func parseSway(raw, filter string) []backend.Item {
	var root swayNode
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil
	}
	items := []backend.Item{}
	var walk func(n swayNode)
	walk = func(n swayNode) {
		if n.AppID != "" || n.Name != "" {
			title := n.Name
			if title == "" {
				title = n.AppID
			}
			if filter == "" || foldContains(title, filter) {
				items = append(items, backend.Item{
					ID:       backend.ResultID(fmt.Sprintf("%d", n.ID)),
					Title:    title,
					Subtitle: n.AppID,
					Glyph:    "window",
					Actions:  []backend.Action{{Type: "focus-window", Value: fmt.Sprintf("%d", n.ID)}},
				})
			}
		}
		for _, c := range n.Nodes {
			walk(c)
		}
		for _, c := range n.Floating {
			walk(c)
		}
	}
	walk(root)
	return items
}

func foldContains(hay, needle string) bool {
	if needle == "" {
		return true
	}
	lh := toLower(hay)
	ln := toLower(needle)
	for i := 0; i+len(ln) <= len(lh); i++ {
		if lh[i:i+len(ln)] == ln {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

type sliceStream struct {
	items []backend.Item
	i     int
}

func (s *sliceStream) Next() (backend.Item, bool, error) {
	if s.i >= len(s.items) {
		return backend.Item{}, false, nil
	}
	it := s.items[s.i]
	s.i++
	return it, true, nil
}

func (s *sliceStream) Close() error { return nil }
