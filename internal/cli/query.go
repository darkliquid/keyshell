package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/ipc"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/logging"
	"github.com/darkliquid/keyshell/internal/rank"
)

// queryResponse mirrors ipc.QueryResponse for CLI clients.
type queryResponse struct {
	Results []ipc.QueryRow `json:"results"`
	Error   string         `json:"error,omitempty"`
}

// runQuery queries the launcher. Against a running host it goes over the
// IPC socket; otherwise it performs a one-shot query in this process, which
// needs no graphical session at all.
func runQuery(args []string) int {
	fs := newFlagSet("query")
	var (
		socket  = fs.String("socket", "", "IPC socket path override")
		mode    = fs.String("mode", "", "query a single backend instead of the merged search")
		limit   = fs.Int("limit", 0, "cap results (default: overlay.max_results)")
		asJSON  = fs.Bool("json", false, "emit JSON")
		host    = fs.Bool("host", false, "require a running host; never fall back to a local query")
		timeout = fs.Duration("timeout", 3*time.Second, "one-shot local query budget")
		config  = fs.String("config", defaultConfigPath(), "config file path (one-shot mode)")
		plugins = fs.String("plugins", "", "override plugin instances (one-shot mode)")
	)
	logLevel := fs.String("log-level", "", "log level: debug|info|warn|error")
	logFormat := fs.String("log-format", "", "log format: text|json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// Go's flag parser stops at the first positional, so `query text --mode x`
	// would silently search for "text --mode x". Say so instead.
	seenText := false
	for _, a := range fs.Args() {
		if strings.HasPrefix(a, "-") && seenText {
			fmt.Fprintln(os.Stderr, "hint: flags must come before the query text")
			break
		}
		if !strings.HasPrefix(a, "-") {
			seenText = true
		}
	}
	if *logLevel != "" || *logFormat != "" {
		logging.Setup(logging.Options{Level: *logLevel, Format: *logFormat})
	}
	text := strings.Join(fs.Args(), " ")

	var resp queryResponse
	if r, err := queryHost(*socket, *mode, text); err == nil {
		resp = r
	} else if *host {
		fmt.Fprintln(os.Stderr, "keyshell not running:", err)
		return 1
	} else {
		slog.Debug("no host on the IPC socket, running a one-shot local query", "err", err)
		r, err := queryOneShot(*mode, text, *limit, *timeout, *config, *plugins)
		if err != nil {
			fmt.Fprintln(os.Stderr, "query:", err)
			return 1
		}
		resp = r
	}

	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "query:", resp.Error)
		return 1
	}
	if *asJSON {
		return printJSON(resp)
	}
	printMatches(resp.Results, text)
	return 0
}

// queryOneShot loads the configuration, starts the provider headlessly and
// runs a single query.
func queryOneShot(mode, text string, limit int, timeout time.Duration, configPath, plugins string) (queryResponse, error) {
	cfg, err := loadConfig(hostFlags{config: configPath, plugins: plugins})
	if err != nil {
		return queryResponse{}, err
	}
	if mode == "" {
		mode = launcher.MergedMode
	}
	provider := buildProvider(cfg)
	provider.HotStart(cfg)
	l := launcher.New(provider, newLauncherOptions(cfg, provider.Registry()))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	matches, err := l.QuerySync(ctx, mode, text, limit)
	if err != nil {
		return queryResponse{}, err
	}
	rows := make([]ipc.QueryRow, 0, len(matches))
	for _, m := range matches {
		rows = append(rows, rowFromMatch(m))
	}
	return queryResponse{Results: rows}, nil
}

func rowFromMatch(m rank.Match) ipc.QueryRow {
	return ipc.QueryRow{
		ID: string(m.Item.ID), Title: m.Item.Title,
		Subtitle: m.Item.Subtitle, Source: m.Source,
		Score: m.Score, Actions: m.Item.Actions,
	}
}

func printJSON(v any) int {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		return 1
	}
	fmt.Println(string(b))
	return 0
}

// decodeQueryResponse parses one JSON query answer from the host.
func decodeQueryResponse(line string) (queryResponse, error) {
	var r queryResponse
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return r, fmt.Errorf("decode %q: %w", line, err)
	}
	return r, nil
}

func printMatches(rows []ipc.QueryRow, text string) {
	if len(rows) == 0 {
		fmt.Printf("no results for %q\n", text)
		return
	}
	for i, r := range rows {
		marker := " "
		if i == 0 {
			marker = ">"
		}
		line := fmt.Sprintf("%s %3d  %-24s %-32s [%s]", marker, i+1, truncate(r.Title, 24), truncate(r.Subtitle, 32), r.Source)
		fmt.Println(line)
		if len(r.Actions) > 0 {
			fmt.Printf("      %s\n", describeActions(r.Actions))
		}
	}
}

func describeActions(actions []backend.Action) string {
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		s := a.Type
		if a.Value != "" {
			s += "=" + a.Value
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
