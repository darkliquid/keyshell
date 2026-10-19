package capability

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// TestExecuteCopyRoutesToCopyCmd verifies copy actions use the injected copy
// command.
func TestExecuteCopyRoutesToCopyCmd(t *testing.T) {
	var got string
	b := New(config.Default())
	b.copyCmd = func(s string) error { got = s; return nil }

	if err := b.Execute(backend.Action{Type: "copy", Value: "hello"}); err != nil {
		t.Fatalf("Execute = err %v", err)
	}
	if got != "hello" {
		t.Errorf("copy got %q, want hello", got)
	}
}

// TestUnknownActionErrors ensures unsupported types surface an error.
func TestUnknownActionErrors(t *testing.T) {
	b := New(config.Default())
	if err := b.Execute(backend.Action{Type: "nope", Value: "x"}); err == nil {
		t.Fatal("Execute(unknown) = nil error, want error")
	}
}

// TestResolveTerminalPicksCandidate verifies the terminal resolution order.
func TestResolveTerminalPicksCandidate(t *testing.T) {
	cfg := config.Default()
	cfg.Drinks.Terminal = []string{"definitely-not-a-real-terminal-xyz", "sh"}
	got := resolveTerminal(cfg)
	if got == "" {
		t.Fatal("resolveTerminal = empty, want sh (existing candidate)")
	}
}
