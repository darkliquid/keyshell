// Command keyshell is the host binary: configuration, theme, in-process
// backends, the launcher, the capability broker, the overlay renderer, and the
// IPC socket. It also exposes the verb CLI so compositor keybinds compose.
package main

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/backend/apps"
	"github.com/darkliquid/keyshell/internal/backend/audioplugin"
	"github.com/darkliquid/keyshell/internal/backend/calc"
	"github.com/darkliquid/keyshell/internal/backend/clipplugin"
	"github.com/darkliquid/keyshell/internal/backend/files"
	"github.com/darkliquid/keyshell/internal/backend/mediaplugin"
	"github.com/darkliquid/keyshell/internal/backend/netplugin"
	"github.com/darkliquid/keyshell/internal/backend/powerplugin"
	"github.com/darkliquid/keyshell/internal/backend/run"
	"github.com/darkliquid/keyshell/internal/backend/trayplugin"
	"github.com/darkliquid/keyshell/internal/backend/windows"
	"github.com/darkliquid/keyshell/internal/capability"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/display"
	"github.com/darkliquid/keyshell/internal/input"
	"github.com/darkliquid/keyshell/internal/ipc"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/plugin"
	"github.com/darkliquid/keyshell/internal/plugin/sdk"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/render"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/godbus/dbus/v5"
	"github.com/gogpu/gogpu"
)

func main() {
	// Plugin mode: `keyshell plugin <name>` when spawned by the host.
	if len(os.Args) > 2 && os.Args[1] == "plugin" {
		os.Exit(runPlugin(os.Args[2]))
	}
	// Clipboard capture mode: `keyshell plugin-clip-capture` reads new
	// selections from the wl-paste --watch pipe.
	if len(os.Args) > 1 && os.Args[1] == "plugin-clip-capture" {
		os.Exit(runClipCapture())
	}
	// Verb CLI: `keyshell toggle` / `keyshell openMode audio`.
	if len(os.Args) > 1 && isVerb(os.Args[1]) {
		os.Exit(runVerb(os.Args[1:]))
	}

	// gogpu gates frames on the compositor's callback and drops a redraw that
	// arrives while one is in flight. A close is exactly that case: the band
	// stays on screen because nothing retries once the callback never comes
	// for a surface the compositor sees as undamaged. Run without the gate.
	if os.Getenv("GOGPU_WAYLAND_FRAME_CALLBACK") == "" {
		_ = os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
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
	provider.(*hybridProvider).HotStart(cfg)
	searchable := []string{"apps", "files", "windows"}
	for _, inst := range cfg.Instances {
		searchable = append(searchable, inst.ID)
	}
	l := launcher.New(provider, launcher.Options{
		SettleMs:   durationMs(cfg.Overlay.SettleMs),
		RankOpts:   rank.Options{Limit: cfg.Overlay.MaxResults},
		Searchable: searchable,
	})
	_ = capability.New(cfg)

	screenW, screenH := display.Primary()
	if screenW <= 0 || screenH <= 0 {
		screenW, screenH = 1920, 1080
	}
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("keyshell").
		WithSize(screenW, screenH).
		WithTransparent(true).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     "keyshell",
			Layer:         gogpu.LayerShellOverlay,
			Anchor:        gogpu.LayerShellAnchorTop | gogpu.LayerShellAnchorBottom | gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorRight,
			Width:         int32(screenW),
			Height:        int32(screenH),
			Keyboard:      gogpu.LayerShellKeyboardOnDemand,
			ExclusiveZone: -1,
		}))

	rdr := render.New()

	// Wire input.
	in := input.New(l, app.EventSource())

	// Serve the IPC socket.
	if ln, err := ipc.Listen(); err == nil {
		defer ln.Close()
		go ipc.New(l).Serve(ln)
	}

	// The layer surface stays mapped between summons, so the compositor never
	// sees it "appear" again. Keyboard focus therefore has to be requested on
	// each show: exclusive while open, none while hidden. Exclusive is what
	// makes a summon take typing immediately; a mapped layer surface cannot be
	// focused by clicking it, so pointer presses re-request it below.
	var keyboardMode atomic.Value
	keyboardMode.Store(gogpu.LayerShellKeyboardNone)
	l.OnChange(func() {
		want := gogpu.LayerShellKeyboardNone
		interactive := false
		if l.IsOpen() {
			want = gogpu.LayerShellKeyboardExclusive
			interactive = true
		}
		if keyboardMode.Load() == want {
			return
		}
		keyboardMode.Store(want)
		_ = app.SetLayerKeyboardInteractivity(want)
		// A hidden fullscreen surface would otherwise keep swallowing every
		// click: empty the input region, and restore it on show.
		_ = app.SetInputRegion(interactive)
	})

	// A click on the scrim dismisses, and a click on the band asks the
	// compositor for keyboard focus again: the surface stays mapped, so a
	// launcher that lost focus to another window has no other way back.
	in.OnPress(func(x, y float64) {
		if !l.IsOpen() {
			return
		}
		if !rdr.BandContains(x, y) {
			l.Close()
			return
		}
		_ = app.SetLayerKeyboardInteractivity(gogpu.LayerShellKeyboardNone)
		_ = app.SetLayerKeyboardInteractivity(gogpu.LayerShellKeyboardExclusive)
	})

	// Run interactively from a terminal: show the overlay immediately so the
	// binary is usable without a keybind. As a session shell (no TTY) it starts
	// hidden and is summoned over IPC.
	if isTerminal() {
		l.Open()
	}

	if err := render.Present(app, l, th, render.PresentConfig{Width: screenW, Height: screenH, Namespace: "keyshell", Reveal: cfg.Reveal, Renderer: rdr}); err != nil {
		fmt.Fprintln(os.Stderr, "overlay:", err)
		os.Exit(1)
	}
}

