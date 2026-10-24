package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/darkliquid/keyshell/internal/capability"
	"github.com/darkliquid/keyshell/internal/ipc"
	"github.com/darkliquid/keyshell/internal/telemetry"
)

// runHeadless runs every host service without the graphical overlay: the
// launcher, the backend and plugin processes, and the IPC socket. It is how
// the launcher is developed and debugged without a Wayland session, and it
// still answers `keyshell toggle` and `keyshell query` clients.
func runHeadless(args []string) int {
	var hf hostFlags
	if code := parseHostFlags("headless", args, &hf); code != 0 {
		return code
	}

	ctx, stop := setup(hf)
	defer stop()

	cfg, err := loadConfig(hf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}

	provider := buildProvider(cfg)
	telemetry.PluginCountFunc = provider.ActiveCount
	provider.HotStart(cfg)
	l := newLauncher(cfg, provider)
	l.Start()
	provider.Warm(newLauncherOptions(cfg, provider.Registry()).Searchable...)
	_ = capability.New(cfg)

	ln, err := ipc.Listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ipc:", err)
		return 1
	}
	defer func() { _ = ln.Close() }()
	go func() { _ = ipc.New(l).Serve(ln) }()

	slog.Info("keyshell headless host running", "socket", ipc.SocketPath(), "config", cfgPathForLog(hf))
	<-ctx.Done()
	return 0
}

func cfgPathForLog(hf hostFlags) string { return hf.config }
