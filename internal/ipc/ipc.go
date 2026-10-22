// Package ipc serves a line-delimited verb socket so compositor keybinds can
// control the overlay with `keyshell toggle`-style commands, and so CLI
// clients can query the launcher and drive scene reloads without the UI.
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/telemetry"
)

// Handler maps an IPC verb to a launcher call.
type Handler struct {
	l *launcher.Launcher
	// reload re-parses the configured KSL scene; nil disables the verb.
	reload func() error
}

// New returns a handler for the launcher.
func New(l *launcher.Launcher) *Handler {
	return &Handler{l: l}
}

// WithReload registers the scene reload hook exposed as the reload verb.
func (h *Handler) WithReload(fn func() error) *Handler {
	h.reload = fn
	return h
}

// Listen opens the control socket, clearing a stale socket file left by a
// previous instance (a killed host leaves the path behind and bind would fail).
func Listen() (net.Listener, error) {
	path := SocketPath()
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path)
	}
	return net.Listen("unix", path)
}

// SocketPath returns the runtime socket path.
func SocketPath() string {
	rt := os.Getenv("XDG_RUNTIME_DIR")
	if rt == "" {
		rt = "/tmp"
	}
	return filepath.Join(rt, "keyshell.sock")
}

// Serve listens on the socket and dispatches verbs until the listener closes.
func (h *Handler) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go h.handleConn(conn)
	}
}

func (h *Handler) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		verb := strings.TrimSpace(sc.Text())
		if resp := h.dispatch(verb); resp != "" {
			if _, err := conn.Write([]byte(resp + "\n")); err != nil {
				return
			}
		}
	}
}

func (h *Handler) dispatch(verb string) string {
	ctx, span := telemetry.StartSpan(context.Background(), "keyshell.ipc.verb",
		trace.WithAttributes(attribute.String(telemetry.Attrs.Verb, verb)))
	defer span.End()
	telemetry.IPCVerbs().Add(ctx, 1,
		metric.WithAttributes(attribute.String(telemetry.Attrs.Verb, verb)))

	switch {
	case verb == "toggle":
		h.l.Toggle()
	case verb == "open":
		h.l.Open()
	case verb == "close":
		h.l.Close()
	case verb == "volumeUp":
		_ = h.l.ControlSource("audio", backend.Change{Control: "volume-delta", Value: 0.05})
	case verb == "volumeDown":
		_ = h.l.ControlSource("audio", backend.Change{Control: "volume-delta", Value: -0.05})
	case verb == "toggleMute":
		_ = h.l.ControlSource("audio", backend.Change{Control: "mute", Bool: true})
	case verb == "playPause":
		_ = h.mediaPlayback("play-pause")
	case verb == "nextTrack":
		_ = h.mediaPlayback("next")
	case verb == "previousTrack":
		_ = h.mediaPlayback("previous")
	case verb == "reload":
		return h.reloadScene(span)
	case verb == "query" || strings.HasPrefix(verb, "query "):
		return h.queryVerb(ctx, launcher.MergedMode, strings.TrimPrefix(verb, "query"))
	case verb == "queryMode" || strings.HasPrefix(verb, "queryMode "):
		rest := strings.TrimPrefix(verb, "queryMode")
		mode, text, _ := strings.Cut(strings.TrimSpace(rest), " ")
		return h.queryVerb(ctx, mode, text)
	default:
		if strings.HasPrefix(verb, "openMode ") {
			mode := strings.TrimPrefix(verb, "openMode ")
			h.l.SetMode(mode)
			h.l.Open()
			return ""
		}
		slog.Warn("unknown ipc verb", "verb", verb)
		span.SetStatus(codes.Error, "unknown verb")
		return ""
	}
	return ""
}

// queryVerb runs an ad-hoc query and answers with one JSON document.
func (h *Handler) queryVerb(ctx context.Context, mode, text string) string {
	text = strings.TrimSpace(text)
	limit := 0
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	matches, err := h.l.QuerySync(ctx, mode, text, limit)
	if err != nil {
		return `{"error":` + strconv.Quote(err.Error()) + `}`
	}
	rows := make([]QueryRow, 0, len(matches))
	for _, m := range matches {
		rows = append(rows, QueryRow{
			ID: string(m.Item.ID), Title: m.Item.Title,
			Subtitle: m.Item.Subtitle, Source: m.Source,
			Score: m.Score, Actions: m.Item.Actions,
		})
	}
	body, err := json.Marshal(QueryResponse{Results: rows})
	if err != nil {
		return `{"error":"encode results"}`
	}
	return string(body)
}

// QueryRow is one query result for CLI clients.
type QueryRow struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Subtitle string           `json:"subtitle,omitempty"`
	Source   string           `json:"source,omitempty"`
	Score    float64          `json:"score,omitempty"`
	Actions  []backend.Action `json:"actions,omitempty"`
}

// QueryResponse is the JSON answer to a query verb.
type QueryResponse struct {
	Results []QueryRow `json:"results"`
	Error   string     `json:"error,omitempty"`
}

func (h *Handler) reloadScene(span trace.Span) string {
	if h.reload == nil {
		return `{"error":"scene reload unavailable"}`
	}
	if err := h.reload(); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return `{"error":` + strconv.Quote(err.Error()) + `}`
	}
	return `{"reloaded":true}`
}

func (h *Handler) mediaPlayback(action string) error {
	return h.l.ControlSource("media", backend.Change{Control: "playback", Text: action})
}
