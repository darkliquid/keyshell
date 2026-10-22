// Package cli implements keyshell's command surface: the graphical host,
// the headless host, the plugin subprocess modes, the verb client, the
// query client and the terminal TUI. It replaces manual os.Args inspection
// with a command registry and per-command flag sets.
package cli

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

// version is overridden at build time with -ldflags "-X .../cli.version=...".
var version = "dev"

// verbs are the IPC verbs a running host understands. The compositor
// keybinds and scripts use them as commands.
var verbs = []string{
	"toggle", "open", "close", "openMode",
	"volumeUp", "volumeDown", "toggleMute",
	"playPause", "nextTrack", "previousTrack", "reload",
}

func isVerb(name string) bool {
	return slices.Contains(verbs, name)
}

const usage = `keyshell - a Wayland launcher

Usage:
  keyshell [flags]              run the graphical host
  keyshell run [flags]          run the graphical host
  keyshell headless [flags]     run the host without the graphical overlay
  keyshell query [flags] TEXT   query the launcher, or a running host over IPC
  keyshell tui [flags]          drive the launcher from the terminal
  keyshell VERB [args]          control a running host
  keyshell plugin NAME          internal: run a plugin subprocess (host-spawned)
  keyshell plugin-clip-capture  internal: capture clipboard selections
  keyshell version              print the version
  keyshell help                 print this help

Host flags (run, headless, tui):
  --config path      config file (default: $KEYSHELL_CONFIG or XDG config)
  --plugins list     override plugin instances: comma-separated ids, or "none"
  --scene path       KSL scene file to render (overrides ui.scene)
  --watch            live-reload the KSL scene when it changes (default: true)
  --log-level L      debug|info|warn|error (default: info; env KEYSHELL_LOG_LEVEL)
  --log-format F     text|json (default: text; env KEYSHELL_LOG_FORMAT)
  --debug-addr ADDR  serve pprof and runtime vars, e.g. 127.0.0.1:6060
  --otel URL         export traces and metrics over OTLP/HTTP, e.g. http://localhost:4318
  --otel-stdout      dump traces and metrics as JSON to stdout
  --service-name N   telemetry service name (default: keyshell)

Verbs:
  toggle, open, close, openMode MODE, volumeUp, volumeDown, toggleMute,
  playPause, nextTrack, previousTrack, reload

Verb and query flags:
  --socket path      IPC socket override (default: $XDG_RUNTIME_DIR/keyshell.sock)

Query flags:
  --mode ID          query a single backend instead of the merged search
  --limit N          cap results (default: overlay.max_results)
  --json             emit JSON
  --host             require a running host; never fall back to a local query
  --timeout D        one-shot local query budget (default: 3s)
`

// newFlagSet builds a flag set that reports usage errors on stderr.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// Main dispatches on the first non-program argument. With no arguments it
// runs the graphical host, which is how a session shell starts.
func Main(argv []string) int {
	args := []string(nil)
	if len(argv) > 0 {
		args = argv[1:]
	}

	if len(args) == 0 {
		return runHost(nil)
	}

	if code, ok := dispatch(args[0], args[1:]); ok {
		return code
	}

	if isVerb(args[0]) {
		return runVerbClient(args[0], args[1:])
	}
	if strings.HasPrefix(args[0], "-") {
		// Global flags before the command: `keyshell --log-level debug query x`
		// splits into flags for the command that follows. Flags without a
		// command mean the host.
		rest := globalFlagPositionals(args)
		if len(rest) == 0 || !dispatchable(rest[0]) {
			return runHost(args)
		}
		flagArgs := args[:len(args)-len(rest)]
		cmdArgs := append(append([]string{}, flagArgs...), rest[1:]...)
		if code, ok := dispatch(rest[0], cmdArgs); ok {
			return code
		}
		return runHost(args)
	}
	fmt.Fprintf(os.Stderr, "keyshell: unknown command %q\n\n%s", args[0], usage)
	return 2
}

// dispatch routes one command word. The bool reports whether the word was a
// command; an exit code of 2 with false never happens.
func dispatch(cmd string, rest []string) (int, bool) {
	switch cmd {
	case "run":
		return runHost(rest), true
	case "headless":
		return runHeadless(rest), true
	case "query":
		return runQuery(rest), true
	case "tui":
		return runTUI(rest), true
	case "plugin":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "usage: keyshell plugin <name>")
			return 2, true
		}
		return runPlugin(rest[0]), true
	case "plugin-clip-capture":
		return runClipCapture(), true
	case "version":
		fmt.Printf("keyshell %s\n", version)
		return 0, true
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0, true
	}
	return 0, false
}

// dispatchable reports whether name is a command the global-flag form can
// reach. Verb clients take no logging or config, so they are excluded.
func dispatchable(name string) bool {
	switch name {
	case "run", "headless", "query", "tui":
		return true
	}
	return false
}

// globalFlagPositionals parses the leading flag block and returns the
// positional arguments after it.
func globalFlagPositionals(args []string) []string {
	fs := newFlagSet("keyshell")
	var hf hostFlags
	addHostFlags(fs, &hf)
	if err := fs.Parse(args); err != nil {
		return nil
	}
	return fs.Args()
}
