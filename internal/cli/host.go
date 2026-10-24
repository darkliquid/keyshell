// Host wiring shared by the run, headless and tui commands: flag surface,
// configuration loading with flag overrides, the backend provider and the
// telemetry setup.
package cli

import (
	"context"
	"encoding/json"
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
	default:
		// Auto-discover plugins from user and system plugin directories
		discovered := scanPluginDirs(defaultPluginDirs()...)
		existing := make(map[string]bool, len(cfg.Instances))
		for _, inst := range cfg.Instances {
			existing[inst.ID] = true
		}
		for _, inst := range discovered {
			if !existing[inst.ID] {
				cfg.Instances = append(cfg.Instances, inst)
			}
		}
		for i := range cfg.Instances {
			if cfg.Instances[i].Searchable == nil && len(cfg.Instances[i].Command) > 0 {
				resolveInstanceSearchable(&cfg.Instances[i])
			}
		}
	}
	if hf.scene != "" {
		cfg.UI.Scene = hf.scene
	}
	return cfg, nil
}

type inprocBackendFactory struct {
	info backend.Info
	make func(config.Config) (backend.Backend, error)
}

var inprocBackends = []inprocBackendFactory{
	{
		info: backend.Info{ID: "apps", Name: "Applications", Badge: "APPS", Searchable: true, Trackable: true, StableIDs: true},
		make: apps.New,
	},
	{
		info: backend.Info{ID: "files", Name: "Files", Badge: "FILES", Searchable: true, Trackable: true, StableIDs: true},
		make: files.New,
	},
	{
		info: backend.Info{ID: "windows", Name: "Windows", Badge: "WIN", Searchable: true, Trackable: true, StableIDs: false},
		make: windows.New,
	},
	{
		info: backend.Info{ID: "calc", Name: "Calculator", Badge: "CALC", Trigger: "=", Searchable: false, Trackable: false, StableIDs: false},
		make: calc.New,
	},
	{
		info: backend.Info{ID: "run", Name: "Run Command", Badge: "RUN", Trigger: "!", Searchable: false, Trackable: false, StableIDs: false},
		make: run.New,
	},
}

type lazyBackend struct {
	info backend.Info
	init func() (backend.Backend, error)
	mu   sync.Mutex
	b    backend.Backend
}

func (l *lazyBackend) Info() backend.Info { return l.info }

func (l *lazyBackend) get() (backend.Backend, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.b != nil {
		return l.b, nil
	}
	b, err := l.init()
	if err != nil {
		return nil, err
	}
	l.b = b
	return b, nil
}

func (l *lazyBackend) Query(ctx context.Context, q backend.Query) (backend.Stream, error) {
	b, err := l.get()
	if err != nil {
		return nil, err
	}
	return b.Query(ctx, q)
}

func (l *lazyBackend) Control(ctx context.Context, c backend.Change) error {
	b, err := l.get()
	if err != nil {
		return err
	}
	return b.Control(ctx, c)
}

func (l *lazyBackend) Invoke(ctx context.Context, i backend.Invoke) error {
	b, err := l.get()
	if err != nil {
		return err
	}
	if inv, ok := b.(backend.Invoker); ok {
		return inv.Invoke(ctx, i)
	}
	return nil
}

func (l *lazyBackend) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.b != nil {
		return l.b.Close()
	}
	return nil
}

type pluginInstanceBackend struct {
	hp   *hybridProvider
	id   string
	info backend.Info
}

func (b *pluginInstanceBackend) Info() backend.Info {
	b.hp.mu.Lock()
	e, ok := b.hp.hosts[b.id]
	b.hp.mu.Unlock()
	if ok && e != nil {
		select {
		case <-e.ready:
			if e.err == nil && e.host != nil && e.host.Alive() {
				info := e.host.Info()
				info.ID = b.id
				info.Searchable = b.info.Searchable
				return info
			}
		default:
		}
	}
	return b.info
}

func (b *pluginInstanceBackend) Query(ctx context.Context, q backend.Query) (backend.Stream, error) {
	h, err := b.hp.getHost(b.id)
	if err != nil {
		return nil, err
	}
	return h.Query(ctx, q)
}

func (b *pluginInstanceBackend) Control(ctx context.Context, c backend.Change) error {
	h, err := b.hp.getHost(b.id)
	if err != nil {
		return err
	}
	return h.Control(ctx, c)
}

func (b *pluginInstanceBackend) Invoke(ctx context.Context, i backend.Invoke) error {
	h, err := b.hp.getHost(b.id)
	if err != nil {
		return err
	}
	return h.Invoke(ctx, i)
}

func (b *pluginInstanceBackend) Close() error {
	return nil
}

