// Package apps provides the applications backend: it enumerates installed
// desktop entries and expands their launch commands, wrapping the freedesktop
// desktop-entry layer behind the host Backend seam.
package apps

import (
	"context"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/go-freedesktop/desktopentry"
)

// Backend lists installed applications. Its items carry a single host-executed
// action that launches the application.
type Backend struct {
	entries []*desktopentry.Entry
}

// New enumerates applications from the standard desktop directories.
func New(_ config.Config) (backend.Backend, error) {
	return newWithScan(desktopentry.Scan)
}

// newWithScan builds a backend whose entry source is injectable, so tests can
// pin the set without fighting the system's XDG directories.
func newWithScan(scan func() []*desktopentry.Entry) (backend.Backend, error) {
	return &Backend{entries: scan()}, nil
}

// Info implements backend.Backend.
func (b *Backend) Info() backend.Info {
	return backend.Info{
		ID:         "apps",
		Name:       "Applications",
		Badge:      "APPS",
		Trigger:    "",
		Searchable: true,
		Trackable:  true,
		StableIDs:  true, // desktop-file ids survive restarts
	}
}

// Query returns all apps whose name/exec/keywords match the query text. There
// is no streaming here: the set is small, so it is materialised.
func (b *Backend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	items := make([]backend.Item, 0, len(b.entries))
	for _, e := range b.entries {
		if e.Type != "Application" {
			continue
		}
		if q.Text != "" && !matches(e, q.Text) {
			continue
		}
		argv, err := e.ExpandExec(nil, "")
		if err != nil {
			continue
		}
		items = append(items, backend.Item{
			ID:       backend.ResultID(e.ID),
			Title:    e.Name,
			Subtitle: e.GenericName,
			Glyph:    e.Icon,
			Badge:    "",
			Actions:  []backend.Action{{Type: "exec", Value: joinPlaceholder(argv)}},
			// No children, no controls for apps in step 1.
		})
	}
	return &sliceStream{items: items}, nil
}

func (b *Backend) Control(context.Context, backend.Change) error { return nil }
func (b *Backend) Close() error                                  { return nil }

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

// matches does a simple substring match across the fields a user would type.
func matches(e *desktopentry.Entry, text string) bool {
	hay := e.Name + " " + e.GenericName + " " + e.Exec
	for _, k := range e.Keywords {
		hay += " " + k
	}
	return containsFold(hay, text)
}

func containsFold(hay, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(hay) < len(needle) {
		return false
	}
	lowerHay := toLower(hay)
	sub := toLower(needle)
	for i := 0; i+len(sub) <= len(lowerHay); i++ {
		if lowerHay[i:i+len(sub)] == sub {
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

// joinPlaceholder joins argv back into a single string for the action value.
// The action model stores one string; the capability broker splits it.
func joinPlaceholder(argv []string) string {
	out := ""
	for i, a := range argv {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
