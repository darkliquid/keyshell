package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/capability"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/display"
	"github.com/darkliquid/keyshell/internal/input"
	"github.com/darkliquid/keyshell/internal/ipc"
	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/render"
	"github.com/darkliquid/keyshell/internal/telemetry"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gogpu"
)

// runHost runs the graphical overlay host: configuration, theme, in-process
// backends, plugin subprocesses, the launcher, the capability broker, the
// renderer and the IPC socket. It blocks until the app quits.
func runHost(args []string) int {
	name := "run"
	if len(args) > 0 && args[0] != "" {
		name = args[0]
	}
	var hf hostFlags
	fs := newFlagSet(name)
	addHostFlags(fs, &hf)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "keyshell %s: unexpected argument %q\n", name, fs.Arg(0))
		return 2
	}

	ctx, shutdown := setup(hf)
	defer shutdown()

	cfg, err := loadConfig(hf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}
	th, err := theme.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "theme:", err)
		return 1
	}
	defer th.Close()

	// gogpu gates frames on the compositor's callback and drops a redraw that
	// arrives while one is in flight. A close is exactly that case: the band
	// stays on screen because nothing retries once the callback never comes
	// for a surface the compositor sees as undamaged. Run without the gate.
	if os.Getenv("GOGPU_WAYLAND_FRAME_CALLBACK") == "" {
		_ = os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
	}

	provider := buildProvider(cfg)
	telemetry.PluginCountFunc = provider.(*hybridProvider).ActiveCount
	provider.(*hybridProvider).HotStart(cfg)
	l := newLauncher(cfg, provider)
	l.Start()
	_ = capability.New(cfg)

	screenW, screenH := display.Primary()
	if screenW <= 0 || screenH <= 0 {
		screenW, screenH = 1920, 1080
	}
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("keyshell").
		WithSize(screenW, screenH).
		WithTransparent(true).
		// The overlay animates while visible, so it renders every vsync
		// instead of waiting for a redraw request; the presenter skips all
		// drawing while it is hidden.
		WithContinuousRender(true).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     "keyshell",
			Layer:         gogpu.LayerShellOverlay,
			Anchor:        gogpu.LayerShellAnchorTop | gogpu.LayerShellAnchorBottom | gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorRight,
			Width:         int32(screenW),
			Height:        int32(screenH),
			Keyboard:      gogpu.LayerShellKeyboardOnDemand,
			ExclusiveZone: -1,
		}))

	rdr := render.NewConfigured(cfg)
	watchScene(ctx, hf, cfg, rdr, app)

	// Wire input.
	in := input.New(l, app.EventSource())

	// Serve the IPC socket.
	if ln, err := ipc.Listen(); err == nil {
		defer func() { _ = ln.Close() }()
		go func() { _ = ipc.New(l).WithReload(reloadSceneFunc(cfg, rdr, app)).Serve(ln) }()
	} else {
		slog.Warn("ipc socket unavailable", "err", err)
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

	// A signal quits the loop, so the deferred telemetry shutdown can flush.
	go func() {
		<-ctx.Done()
		app.Quit()
	}()

	if err := render.Present(app, l, th, render.PresentConfig{Width: screenW, Height: screenH, Namespace: "keyshell", Reveal: cfg.Reveal, Renderer: rdr}); err != nil {
		fmt.Fprintln(os.Stderr, "overlay:", err)
		return 1
	}
	return 0
}

// newLauncher builds the launcher for a config, with the searchable set
// covering the in-process backends plus every configured plugin instance.
func newLauncher(cfg config.Config, provider backend.BackendProvider) *launcher.Launcher {
	// Queries run on a worker and publish through channels, so a plugin that
	// shells out never stalls the frame loop.
	return launcher.New(provider, newLauncherOptions(cfg))
}

// newLauncherOptions derives the launcher options from the config.
func newLauncherOptions(cfg config.Config) launcher.Options {
	searchable := []string{"apps", "files", "windows"}
	for _, inst := range cfg.Instances {
		searchable = append(searchable, inst.ID)
	}
	return launcher.Options{
		SettleMs:   settleDuration(cfg.Overlay.SettleMs),
		RankOpts:   rank.Options{Limit: cfg.Overlay.MaxResults},
		Searchable: searchable,
	}
}

func settleDuration(ms int) time.Duration {
	if ms <= 0 {
		return 80 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

// watchScene starts the live reload watcher when a scene file is configured
// and watching is on. Reload failures keep the previous scene.
func watchScene(ctx context.Context, hf hostFlags, cfg config.Config, rdr *render.Renderer, app *gogpu.App) {
	if cfg.UI.Scene == "" || !hf.watch {
		return
	}
	w := render.NewSceneWatcher(cfg.UI.Scene, func() (*ksl.Scene, error) {
		return render.LoadScene(cfg)
	})
	go w.Watch(ctx,
		func(s *ksl.Scene) {
			slog.Info("ksl: scene reloaded", "path", cfg.UI.Scene)
			rdr.SetScene(s)
			app.RequestRedraw()
		},
		render.DefaultOnError)
}

// reloadSceneFunc returns the IPC hook that force-reloads the scene on demand.
func reloadSceneFunc(cfg config.Config, rdr *render.Renderer, app *gogpu.App) func() error {
	return func() error {
		s, err := render.LoadScene(cfg)
		if err != nil {
			return err
		}
		rdr.SetScene(s)
		app.RequestRedraw()
		return nil
	}
}
