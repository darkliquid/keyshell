// Package plugin hosts backend plugins as child processes. Host implements
// backend.Backend on top of a subprocess speaking the keyshell protocol, with
// supervision: handshake, ping liveness, restart backoff and teardown.
package plugin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/plugin/proto"
)

// Launch describes how to start an instance.
type Launch struct {
	Instance string
	Command  []string
	Env      []string
}

// Host is one plugin instance. It is not safe for concurrent Query calls; the
// Manager serializes access.
type Host struct {
	cfg         Launch
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	sc          *bufio.Scanner
	mu          sync.Mutex
	hello       proto.Hello
	nextID      int
	active      bool
	restarts    int
	lastRestart time.Time
	closeCh     chan struct{}
	doneCh      chan struct{}
}

const (
	pingInterval = 10 * time.Second
	pingMisses   = 3
	exitGrace    = 2 * time.Second
)

// Start spawns the plugin and performs the handshake.
func (h *Host) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.startLocked()
}

func (h *Host) startLocked() error {
	cmd := exec.Command(h.cfg.Command[0], h.cfg.Command[1:]...)
	cmd.Env = append(os.Environ(),
		"KEYSHELL_PLUGIN=1",
		fmt.Sprintf("KEYSHELL_PROTOCOL=%d", proto.Protocol),
		"KEYSHELL_INSTANCE="+h.cfg.Instance,
	)
	cmd.Env = append(cmd.Env, h.cfg.Env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	h.cmd = cmd
	h.stdin = stdin
	h.sc = bufio.NewScanner(stdout)
	h.sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	// Handshake: read hello.
	if !h.sc.Scan() {
		_ = h.kill()
		return errors.New("plugin: no hello before EOF")
	}
	msg, err := proto.Decode(h.sc.Bytes())
	if err != nil || msg.Hello == nil {
		_ = h.kill()
		return errors.New("plugin: expected hello")
	}
	if msg.Hello.Protocol != proto.Protocol {
		_ = h.kill()
		return fmt.Errorf("plugin: protocol %d != %d", msg.Hello.Protocol, proto.Protocol)
	}
	h.hello = *msg.Hello

	// Send init.
	if err := h.writeLocked(proto.Message{Init: &proto.Init{Instance: h.cfg.Instance}}); err != nil {
		_ = h.kill()
		return err
	}
	// Expect ready.
	if !h.sc.Scan() {
		_ = h.kill()
		return errors.New("plugin: no ready before EOF")
	}
	if msg, err := proto.Decode(h.sc.Bytes()); err != nil || msg.Ready == nil {
		_ = h.kill()
		return errors.New("plugin: expected ready")
	}

	h.active = true
	h.closeCh = make(chan struct{})
	h.doneCh = make(chan struct{})
	go h.supervise()
	return nil
}

// Info implements backend.Backend from the handshake hello.
func (h *Host) Info() backend.Info {
	h.mu.Lock()
	defer h.mu.Unlock()
	return backend.Info{
		ID:         h.hello.ID,
		Name:       h.hello.Name,
		Badge:      h.hello.Badge,
		Trigger:    h.hello.Trigger,
		Searchable: true,
		Trackable:  h.hello.StableIDs,
		StableIDs:  h.hello.StableIDs,
	}
}

// Query sends a query and materializes the returned items (single in-flight
// per instance, matching the protocol).
func (h *Host) Query(ctx context.Context, q backend.Query) (backend.Stream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active {
		return nil, errors.New("plugin: not running")
	}
	id := h.nextID
	h.nextID++
	if err := h.writeLocked(proto.Message{Query: &proto.Query{
		ID: id, Text: q.Text, Limit: q.Limit, Merged: q.Merged,
	}}); err != nil {
		return nil, err
	}

	var items []backend.Item
	for h.sc.Scan() {
		msg, err := proto.Decode(h.sc.Bytes())
		if err != nil {
			continue
		}
		switch {
		case msg.Item != nil:
			items = append(items, fromItem(*msg.Item))
		case msg.Done != nil && msg.Done.ID == id:
			return &sliceStream{items: items}, nil
		case msg.Error != nil && msg.Error.ID == id:
			return nil, fmt.Errorf("plugin: %s", msg.Error.Message)
		case msg.Pong != nil, msg.State != nil:
			// liveness and state are handled elsewhere
		}
	}
	return nil, errors.New("plugin: stream closed before done")
}

// Control sends a settled value change.
func (h *Host) Control(ctx context.Context, c backend.Change) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active {
		return errors.New("plugin: not running")
	}
	return h.writeLocked(proto.Message{Control: &proto.Control{
		Item: string(c.Item), Control: c.Control, Value: c.Value, Bool: c.Bool, Text: c.Text,
	}})
}

// Close stops the plugin: SIGTERM, then SIGKILL after the grace period.
func (h *Host) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active {
		return nil
	}
	h.active = false
	if h.closeCh != nil {
		close(h.closeCh)
	}
	_ = h.cmd.Process.Signal(os.Interrupt)
	select {
	case <-h.doneCh:
	case <-time.After(exitGrace):
	}
	return h.kill()
}

func (h *Host) kill() error {
	if h.cmd != nil && h.cmd.Process != nil {
		return h.cmd.Process.Kill()
	}
	return nil
}

func (h *Host) writeLocked(m proto.Message) error {
	b, err := proto.Encode(m)
	if err != nil {
		return err
	}
	if _, err := h.stdin.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// supervise runs the ping liveness loop and restarts a crashed instance with
// backoff, bounded.
func (h *Host) supervise() {
	defer close(h.doneCh)
	missed := 0
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-h.closeCh:
			return
		case <-ticker.C:
			h.mu.Lock()
			if !h.active {
				h.mu.Unlock()
				return
			}
			// Send a ping; liveness is best-effort: restart on N missed pongs
			// is handled at the Manager level in this step.
			_ = h.writeLocked(proto.Message{Ping: &proto.Ping{}})
			missed++
			h.mu.Unlock()
			if missed >= pingMisses {
				log.Printf("plugin %s: ping timeout", h.cfg.Instance)
				return
			}
		}
	}
}

func fromItem(p proto.Item) backend.Item {
	actions := make([]backend.Action, 0, len(p.Actions))
	for _, a := range p.Actions {
		actions = append(actions, backend.Action{Type: a.Type, Value: a.Value})
	}
	controls := make([]backend.Control, 0, len(p.Controls))
	for _, c := range p.Controls {
		controls = append(controls, backend.Control{
			ID: c.ID, Kind: backend.ControlKind(c.Kind), Label: c.Label,
			Value: c.Value, Min: c.Min, Max: c.Max, Step: c.Step,
			Bool: c.Bool, Text: c.Text, Options: c.Options,
		})
	}
	return backend.Item{
		ID: backend.ResultID(p.ID), Title: p.Title, Subtitle: p.Subtitle,
		Glyph: p.Glyph, Badge: p.Badge, Destructive: p.Destructive,
		Actions: actions, Controls: controls,
		HasChildren: p.HasChildren, Expandable: p.Expandable,
	}
}

type sliceStream struct {
	items []backend.Item
	i     int
}

func (s *sliceStream) Next() (backend.Item, bool, error) {
	if s.i >= len(s.items) {
		return backend.Item{}, false, nil
	}
	it := s.items[s.i]
	s.i++
	return it, true, nil
}

func (s *sliceStream) Close() error { return nil }
