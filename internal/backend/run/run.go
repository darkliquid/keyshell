// Package run is the run-command backend: the user types a command and the host
// runs it in a terminal, or copies it if no terminal is available.
package run

import (
	"context"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// Backend offers the query itself as a command to run.
type Backend struct{}

// New builds the run-command backend.
func New(_ config.Config) (backend.Backend, error) {
	return &Backend{}, nil
}

// Info implements backend.Backend. It is non-searchable and reached by the ":"
// trigger, exactly like the calculator.
func (b *Backend) Info() backend.Info {
	return backend.Info{
		ID:         "run",
		Name:       "Run",
		Badge:      "RUN",
		Trigger:    ":",
		Searchable: false,
		Trackable:  false,
		StableIDs:  false,
	}
}

// Query turns the query text into a single run command item. A query that is
// empty or just whitespace yields nothing.
func (b *Backend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	if q.Text == "" {
		return &sliceStream{}, nil
	}
	title := q.Text
	return &sliceStream{items: []backend.Item{{
		ID:       backend.ResultID("run-command"),
		Title:    title,
		Subtitle: "Run in terminal",
		Glyph:    "command",
		Actions: []backend.Action{{
			Type:  "exec-terminal",
			Value: title,
		}},
	}}}, nil
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
