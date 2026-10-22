package cli

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/capability"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/rank"
	"golang.org/x/term"
)

// tuiState is one TUI session over an in-process launcher: no graphical UI,
// no Wayland, no IPC required.
type tuiState struct {
	l        *launcher.Launcher
	provider backend.BackendProvider
	broker   *capability.Broker
	modes    []string
	mode     int
	query    string
	sel      int
	matches  []rank.Match
	maxShown int
}

// runTUI drives the launcher from the terminal. Every keystroke queries the
// merged search (or a selected mode) and prints the ranked results; the
// selection and Enter work like the overlay, but entirely headlessly.
func runTUI(args []string) int {
	fs := newFlagSet("tui")
	var hf hostFlags
	mode := fs.String("mode", "", "start in this mode")
	maxShown := fs.Int("rows", 10, "results shown at once")
	remote := fs.Bool("host", false, "drive a running host over IPC instead of an in-process launcher")
	socket := fs.String("socket", "", "IPC socket path override (with --host)")
	addHostFlags(fs, &hf)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := loadConfig(hf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}

	if *remote {
		return tuiRemote(*socket, *maxShown)
	}

	provider := buildProvider(cfg)
	provider.(*hybridProvider).HotStart(cfg)
	l := launcher.New(provider, newLauncherOptions(cfg))
	l.Start()

	st := &tuiState{
		l:        l,
		provider: provider,
		broker:   capability.New(cfg),
		modes:    append([]string{launcher.MergedMode}, newLauncherOptions(cfg).Searchable...),
		maxShown: *maxShown,
	}
	if *mode != "" {
		st.setMode(*mode)
	}

	if !isTerminal() {
		return st.lineMode()
	}
	return st.rawMode()
}

// tuiRemote is a fallback line loop over a running host's IPC socket.
func tuiRemote(socket string, maxShown int) int {
	conn, err := dialIPC(socket, 2*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keyshell not running:", err)
		return 1
	}
	defer func() { _ = conn.Close() }()
	fmt.Println("connected to the running host; type a query, empty line or Ctrl-D quits")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		verb := "query " + text
		if _, err := conn.Write([]byte(verb + "\n")); err != nil {
			fmt.Fprintln(os.Stderr, "send:", err)
			return 1
		}
		resp, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil && resp == "" {
			fmt.Fprintln(os.Stderr, "read:", err)
			return 1
		}
		printResponse(strings.TrimRight(resp, "\n"), maxShown)
	}
	return 0
}

func printResponse(line string, maxShown int) {
	resp, err := decodeQueryResponse(line)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad response:", err)
		return
	}
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "query:", resp.Error)
		return
	}
	if len(resp.Results) > maxShown {
		resp.Results = resp.Results[:maxShown]
	}
	for i, r := range resp.Results {
		marker := " "
		if i == 0 {
			marker = ">"
		}
		fmt.Printf("%s %3d  %s\n", marker, i+1, r.Title)
		if r.Subtitle != "" {
			fmt.Printf("      %s [%s]\n", r.Subtitle, r.Source)
		}
	}
	if len(resp.Results) == 0 {
		fmt.Println("(no results)")
	}
}

// setMode switches the query mode by id.
func (st *tuiState) setMode(id string) {
	for i, m := range st.modes {
		if m == id {
			st.mode = i
			return
		}
	}
	st.modes = append(st.modes, id)
	st.mode = len(st.modes) - 1
}

func (st *tuiState) currentMode() string {
	if st.mode < len(st.modes) {
		return st.modes[st.mode]
	}
	return launcher.MergedMode
}

// search runs the current query synchronously; the TUI is a dev tool, and a
// blocking query is exactly the visible feedback it wants.
func (st *tuiState) search() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	matches, err := st.l.QuerySync(ctx, st.currentMode(), st.query, st.maxShown*4)
	if err != nil {
		slog.Debug("tui query failed", "err", err)
		return
	}
	if len(matches) > st.maxShown*4 {
		matches = matches[:st.maxShown*4]
	}
	st.matches = matches
	st.sel = 0
	_ = time.Since(start)
}

// selected returns the selected match, if any.
func (st *tuiState) selected() (rank.Match, bool) {
	if st.sel < 0 || st.sel >= len(st.matches) {
		return rank.Match{}, false
	}
	return st.matches[st.sel], true
}

