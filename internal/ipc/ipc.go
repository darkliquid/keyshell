// Package ipc serves a line-delimited verb socket so compositor keybinds can
// control the overlay with `keyshell toggle`-style commands.
package ipc

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/launcher"
)

// Handler maps an IPC verb to a launcher call.
type Handler struct {
	l *launcher.Launcher
}

// New returns a handler for the launcher.
func New(l *launcher.Launcher) *Handler {
	return &Handler{l: l}
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
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		verb := strings.TrimSpace(sc.Text())
		h.dispatch(verb)
	}
}

func (h *Handler) dispatch(verb string) {
	switch verb {
	case "toggle":
		h.l.Toggle()
	case "open":
		h.l.Open()
	case "close":
		h.l.Close()
	case "volumeUp":
		h.l.ControlSource("audio", backend.Change{Control: "volume-delta", Value: 0.05})
	case "volumeDown":
		h.l.ControlSource("audio", backend.Change{Control: "volume-delta", Value: -0.05})
	case "toggleMute":
		h.l.ControlSource("audio", backend.Change{Control: "mute", Bool: true})
	case "playPause":
		h.mediaPlayback("play-pause")
	case "nextTrack":
		h.mediaPlayback("next")
	case "previousTrack":
		h.mediaPlayback("previous")
	default:
		if strings.HasPrefix(verb, "openMode ") {
			mode := strings.TrimPrefix(verb, "openMode ")
			h.l.SetMode(mode)
			h.l.Open()
		}
	}
}

func (h *Handler) mediaPlayback(action string) {
	h.l.ControlSource("media", backend.Change{Control: "playback", Text: action})
}
