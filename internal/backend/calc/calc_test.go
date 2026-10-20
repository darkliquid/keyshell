package calc

import (
	"context"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// TestQueryEvaluatesExpression checks the happy path.
func TestQueryEvaluatesExpression(t *testing.T) {
	b, err := New(config.Default())
	if err != nil {
		t.Fatalf("New = err %v", err)
	}
	stream, err := b.Query(context.Background(), backend.Query{Text: "2+3*4"})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	it, more, err := stream.Next()
	if err != nil || !more {
		t.Fatalf("Next = more=%v err=%v, want more", more, err)
	}
	if it.Title != "14" {
		t.Errorf("Title = %q, want 14", it.Title)
	}
	if len(it.Actions) != 1 || it.Actions[0].Type != "copy" {
		t.Errorf("Actions = %+v, want one copy action", it.Actions)
	}
	_, more, err = stream.Next()
	if err != nil || more {
		t.Errorf("Next after first = more=%v, want false", more)
	}
}

// TestQueryIgnoresInvalidExpression ensures a non-expression yields nothing.
func TestQueryIgnoresInvalidExpression(t *testing.T) {
	b, _ := New(config.Default())
	stream, _ := b.Query(context.Background(), backend.Query{Text: "2+"})
	defer stream.Close()
	_, more, err := stream.Next()
	if err != nil || more {
		t.Errorf("Next = more=%v err=%v, want false,nil", more, err)
	}
}

// TestInfoNonSearchable pins the calculator's placement out of the merged query.
func TestInfoNonSearchable(t *testing.T) {
	b, _ := New(config.Default())
	info := b.Info()
	if info.Searchable || info.Trackable {
		t.Errorf("Info = %+v, want searchable=false trackable=false", info)
	}
	if info.Trigger != "=" {
		t.Errorf("Trigger = %q, want =", info.Trigger)
	}
}

// TestResultHasNoIcon keeps the calculator a body card.
func TestResultHasNoIcon(t *testing.T) {
	b, err := New(config.Default())
	if err != nil {
		t.Fatalf("New = err %v", err)
	}
	stream, err := b.Query(context.Background(), backend.Query{Text: "1+1"})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()
	it, more, _ := stream.Next()
	if !more {
		t.Fatal("no item")
	}
	if it.Icon != "" || it.Glyph != "" {
		t.Errorf("icon = %q glyph = %q, want both empty", it.Icon, it.Glyph)
	}
}
