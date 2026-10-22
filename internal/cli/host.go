// Host wiring shared by the run, headless and tui commands: flag surface,
// configuration loading with flag overrides, the backend provider and the
// telemetry setup.
package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/debugsrv"
	"github.com/darkliquid/keyshell/internal/ipc"
	"github.com/darkliquid/keyshell/internal/logging"
	"github.com/darkliquid/keyshell/internal/plugin"
	"github.com/darkliquid/keyshell/internal/plugin/sdk"
	"github.com/darkliquid/keyshell/internal/telemetry"
	"github.com/godbus/dbus/v5"
)

// hostFlags is the flag surface shared by every long-lived command.
type hostFlags struct {
	config      string
	plugins     string
	scene       string
	watch       bool
	debugAddr   string
	logLevel    string
	logFormat   string
	otel        string
	otelStdout  bool
	serviceName string
}

func defaultConfigPath() string {
	if p := os.Getenv("KEYSHELL_CONFIG"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg + "/keyshell/config.toml"
	}
	home, _ := os.UserHomeDir()
	return home + "/.config/keyshell/config.toml"
}

func addHostFlags(fs *flag.FlagSet, hf *hostFlags) {
	env := logging.DefaultOptions()
	fs.StringVar(&hf.config, "config", defaultConfigPath(), "config file path")
	fs.StringVar(&hf.plugins, "plugins", "", "override plugin instances: comma-separated ids, or \"none\"")
	fs.StringVar(&hf.scene, "scene", "", "KSL scene file (overrides ui.scene)")
	fs.BoolVar(&hf.watch, "watch", true, "live-reload the KSL scene when it changes")
	fs.StringVar(&hf.debugAddr, "debug-addr", "", "address for the pprof/expvar server, e.g. 127.0.0.1:6060")
	fs.StringVar(&hf.logLevel, "log-level", env.Level, "log level: debug|info|warn|error")
	fs.StringVar(&hf.logFormat, "log-format", env.Format, "log format: text|json")
	fs.StringVar(&hf.otel, "otel", "", "OTLP/HTTP endpoint for traces and metrics, e.g. http://localhost:4318")
	fs.BoolVar(&hf.otelStdout, "otel-stdout", false, "export traces and metrics as JSON to stdout")
	fs.StringVar(&hf.serviceName, "service-name", "keyshell", "telemetry service name")
}

// parseHostFlags parses args into a flag set with the shared host flags.
// It returns the command's exit code after printing the error, if any.
func parseHostFlags(name string, args []string, hf *hostFlags) int {
	fs := newFlagSet(name)
	addHostFlags(fs, hf)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "keyshell %s: unexpected argument %q\n", name, fs.Arg(0))
		return 2
	}
	return 0
}

// setup installs logging and telemetry and starts the debug server. The
// returned stop cancels the signal context and flushes telemetry.
func setup(hf hostFlags) (context.Context, func()) {
	logging.Setup(logging.Options{Level: hf.logLevel, Format: hf.logFormat})
	ctx, cancel := signalContext()
	shutdown, err := telemetry.Init(ctx, telemetry.Options{
		ServiceName: hf.serviceName,
		Endpoint:    hf.otel,
		Stdout:      hf.otelStdout,
	})
	if err != nil {
		shutdown, _ = telemetry.Init(ctx, telemetry.Options{})
		// Continue with noop telemetry: a bad exporter must not take the
		// launcher down, but say so.
		fmt.Fprintln(os.Stderr, "telemetry:", err)
	}
	if hf.debugAddr != "" {
		if _, err := debugsrv.Serve(hf.debugAddr, slog.Default()); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	return ctx, func() {
		cancel()
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = shutdown(shutdownCtx)
	}
}

// loadConfig reads the config file and applies flag overrides. A missing
// file is fine; a malformed one is an error.
func loadConfig(hf hostFlags) (config.Config, error) {
	cfg, err := config.Load(hf.config)
	if err != nil {
		return cfg, err
	}
	switch {
	case strings.EqualFold(hf.plugins, "none") || hf.plugins == "-":
		cfg.Instances = nil
	case hf.plugins != "":
		cfg.Instances = nil
		for _, id := range strings.Split(hf.plugins, ",") {
			if id = strings.TrimSpace(id); id != "" {
				cfg.Instances = append(cfg.Instances, config.Instance{ID: id, Mode: "hot"})
			}
		}
	}
	if hf.scene != "" {
		cfg.UI.Scene = hf.scene
	}
	return cfg, nil
}

// buildProvider serves configured plugin instances as subprocesses and falls
// back to in-process backends for the rest. Plugin instances start lazily on
// first use.
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

	// Plugin processes are started once and reused: spawning one per query
	// costs tens of milliseconds of process setup and made every keystroke
	// block the launcher.
	mu    sync.Mutex
	hosts map[string]*plugin.Host
}

func (p *hybridProvider) Get(id string) (backend.Backend, error) {
	launch, ok := p.plugins[id]
	if !ok {
		return p.inproc.Get(id)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if h, ok := p.hosts[id]; ok && h.Alive() {
		return h, nil
	}
	h := plugin.NewHost(launch)
	if err := h.Start(); err != nil {
		return nil, err
	}
	if p.hosts == nil {
		p.hosts = make(map[string]*plugin.Host, len(p.plugins))
	}
	p.hosts[id] = h
	return h, nil
}

// ActiveCount reports how many plugin subprocesses are currently running,
// for the telemetry gauge.
func (p *hybridProvider) ActiveCount() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, h := range p.hosts {
		if h.Alive() {
			n++
		}
	}
	return int64(n)
}

// HotStart best-effort starts configured hot plugin instances before the
// first summon. It never blocks and ignores failures; a failed instance is
// simply re-attempted on first use.
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

// pluginBackend is what a plugin subcommand must provide: identity plus the
// SDK behaviour.
type pluginBackend interface {
	Info() sdk.Info
	sdk.Backend
}

// runPlugin starts a plugin in SDK mode.
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
			defer func() { _ = conn.Close() }()
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
		defer func() { _ = conn.Close() }()
		w := trayplugin.NewWatcher(conn)
		_ = w.Serve()
		b = trayplugin.New(conn, w)
	default:
		fmt.Fprintln(os.Stderr, "unknown plugin:", name)
		return 1
	}
	return servePlugin(b)
}

func runClipCapture() int {
	b := clipplugin.New(clipplugin.StoreConfig{Path: clipplugin.DefaultStorePath()})
	if err := b.Capture(os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		return 1
	}
	return 0
}

func servePlugin(b pluginBackend) int {
	if err := sdk.Serve(b.Info(), b); err != nil {
		fmt.Fprintln(os.Stderr, "plugin:", err)
		return 1
	}
	return 0
}

// isTerminal reports whether the binary is being run interactively.
func isTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// signalContext is done on SIGINT or SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// dialIPC connects to the host's verb socket.
func dialIPC(path string, timeout time.Duration) (net.Conn, error) {
	if path == "" {
		path = ipc.SocketPath()
	}
	conn, err := net.DialTimeout("unix", path, timeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
