package run

import (
	"context"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

func TestQueryCreatesRunCommand(t *testing.T) {
	b, err := New(config.Default())
	if err != nil {
		t.Fatalf("New = err %v", err)
	}
	stream, err := b.Query(context.Background(), backend.Query{Text: "htop"})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	it, more, err := stream.Next()
	if err != nil || !more {
		t.Fatalf("Next = more=%v err=%v, want more", more, err)
	}
	if it.Title != "htop" {
		t.Errorf("Title = %q", it.Title)
	}
	if len(it.Actions) != 1 || it.Actions[0].Type != "exec-terminal" {
		t.Errorf("Actions = %+v, want one exec-terminal", it.Actions)
	}
	if it.Actions[0].Value != "htop" {
		t.Errorf("action value = %q", it.Actions[0].Value)
	}
}

func TestQueryEmptyYieldsNothing(t *testing.T) {
	b, _ := New(config.Default())
	stream, _ := b.Query(context.Background(), backend.Query{Text: ""})
	defer stream.Close()
	_, more, err := stream.Next()
	if err != nil || more {
		t.Errorf("Next = more=%v err=%v, want false,nil", more, err)
	}
}

func TestTriggerIsColon(t *testing.T) {
	b, _ := New(config.Default())
	if got := b.Info().Trigger; got != ":" {
		t.Errorf("Trigger = %q, want :", got)
	}
}
