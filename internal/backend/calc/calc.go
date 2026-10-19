// Package calc is the calculator backend. It evaluates the query as an
// expression and offers copying the result as its action.
package calc

import (
	"context"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/calc"
	"github.com/darkliquid/keyshell/internal/config"
)

// Backend evaluates expressions.
type Backend struct{}

// New builds the calculator backend.
func New(_ config.Config) (backend.Backend, error) {
	return &Backend{}, nil
}

// Info implements backend.Backend. The calculator is reached only by its
// "=" trigger and never joins the merged query, matching the QML original.
func (b *Backend) Info() backend.Info {
	return backend.Info{
		ID:         "calc",
		Name:       "Calculator",
		Badge:      "CALC",
		Trigger:    "=",
		Searchable: false,
		Trackable:  false,
		StableIDs:  false,
	}
}

// Query evaluates the query text. A valid expression produces one item whose
// action copies the result; anything unparsable yields no results.
func (b *Backend) Query(_ context.Context, q backend.Query) (backend.Stream, error) {
	result, ok := calc.Eval(q.Text)
	if !ok {
		return &sliceStream{}, nil
	}
	return &sliceStream{items: []backend.Item{{
		ID:       backend.ResultID("result"),
		Title:    result,
		Subtitle: "Calculator",
		Glyph:    "calc",
		Actions:  []backend.Action{{Type: "copy", Value: result}},
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
