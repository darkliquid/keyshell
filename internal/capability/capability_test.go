package capability

import (
	"slices"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// TestDefaultCommandsConfigured verifies default copy and openURL handlers are initialized.
func TestDefaultCommandsConfigured(t *testing.T) {
	cfg := config.Config{}
	b := New(cfg)
	if b.copyCmd == nil {
		t.Fatal("expected non-nil copyCmd")
	}
	if b.openURLCmd == nil {
		t.Fatal("expected non-nil openURLCmd")
	}
}

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

// TestExecuteOpenURLRoutesToOpenURLCmd verifies open-url actions use the injected
// openURL command.
func TestExecuteOpenURLRoutesToOpenURLCmd(t *testing.T) {
	var got string
	b := New(config.Default())
	b.openURLCmd = func(url string) error { got = url; return nil }

	if err := b.Execute(backend.Action{Type: "open-url", Value: "https://example.com"}); err != nil {
		t.Fatalf("Execute = err %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("open-url got %q, want https://example.com", got)
	}
}

// TestUnknownActionErrors ensures unsupported types surface an error.
func TestUnknownActionErrors(t *testing.T) {
	b := New(config.Default())
	if err := b.Execute(backend.Action{Type: "nope", Value: "x"}); err == nil {
		t.Fatal("Execute(unknown) = nil error, want error")
	}
}

// TestExecuteExecSplitsCommandLine verifies an exec value is split into argv
// honouring quotes, instead of the whole value becoming one long binary name.
func TestExecuteExecSplitsCommandLine(t *testing.T) {
	var got []string
	b := New(config.Default())
	b.execCmd = func(argv []string) error { got = argv; return nil }

	if err := b.Execute(backend.Action{Type: "exec", Value: `ghostty --class="a b" -e 'c d'`}); err != nil {
		t.Fatalf("Execute = err %v", err)
	}
	want := []string{"ghostty", "--class=a b", "-e", "c d"}
	if !slices.Equal(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

// TestExecuteExecEmptyErrors: an empty exec value has no binary to run.
func TestExecuteExecEmptyErrors(t *testing.T) {
	b := New(config.Default())
	if err := b.Execute(backend.Action{Type: "exec", Value: "  "}); err == nil {
		t.Fatal("Execute(empty exec) = nil, want error")
	}
}

// TestConfiguredCommandsReplaceBuiltins proves [commands] overrides reach
// the exec: a missing override binary surfaces its name in the error rather
// than the built-in tool's.
func TestConfiguredCommandsReplaceBuiltins(t *testing.T) {
	cfg := config.Default()
	cfg.Commands.Copy = []string{"definitely-missing-xyz-copy"}
	cfg.Commands.OpenURL = []string{"definitely-missing-xyz-open"}
	b := New(cfg)

	if err := b.Execute(backend.Action{Type: "copy", Value: "hi"}); err == nil ||
		!strings.Contains(err.Error(), "definitely-missing-xyz-copy") {
		t.Errorf("copy err = %v, want the override binary name", err)
	}
	if err := b.Execute(backend.Action{Type: "open-url", Value: "https://example.com"}); err == nil ||
		!strings.Contains(err.Error(), "definitely-missing-xyz-open") {
		t.Errorf("open-url err = %v, want the override binary name", err)
	}
}

// TestFocusWindowCommandResolution proves the focus-window argv: a
// configured template wins, the {id} placeholder is expanded, and without a
// template the compositor is detected from the environment.
func TestFocusWindowCommandResolution(t *testing.T) {
	cfg := config.Default()
	cfg.Commands.FocusWindow = []string{"mywm", "focus", "{id}"}
	if got, ok := focusCommand(cfg, "7"); !ok || !slices.Equal(got, []string{"mywm", "focus", "7"}) {
		t.Errorf("focusCommand(override) = %v %v, want mywm focus 7", got, ok)
	}
	cfg.Commands.FocusWindow = []string{"mywm", "focus"}
	if got, ok := focusCommand(cfg, "7"); !ok || !slices.Equal(got, []string{"mywm", "focus", "7"}) {
		t.Errorf("focusCommand(append) = %v %v, want mywm focus 7", got, ok)
	}

	cfg.Commands.FocusWindow = nil
	t.Setenv("NIRI_SOCKET", "/tmp/niri.sock")
	if got, ok := focusCommand(cfg, "3"); !ok || !slices.Equal(got, []string{"niri", "msg", "action", "focus-window", "3"}) {
		t.Errorf("focusCommand(niri) = %v %v", got, ok)
	}
	t.Setenv("NIRI_SOCKET", "")
	t.Setenv("SWAYSOCK", "/tmp/sway.sock")
	if got, ok := focusCommand(cfg, "3"); !ok || !slices.Equal(got, []string{"swaymsg", "[con_id=3]", "focus"}) {
		t.Errorf("focusCommand(sway) = %v %v", got, ok)
	}
}

// TestResolveTerminalPicksCandidate verifies the terminal resolution order.
func TestResolveTerminalPicksCandidate(t *testing.T) {
	cfg := config.Default()
	cfg.Commands.Terminal = []string{"definitely-not-a-real-terminal-xyz", "sh"}
	got := resolveTerminal(cfg)
	if got == "" {
		t.Fatal("resolveTerminal = empty, want sh (existing candidate)")
	}
}