// activate performs the selected item's primary action: host-executed
// actions through the capability broker, plugin actions through the backend.
func (st *tuiState) activate() string {
	m, ok := st.selected()
	if !ok {
		return "nothing selected"
	}
	if len(m.Item.Actions) == 0 {
		return fmt.Sprintf("%s has no actions", m.Item.Title)
	}
	a := m.Item.Actions[0]
	switch a.Type {
	case "copy", "open-url", "exec", "exec-terminal", "focus-window", "set-query":
		if err := st.broker.Execute(a); err != nil {
			return fmt.Sprintf("%s %s: %v", a.Type, a.Value, err)
		}
		return fmt.Sprintf("%s %s", a.Type, a.Value)
	default:
		// Plugin-domain action: forward to the backend that produced it.
		b, err := st.provider.Get(m.Source)
		if err != nil {
			return fmt.Sprintf("backend %s: %v", m.Source, err)
		}
		inv, ok := b.(backend.Invoker)
		if !ok {
			return "backend does not accept actions"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := inv.Invoke(ctx, backend.Invoke{Item: m.Item.ID, Action: a.Type, Value: a.Value}); err != nil {
			return fmt.Sprintf("plugin action: %v", err)
		}
		return fmt.Sprintf("plugin action %s=%s", a.Type, a.Value)
	}
}

// lineMode reads one query per line. It is the no-TTY fallback that also
// makes the TUI scriptable.
func (st *tuiState) lineMode() int {
	fmt.Println("keyshell tui (line mode): type a query, one per line; Ctrl-D quits")
	fmt.Printf("commands: :mode <id>, :list\n")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, ":mode "):
			st.setMode(strings.TrimPrefix(line, ":mode "))
			fmt.Println("mode:", st.currentMode())
			continue
		case line == ":list":
			fmt.Println("modes:", strings.Join(st.modes, ", "))
			continue
		}
		st.query = line
		st.search()
		for i, m := range st.matches {
			if i >= st.maxShown {
				break
			}
			fmt.Printf("%d %s | %s [%s]\n", i+1, m.Item.Title, m.Item.Subtitle, m.Source)
		}
		if len(st.matches) == 0 {
			fmt.Println("(no results)")
		}
	}
	return 0
}

// rawMode runs the interactive TUI. It renders a prompt plus the ranked
// results and handles the small key set the overlay uses.
func (st *tuiState) rawMode() int {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "terminal:", err)
		return 1
	}
	defer func() { _ = term.Restore(fd, old) }()

	st.search()
	if err := st.render(0); err != nil {
		return 1
	}

	buf := make([]byte, 8)
	printed := 1
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		quit := false
		move := 0
		switch {
		case buf[0] == 0x03 || buf[0] == 0x04: // Ctrl-C, Ctrl-D
			quit = true
		case buf[0] == 0x0d || buf[0] == 0x0a: // Enter
			if _, err := fmt.Fprintf(os.Stdout, "\r\n  %s\r\n", st.activate()); err != nil {
				quit = true
			}
			printed += 2
		case buf[0] == 0x09: // Tab
			st.mode = (st.mode + 1) % len(st.modes)
			st.search()
		case buf[0] == 0x7f: // Backspace
			if st.query != "" {
				st.query = st.query[:len(st.query)-1]
			}
			st.search()
		case buf[0] == 0x1b && n >= 3 && buf[1] == '[': // arrows
			switch buf[2] {
			case 'A':
				move = -1
			case 'B':
				move = 1
			}
		case buf[0] >= 0x20:
			st.query += string(buf[:n])
			st.search()
		}
		if quit {
			break
		}
		if move != 0 && len(st.matches) > 0 {
			st.sel = (st.sel + move + len(st.matches)) % len(st.matches)
		}
		if err := st.render(printed); err != nil {
			break
		}
		printed = 1 + len(st.matchesShown())
	}
	_, _ = fmt.Fprint(os.Stdout, "\r\n")
	return 0
}

func (st *tuiState) matchesShown() []rank.Match {
	if len(st.matches) > st.maxShown {
		return st.matches[:st.maxShown]
	}
	return st.matches
}

// render repaints the prompt and results. up is how many lines the previous
// frame printed, so the cursor can walk back over them. A write error means
// the terminal went away and the loop should stop.
func (st *tuiState) render(up int) error {
	out := os.Stdout
	if up > 0 {
		if _, err := fmt.Fprintf(out, "\x1b[%dA\r", up); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(out, "\x1b[J"); err != nil {
		return err
	}

	shown := st.matchesShown()
	var b strings.Builder
	fmt.Fprintf(&b, "mode %s\r\n", st.currentMode())
	fmt.Fprintf(&b, "> %s\x1b[K\r\n", st.query)
	for i, m := range shown {
		marker := "  "
		if i == st.sel {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%3d %s", marker, i+1, m.Item.Title)
		if m.Item.Subtitle != "" {
			fmt.Fprintf(&b, " | %s", m.Item.Subtitle)
		}
		if m.Source != "" {
			fmt.Fprintf(&b, " [%s]", m.Source)
		}
		b.WriteString("\x1b[K\r\n")
	}
	if len(shown) == 0 {
		fmt.Fprintf(&b, "  (no results)\r\n")
	}
	fmt.Fprintf(&b, "\x1b[2mTab mode | arrows select | Enter run | Ctrl-C quit\x1b[0m\x1b[K")
	_, err := fmt.Fprint(out, b.String())
	return err
}
