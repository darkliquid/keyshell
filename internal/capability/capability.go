// Package capability is the host's environment-effect broker. Plugins and
// backends never execute commands themselves in step 1; they declare actions
// and the host performs them here.
package capability

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// Broker executes host actions. It is thin and auditable by design.
type Broker struct {
	cfg         config.Config
	copyCmd     func(string) error
	openURLCmd  func(string) error
	execCmd     func([]string) error
	terminalCmd func([]string) error
	focusCmd    func(id string) error
}

// New builds a broker with the real environment commands. [commands] config
// entries replace the built-in tools; empty entries keep the defaults.
func New(cfg config.Config) *Broker {
	return &Broker{
		cfg: cfg,
		copyCmd: func(s string) error {
			if argv := cfg.Commands.Copy; len(argv) > 0 {
				return runDetached(argv[0], append(argv[1:], s)...)
			}
			return exec.Command("wl-copy", s).Run()
		},
		openURLCmd: func(url string) error {
			if argv := cfg.Commands.OpenURL; len(argv) > 0 {
				return runDetached(argv[0], append(argv[1:], url)...)
			}
			return runDetached("xdg-open", url)
		},
		execCmd: func(argv []string) error {
			return runDetached(argv[0], argv[1:]...)
		},
		terminalCmd: func(argv []string) error {
			term := resolveTerminal(cfg)
			if term == "" {
				return fmt.Errorf("no terminal configured")
			}
			return runDetached(term, append([]string{"-e"}, argv...)...)
		},
		focusCmd: func(id string) error {
			argv, ok := focusCommand(cfg, id)
			if !ok {
				return fmt.Errorf("no window-focus command: unknown compositor, set [commands] focus_window")
			}
			return runDetached(argv[0], argv[1:]...)
		},
	}
}

// focusCommand resolves the argv that focuses a window by id: a configured
// [commands] focus_window template wins (with {id} expanded, or the id
// appended when the template has no placeholder); otherwise the compositor
// is detected from the environment (niri, then sway).
func focusCommand(cfg config.Config, id string) ([]string, bool) {
	if tmpl := cfg.Commands.FocusWindow; len(tmpl) > 0 {
		argv := make([]string, 0, len(tmpl)+1)
		placed := false
		for _, arg := range tmpl {
			if strings.Contains(arg, "{id}") {
				argv = append(argv, strings.ReplaceAll(arg, "{id}", id))
				placed = true
			} else {
				argv = append(argv, arg)
			}
		}
		if !placed {
			argv = append(argv, id)
		}
		return argv, true
	}
	if os.Getenv("NIRI_SOCKET") != "" {
		return []string{"niri", "msg", "action", "focus-window", id}, true
	}
	if os.Getenv("SWAYSOCK") != "" {
		return []string{"swaymsg", "[con_id=" + id + "]", "focus"}, true
	}
	return nil, false
}

// Execute performs one host action.
func (b *Broker) Execute(a backend.Action) error {
	switch a.Type {
	case "copy":
		return b.copyCmd(a.Value)
	case "open-url":
		return b.openURLCmd(a.Value)
	case "exec":
		argv := splitWords(a.Value)
		if len(argv) == 0 {
			return fmt.Errorf("exec action has no command")
		}
		return b.execCmd(argv)
	case "exec-terminal":
		argv := splitWords(a.Value)
		if len(argv) == 0 {
			return fmt.Errorf("exec-terminal action has no command")
		}
		return b.terminalCmd(argv)
	case "focus-window":
		return b.focusCmd(a.Value)
	case "set-query":
		return nil // handled by the launcher
	default:
		return fmt.Errorf("unknown host action %q", a.Type)
	}
}

// splitWords splits a command line into argv, honouring single quotes,
// double quotes and backslash escapes. It is the counterpart of the quoting
// the apps backend applies when it flattens argv into an action value, so an
// argument containing spaces survives the round trip.
func splitWords(s string) []string {
	var (
		words   []string
		cur     strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	flush := func() {
		if inWord || cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quote != 0:
			switch {
			case r == quote:
				quote = 0
			case quote == '"' && r == '\\':
				escaped = true
			default:
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped = true
			inWord = true
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if escaped {
		cur.WriteRune('\\')
	}
	flush()
	return words
}

func runDetached(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // detach: don't block on the child
	return nil
}

// resolveTerminal picks the first configured candidate present on PATH.
func resolveTerminal(cfg config.Config) string {
	for _, t := range cfg.Drinks.Terminal {
		if p, err := exec.LookPath(t); err == nil {
			return p
		}
	}
	return ""
}
