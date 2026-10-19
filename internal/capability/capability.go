// Package capability is the host's environment-effect broker. Plugins and
// backends never execute commands themselves in step 1; they declare actions
// and the host performs them here.
package capability

import (
	"fmt"
	"os"
	"os/exec"

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

// New builds a broker with the real environment commands.
func New(cfg config.Config) *Broker {
	return &Broker{
		cfg: cfg,
		copyCmd: func(s string) error {
			return exec.Command("wl-copy", s).Run()
		},
		openURLCmd: func(url string) error {
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
			return runDetached("niri", "msg", "action", "focus-window", id)
		},
	}
}

// Execute performs one host action.
func (b *Broker) Execute(a backend.Action) error {
	switch a.Type {
	case "copy":
		return b.copyCmd(a.Value)
	case "open-url":
		return b.openURLCmd(a.Value)
	case "exec":
		return b.execCmd([]string{a.Value})
	case "exec-terminal":
		return b.terminalCmd([]string{a.Value})
	case "focus-window":
		return b.focusCmd(a.Value)
	case "set-query":
		return nil // handled by the launcher
	default:
		return fmt.Errorf("unknown host action %q", a.Type)
	}
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
