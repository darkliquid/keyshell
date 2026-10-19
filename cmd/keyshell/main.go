// Command keyshell is the host binary: configuration, theme, in-process
// backends, the launcher, the capability broker, the overlay renderer, and the
// IPC socket. It also exposes the verb CLI so compositor keybinds compose.
package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/backend/apps"
	"github.com/darkliquid/keyshell/internal/backend/calc"
	"github.com/darkliquid/keyshell/internal/backend/files"
	"github.com/darkliquid/keyshell/internal/backend/run"
	"github.com/darkliquid/keyshell/internal/backend/windows"
	"github.com/darkliquid/keyshell/internal/capability"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/input"
	"github.com/darkliquid/keyshell/internal/ipc"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/render"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gogpu"
)

func main() {
	// Verb CLI: `keyshell toggle` / `keyshell openMode audio`.
	if len(os.Args) > 1 && isVerb(os.Args[1]) {
		os.Exit(runVerb(os.Args[1:]))
	}

	cfg, err := config.Load(configPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	th, err := theme.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "theme:", err)
		os.Exit(1)
	}
	defer th.Close()

	provider := buildProvider(cfg)
	l := launcher.New(provider, launcher.Options{
		SettleMs: durationMs(cfg.Overlay.SettleMs),
		RankOpts: rank.Options{Limit: cfg.Overlay.MaxResults},
	})
	_ = capability.New(cfg)

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("keyshell").
		WithSize(1600, 900).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     "keyshell",
			Layer:         gogpu.LayerShellOverlay,
			Anchor:        gogpu.LayerShellAnchorTop | gogpu.LayerShellAnchorBottom | gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorRight,
			Width:         1600,
			Height:        900,
			Keyboard:      gogpu.LayerShellKeyboardOnDemand,
			ExclusiveZone: -1,
		}))

	// Wire input.
	_ = input.New(l, app.EventSource())

	// Serve the IPC socket.
	ln, err := net.Listen("unix", ipc.SocketPath())
	if err == nil {
		defer ln.Close()
		go ipc.New(l).Serve(ln)
	}

	if err := render.Present(app, l, th, render.PresentConfig{Width: 1600, Height: 900, Namespace: "keyshell"}); err != nil {
		fmt.Fprintln(os.Stderr, "overlay:", err)
		os.Exit(1)
	}
}

func buildProvider(cfg config.Config) backend.BackendProvider {
	return backend.NewInProc(func(id string, c config.Config) (backend.Backend, error) {
		switch id {
		case "apps":
			return apps.New(c)
		case "files":
			return files.New(c)
		case "windows":
			return windows.New(c)
		case "calc":
			return calc.New(c)
		case "run":
			return run.New(c)
		}
		return nil, fmt.Errorf("unknown backend %q", id)
	})
}

func configPath() string {
	if p := os.Getenv("KEYSHELL_CONFIG"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg + "/keyshell/config.toml"
	}
	home, _ := os.UserHomeDir()
	return home + "/.config/keyshell/config.toml"
}

func durationMs(ms int) time.Duration {
	if ms <= 0 {
		return 80 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

func isVerb(s string) bool {
	switch s {
	case "toggle", "open", "close", "openMode":
		return true
	}
	return false
}

func runVerb(args []string) int {
	conn, err := net.Dial("unix", ipc.SocketPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "keyshell not running:", err)
		return 1
	}
	defer conn.Close()
	verb := args[0]
	if len(args) > 1 {
		verb += " " + args[1]
	}
	if _, err := conn.Write([]byte(verb + "\n")); err != nil {
		fmt.Fprintln(os.Stderr, "send:", err)
		return 1
	}
	return 0
}