// buildProvider serves configured plugin instances as subprocesses and falls
// back to in-process backends for the rest. Plugin instances start lazily on
// first use, which is enough for step 2.
func buildProvider(cfg config.Config) backend.BackendProvider {
	inproc := backend.NewInProc(func(id string, c config.Config) (backend.Backend, error) {
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

	plugins := map[string]plugin.Launch{}
	self, err := os.Executable()
	if err != nil {
		self = "keyshell"
	}
	for _, inst := range cfg.Instances {
		cmd := inst.Command
		if len(cmd) == 0 {
			cmd = []string{self, "plugin", inst.ID}
		}
		plugins[inst.ID] = plugin.Launch{Instance: inst.ID, Command: cmd}
	}
	return &hybridProvider{inproc: inproc, plugins: plugins}
}

type hybridProvider struct {
	inproc  backend.BackendProvider
	plugins map[string]plugin.Launch
}

func (p *hybridProvider) Get(id string) (backend.Backend, error) {
	if launch, ok := p.plugins[id]; ok {
		h := plugin.NewHost(launch)
		if err := h.Start(); err != nil {
			return nil, err
		}
		return h, nil
	}
	return p.inproc.Get(id)
}

// HotStart best-effort starts configured hot plugin instances before the first
// summon. It never blocks and ignores failures; a failed instance is simply
// re-attempted on first use.
func (p *hybridProvider) HotStart(cfg config.Config) {
	for _, inst := range cfg.Instances {
		if inst.Mode != "hot" {
			continue
		}
		if _, ok := p.plugins[inst.ID]; !ok {
			continue
		}
		go func(id string) {
			_, _ = p.Get(id)
		}(inst.ID)
	}
}

// runPlugin starts a plugin in SDK mode.
// pluginBackend is what a plugin subcommand must provide: identity plus the
// SDK behaviour.
type pluginBackend interface {
	Info() sdk.Info
	sdk.Backend
}

func runClipCapture() int {
	b := clipplugin.New(clipplugin.StoreConfig{Path: clipplugin.DefaultStorePath()})
	if err := b.Capture(os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		return 1
	}
	return 0
}

func runPlugin(name string) int {
	var b pluginBackend
	switch name {
	case "audio":
		b = audioplugin.New()
	case "media":
		b = mediaplugin.New()
	case "network":
		nb := netplugin.New()
		if conn, err := dbus.ConnectSessionBus(); err == nil {
			defer conn.Close()
			_ = netplugin.NewAgent(nb).Register(conn)
		}
		b = nb
	case "power":
		b = powerplugin.New()
	case "clipboard":
		b = clipplugin.New(clipplugin.StoreConfig{Path: clipplugin.DefaultStorePath()})
	case "tray":
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			fmt.Fprintln(os.Stderr, "tray: session bus:", err)
			return 1
		}
		defer conn.Close()
		w := trayplugin.NewWatcher(conn)
		_ = w.Serve()
		b = trayplugin.New(conn, w)
	default:
		fmt.Fprintln(os.Stderr, "unknown plugin:", name)
		return 1
	}
	return servePlugin(b)
}

func servePlugin(b pluginBackend) int {
	if err := sdk.Serve(b.Info(), b); err != nil {
		fmt.Fprintln(os.Stderr, "plugin:", err)
		return 1
	}
	return 0
}

// isTerminal reports whether the binary is being run interactively, so the
// host can show the overlay immediately instead of waiting for an IPC summon.
func isTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
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
