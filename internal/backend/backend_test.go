package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/darkliquid/keyshell/internal/config"
)

// fakeBackend is a minimal in-process backend used to prove the seam works
// without a real domain behind it.
type fakeBackend struct {
	info    Info
	queries []Query // records q.Text of each Query call
}

func (f *fakeBackend) Info() Info { return f.info }

func (f *fakeBackend) Query(_ context.Context, q Query) (Stream, error) {
	f.queries = append(f.queries, q)
	items := []Item{
		{ID: "one", Title: "One"},
		{ID: "two", Title: "Two"},
	}
	// Return a slice-based stream.
	next := make(chan Item, len(items))
	for _, it := range items {
		next <- it
	}
	close(next)
	return &chanStream{ch: next}, nil
}

func (f *fakeBackend) Control(_ context.Context, _ Change) error { return nil }
func (f *fakeBackend) Close() error                              { return nil }

type chanStream struct {
	ch <-chan Item
}

func (s *chanStream) Next() (Item, bool, error) {
	it, ok := <-s.ch
	return it, ok, nil
}

func (s *chanStream) Close() error { return nil }

// TestBackendSeamIsQueryable proves a Backend behind the seam can be queried
// and its items consumed though the Stream interface.
func TestBackendSeamIsQueryable(t *testing.T) {
	b := &fakeBackend{info: Info{ID: "fake", Name: "Fake", Searchable: true}}
	stream, err := b.Query(context.Background(), Query{Text: "x", Limit: 10, Merged: false})
	if err != nil {
		t.Fatalf("Query = err %v, want nil", err)
	}
	defer stream.Close()

	var got []Item
	for {
		it, more, err := stream.Next()
		if err != nil {
			t.Fatalf("Next = err %v, want nil", err)
		}
		if !more {
			break
		}
		got = append(got, it)
	}
	if len(got) != 2 || got[0].ID != "one" || got[1].ID != "two" {
		t.Errorf("items = %+v, want two items [one two]", got)
	}
	if len(b.queries) != 1 || b.queries[0].Text != "x" {
		t.Errorf("queries = %+v, want one query for %q", b.queries, "x")
	}
}

// TestProviderGetReturnsRegisteredBackend covers the provider contract: a
// provider returns the backend registered under an id, and an error for an
// unknown id.
func TestProviderGetReturnsRegisteredBackend(t *testing.T) {
	prov := NewInProc(func(id string, _ config.Config) (Backend, error) {
		if id == "apps" {
			return &fakeBackend{info: Info{ID: "apps"}}, nil
		}
		return nil, fmt.Errorf("unknown backend %q", id)
	})

	b, err := prov.Get("apps")
	if err != nil {
		t.Fatalf("Get(apps) = err %v, want nil", err)
	}
	if b.Info().ID != "apps" {
		t.Errorf("Get(apps).Info().ID = %q", b.Info().ID)
	}

	if _, err := prov.Get("missing"); err == nil {
		t.Fatal("Get(missing) = nil error, want error")
	}
}
