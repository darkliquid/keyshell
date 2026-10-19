// Package sdk is what plugin authors import. A plugin is a program that starts
// with sdk.Serve, declares its Info, and implements a Handler. The SDK owns the
// protocol handshake, the read loop and reply encoding; the plugin owns its
// domain.
package sdk

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/darkliquid/keyshell/internal/plugin/proto"
)

// Re-exported backend shapes so plugins depend on the SDK, not the host.
// (Aliases: the wire conversion in serve.go bridges these to proto types.)
type (
	Item    = ItemShape
	Control = ControlSpecShape
	Action  = ActionShape
	Query   = QueryShape
	Change  = ChangeShape
	Info    = InfoShape
	Stream  = StreamShape
	Backend = BackendShape
)

// ItemShape is one result.
type ItemShape struct {
	ID          string
	Title       string
	Subtitle    string
	Glyph       string
	Badge       string
	Destructive bool
	Actions     []Action
	Controls    []Control
	HasChildren bool
	Expandable  bool
}

// ControlSpecShape is an editable value.
type ControlSpecShape struct {
	ID      string
	Kind    string
	Label   string
	Value   float64
	Min     float64
	Max     float64
	Step    float64
	Bool    bool
	Text    string
	Options []string
}

// ActionShape is a declarative action.
type ActionShape struct {
	Type  string
	Value string
}

// QueryShape asks for items.
type QueryShape struct {
	Text   string
	Limit  int
	Merged bool
}

// ChangeShape is a settled value change.
type ChangeShape struct {
	Item    string
	Control string
	Value   float64
	Bool    bool
	Text    string
}

// InfoShape is the plugin's static identity.
type InfoShape struct {
	ID        string
	Name      string
	Badge     string
	Trigger   string
	Controls  []string
	Hot       bool
	StableIDs bool
}

// StreamShape yields items.
type StreamShape interface {
	Next() (Item, bool, error)
	Close() error
}

// BackendShape is the domain logic a plugin implements.
type BackendShape interface {
	Query(ctx context.Context, q Query) (Stream, error)
	Control(ctx context.Context, c Change) error
}

// NewStream builds a slice-backed stream.
func NewStream(items ...Item) Stream {
	return &sliceStream{items: items}
}

type sliceStream struct {
	items []Item
	i     int
}

func (s *sliceStream) Next() (Item, bool, error) {
	if s.i >= len(s.items) {
		return Item{}, false, nil
	}
	it := s.items[s.i]
	s.i++
	return it, true, nil
}

func (s *sliceStream) Close() error { return nil }

// Serve runs the plugin protocol until stdin closes. It returns an error only
// for a fatal handshake failure; domain errors are reported per query.
func Serve(info Info, backend Backend) error {
	if os.Getenv("KEYSHELL_PLUGIN") == "" {
		return errors.New("sdk: not running as a plugin (KEYSHELL_PLUGIN not set)")
	}
	if got := os.Getenv("KEYSHELL_PROTOCOL"); got != fmt.Sprintf("%d", proto.Protocol) {
		return fmt.Errorf("sdk: protocol mismatch: have %s want %d", got, proto.Protocol)
	}

	w := bufio.NewWriter(os.Stdout)
	write := func(m proto.Message) error {
		b, err := proto.Encode(m)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
		return w.Flush()
	}

	// 1. Hello.
	if err := write(proto.Message{Hello: &proto.Hello{
		Protocol:  proto.Protocol,
		ID:        info.ID,
		Name:      info.Name,
		Badge:     info.Badge,
		Trigger:   info.Trigger,
		Controls:  info.Controls,
		Hot:       info.Hot,
		StableIDs: info.StableIDs,
	}}); err != nil {
		return err
	}

	// 2. Wait for init, reply ready.
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), protoLineCap)
	if !sc.Scan() {
		return errors.New("sdk: connection closed before init")
	}
	if msg, err := proto.Decode(sc.Bytes()); err != nil || msg.Init == nil {
		return errors.New("sdk: expected init")
	}
	if err := write(proto.Message{Ready: &proto.Ready{}}); err != nil {
		return err
	}

	// 3. Main loop.
	for sc.Scan() {
		msg, err := proto.Decode(sc.Bytes())
		if err != nil {
			continue // drop malformed lines
		}
		if err := handle(w, msg, backend); err != nil {
			log.Printf("sdk: handler: %v", err)
		}
	}
	return sc.Err()
}

const protoLineCap = 1 << 20 // 1 MiB

func handle(w *bufio.Writer, msg proto.Message, backend Backend) error {
	switch {
	case msg.Ping != nil:
		return writeMsg(w, proto.Message{Pong: &proto.Pong{Ready: true}})
	case msg.Control != nil:
		return backend.Control(context.Background(), fromChange(*msg.Control))
	case msg.Query != nil:
		return serveQuery(w, backend, *msg.Query)
	case msg.Cancel != nil:
		// No cancellation support required by the spec; the host ignores stale
		// items by id, so returning nil is correct.
		return nil
	}
	return nil
}

func writeMsg(w *bufio.Writer, m proto.Message) error {
	b, err := proto.Encode(m)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	return w.Flush()
}

func serveQuery(w *bufio.Writer, backend Backend, q proto.Query) error {
	stream, err := backend.Query(context.Background(), Query{Text: q.Text, Limit: q.Limit, Merged: q.Merged})
	if err != nil {
		return writeErr(w, q.ID, err.Error())
	}
	defer stream.Close()
	for {
		it, more, err := stream.Next()
		if err != nil {
			return writeErr(w, q.ID, err.Error())
		}
		if !more {
			break
		}
		pitem := toItem(it)
		if err := writeMsg(w, proto.Message{Item: &pitem}); err != nil {
			return err
		}
	}
	return writeMsg(w, proto.Message{Done: &proto.Done{ID: q.ID}})
}

func writeErr(w *bufio.Writer, id int, message string) error {
	b, err := proto.Encode(proto.Message{Error: &proto.Err{ID: id, Message: message}})
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	return w.Flush()
}

func toItem(it Item) proto.Item {
	actions := make([]proto.Action, 0, len(it.Actions))
	for _, a := range it.Actions {
		actions = append(actions, proto.Action{Type: a.Type, Value: a.Value})
	}
	controls := make([]proto.ControlSpec, 0, len(it.Controls))
	for _, c := range it.Controls {
		controls = append(controls, proto.ControlSpec{
			ID: c.ID, Kind: c.Kind, Label: c.Label, Value: c.Value,
			Min: c.Min, Max: c.Max, Step: c.Step, Bool: c.Bool,
			Text: c.Text, Options: c.Options,
		})
	}
	return proto.Item{
		ID: it.ID, Title: it.Title, Subtitle: it.Subtitle, Glyph: it.Glyph,
		Badge: it.Badge, Destructive: it.Destructive, Actions: actions,
		Controls: controls, HasChildren: it.HasChildren, Expandable: it.Expandable,
	}
}

func fromChange(c proto.Control) Change {
	return Change{Item: c.Item, Control: c.Control, Value: c.Value, Bool: c.Bool, Text: c.Text}
}
