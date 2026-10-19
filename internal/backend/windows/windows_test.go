package windows

import (
	"context"
	"os/exec"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

const niriJSON = `[
  {"id":1,"title":"ghostty","app-id":"com.mitchellh.ghostty","is-focused":true},
  {"id":2,"title":"Firefox","app-id":"firefox","is-focused":false}
]`

const swayJSON = `{
  "nodes":[
    {"id":10,"name":"ghostty","app_id":"com.mitchellh.ghostty","focused":true,
     "nodes":[],"floating_nodes":[]},
    {"id":11,"name":"Firefox","app_id":"firefox","focused":false,
     "nodes":[],"floating_nodes":[]}
  ],
  "floating_nodes":[]
}`

// TestParseNiri covers the niri window list, including filtering by query.
func TestParseNiri(t *testing.T) {
	items := parseNiri(niriJSON, "")
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Title != "ghostty" || items[0].Actions[0].Type != "focus-window" {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[0].Actions[0].Value != "1" {
		t.Errorf("focus value = %q, want 1", items[0].Actions[0].Value)
	}

	filtered := parseNiri(niriJSON, "fire")
	if len(filtered) != 1 || filtered[0].Title != "Firefox" {
		t.Errorf("filtered = %+v, want only Firefox", filtered)
	}
}

// TestParseSway covers the recursive sway tree, including floating nodes.
func TestParseSway(t *testing.T) {
	items := parseSway(swayJSON, "")
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Actions[0].Value != "10" {
		t.Errorf("first sway focus value = %q, want 10", items[0].Actions[0].Value)
	}
}

// TestEmptyQueryForUnknownCompositor ensures an unsupported compositor yields an
// empty stream rather than an error.
func TestEmptyQueryForUnknownCompositor(t *testing.T) {
	b := &Backend{
		query: func() (string, string, bool) { return "", "", false },
		focus: func(string) *exec.Cmd { return nil },
	}
	stream, err := b.Query(context.Background(), backend.Query{Text: ""})
	if err != nil {
		t.Fatalf("Query = err %v, want nil", err)
	}
	defer stream.Close()
	_, more, err := stream.Next()
	if err != nil || more {
		t.Errorf("Next = more=%v err=%v, want false,nil", more, err)
	}
}