// buildProvider serves configured plugin instances as subprocesses and falls
// back to in-process backends for the rest. Plugin instances start lazily on
// first use. It returns the concrete provider so hosts can hot-start plugins
// and report the active count without asserting through the interface.
func buildProvider(cfg config.Config) *hybridProvider {
	reg := backend.NewRegistry()

	// Register in-process backends lazily.
	for _, factory := range inprocBackends {
		f := factory
		reg.RegisterBackend(f.info.ID, &lazyBackend{
			info: f.info,
			init: func() (backend.Backend, error) {
				return f.make(cfg)
			},
		})
	}

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
		var cfgBytes json.RawMessage
		if inst.Config != nil {
			b, err := json.Marshal(inst.Config)
			if err != nil {
				slog.Warn("failed to marshal plugin configuration", "instance", inst.ID, "err", err)
			} else {
				cfgBytes = b
			}
		}
		plugins[inst.ID] = plugin.Launch{
			Instance: inst.ID,
			Command:  cmd,
			Config:   cfgBytes,
		}
	}

	hp := &hybridProvider{
		reg:     reg,
		plugins: plugins,
	}

	for _, inst := range cfg.Instances {
		searchable := true
		if inst.Searchable != nil {
			searchable = *inst.Searchable
		}
		reg.RegisterBackend(inst.ID, &pluginInstanceBackend{
			hp: hp,
			id: inst.ID,
			info: backend.Info{
				ID:         inst.ID,
				Searchable: searchable,
			},
		})
	}

	return hp
}

type hybridProvider struct {
	reg     *backend.ProviderRegistry
	plugins map[string]plugin.Launch

	// Plugin processes are started once and reused: spawning one per query
	// costs tens of milliseconds of process setup and made every keystroke
	// block the launcher. The entry's creator starts the process off the
	// provider mutex, so one plugin's spawn never stalls another's query.
	mu    sync.Mutex
	hosts map[string]*hostEntry
}

// Registry returns the underlying backend provider registry.
func (p *hybridProvider) Registry() *backend.ProviderRegistry {
	return p.reg
}

// hostEntry is one plugin process slot. ready closes when its Start settles;
// err carries a spawn failure. Reads of err must follow <-ready.
type hostEntry struct {
	host  *plugin.Host
	ready chan struct{}
	err   error
}

func (p *hybridProvider) Get(id string) (backend.Backend, error) {
	return p.reg.Get(id)
}

func (p *hybridProvider) getHost(id string) (*plugin.Host, error) {
	launch, ok := p.plugins[id]
	if !ok {
		return nil, fmt.Errorf("unknown plugin %q", id)
	}
	for {
		p.mu.Lock()
		if p.hosts == nil {
			p.hosts = make(map[string]*hostEntry, len(p.plugins))
		}
		e, ok := p.hosts[id]
		if !ok {
			e = &hostEntry{host: plugin.NewHost(launch), ready: make(chan struct{})}
			p.hosts[id] = e
			p.mu.Unlock()
			e.err = e.host.Start()
			close(e.ready)
			if e.err != nil {
				p.mu.Lock()
				if p.hosts[id] == e {
					delete(p.hosts, id)
				}
				p.mu.Unlock()
				return nil, e.err
			}
			return e.host, nil
		}
		p.mu.Unlock()
		<-e.ready
		if e.err == nil && e.host.Alive() {
			return e.host, nil
		}
		// Failed to start, or died since: drop the stale entry unless
		// another goroutine already replaced it, then recreate.
		p.mu.Lock()
		if p.hosts[id] == e {
			delete(p.hosts, id)
		}
		p.mu.Unlock()
	}
}

// ActiveCount reports how many plugin subprocesses are currently running,
// for the telemetry gauge.
func (p *hybridProvider) ActiveCount() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	var n int64
	for _, e := range p.hosts {
		select {
		case <-e.ready:
			if e.err == nil && e.host.Alive() {
				n++
			}
		default:
			// still spawning
		}
	}
	return n
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
			_, _ = p.getHost(id)
		}(inst.ID)
	}
}

// Warm pre-builds the given in-process backends in the background, so the
// first summon does not pay their one-time scan or detection costs. Plugin
// instances are ignored: HotStart covers those.
func (p *hybridProvider) Warm(ids ...string) {
	for _, id := range ids {
		if _, isPlugin := p.plugins[id]; isPlugin {
			continue
		}
		go func(id string) {
			if b, err := p.reg.Get(id); err == nil {
				if lb, ok := b.(*lazyBackend); ok {
					_, _ = lb.get()
				}
			}
		}(id)
	}
}

// pluginBackend is what a plugin subcommand must provide: identity plus the
// SDK behaviour.
type pluginBackend interface {
	Info() sdk.Info
	sdk.Backend
}

var builtinPlugins = map[string]func() (pluginBackend, error){
	"audio": func() (pluginBackend, error) {
		return audioplugin.New(), nil
	},
	"media": func() (pluginBackend, error) {
		return mediaplugin.New(), nil
	},
	"network": func() (pluginBackend, error) {
		nb := netplugin.New()
		if conn, err := dbus.ConnectSessionBus(); err == nil {
			_ = netplugin.NewAgent(nb).Register(conn)
		}
		return nb, nil
	},
	"power": func() (pluginBackend, error) {
		return powerplugin.New(), nil
	},
	"clipboard": func() (pluginBackend, error) {
		return clipplugin.New(clipplugin.StoreConfig{Path: clipplugin.DefaultStorePath()}), nil
	},
	"tray": func() (pluginBackend, error) {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			return nil, fmt.Errorf("tray: session bus: %w", err)
		}
		w := trayplugin.NewWatcher(conn)
		_ = w.Serve()
		return trayplugin.New(conn, w), nil
	},
}

// runPlugin starts a plugin in SDK mode.
func runPlugin(name string) int {
	factory, ok := builtinPlugins[name]
	if !ok {
		fmt.Fprintln(os.Stderr, "unknown plugin:", name)
		return 1
	}
	b, err := factory()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
