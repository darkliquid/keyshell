// Package proto defines the wire messages of the keyshell plugin protocol
// (version 1). Every message is one JSON object on one line, with a single
// top-level type key so decoding is unambiguous.
package proto

import (
	"encoding/json"
	"fmt"
)

// Protocol is the protocol version this package speaks.
const Protocol = 1

// Message is any wire message. Exactly one field is set, matching the top-level
// type key.
type Message struct {
	Hello   *Hello   `json:"hello,omitempty"`
	Init    *Init    `json:"init,omitempty"`
	Ready   *Ready   `json:"ready,omitempty"`
	Query   *Query   `json:"query,omitempty"`
	Item    *Item    `json:"item,omitempty"`
	Done    *Done    `json:"done,omitempty"`
	Error   *Err     `json:"error,omitempty"`
	Cancel  *Cancel  `json:"cancel,omitempty"`
	Control *Control `json:"control,omitempty"`
	State   *State   `json:"state,omitempty"`
	Invoke  *Invoke  `json:"invoke,omitempty"`
	Ping    *Ping    `json:"ping,omitempty"`
	Pong    *Pong    `json:"pong,omitempty"`
}

// Hello is the plugin's first message (plugin → host).
type Hello struct {
	Protocol   int      `json:"protocol"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Badge      string   `json:"badge"`
	Trigger    string   `json:"trigger"`
	Controls   []string `json:"controls"`
	Hot        bool     `json:"hot"`
	StableIDs  bool     `json:"stable_ids"`
	Searchable *bool    `json:"searchable,omitempty"` // nil means default (true)
}

// Init is the host's reply to Hello (host → plugin).
type Init struct {
	Instance string          `json:"instance"`
	Config   json.RawMessage `json:"config,omitempty"`
}

// Ready confirms the plugin's services are up (plugin → host).
type Ready struct{}

// Query asks for items. One request in flight per instance; Cancel supersedes.
type Query struct {
	ID     int      `json:"id"`
	Text   string   `json:"text"`
	Limit  int      `json:"limit"`
	Merged bool     `json:"merged"`
	Scope  []string `json:"scope,omitempty"`
}

// Item is one result.
type Item struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle,omitempty"`
	Glyph       string         `json:"glyph,omitempty"`
	Badge       string         `json:"badge,omitempty"`
	Destructive bool           `json:"destructive,omitempty"`
	Actions     []Action       `json:"actions,omitempty"`
	Controls    []ControlSpec  `json:"controls,omitempty"`
	HasChildren bool           `json:"has_children,omitempty"`
	Expandable  bool           `json:"expandable,omitempty"`
	Icon        string         `json:"icon,omitempty"`
	Image       string         `json:"image,omitempty"`
	Preview     string         `json:"preview,omitempty"`
	Status      string         `json:"status,omitempty"`
	StatusTone  string         `json:"status_tone,omitempty"`
	Detail      string         `json:"detail,omitempty"`
	Hint        string         `json:"hint,omitempty"`
	Card        []CardPartSpec `json:"card,omitempty"`
}

// Action is one declarative action.
type Action struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ControlSpec is an editable value a plugin offers on an item.
type ControlSpec struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Label   string   `json:"label"`
	Value   float64  `json:"value,omitempty"`
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Step    float64  `json:"step,omitempty"`
	Bool    bool     `json:"bool,omitempty"`
	Text    string   `json:"text,omitempty"`
	Options []string `json:"options,omitempty"`
	Display string   `json:"display,omitempty"`
	Primary bool     `json:"primary,omitempty"`
}

// CardPartSpec is one explicit card part in an override list.
type CardPartSpec struct {
	Kind    string `json:"kind"`
	Control string `json:"control,omitempty"`
	Text    string `json:"text,omitempty"`
	Tone    string `json:"tone,omitempty"`
}

// Done ends a query response (plugin → host).
type Done struct {
	ID int `json:"id"`
}

// Err reports a failed query or an invalid scope (plugin → host).
type Err struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

// Cancel tells the plugin to drop an in-flight query (host → plugin).
type Cancel struct {
	ID int `json:"id"`
}

// Control is a settled value change (host → plugin).
type Control struct {
	Item    string  `json:"item"`
	Control string  `json:"control"`
	Value   float64 `json:"value,omitempty"`
	Bool    bool    `json:"bool,omitempty"`
	Text    string  `json:"text,omitempty"`
}

// State carries current control values for reconciliation (plugin → host).
type State struct {
	Item     string             `json:"item"`
	Controls map[string]float64 `json:"controls"`
	Booleans map[string]bool    `json:"booleans,omitempty"`
}

// Invoke forwards a domain action (host → plugin).
type Invoke struct {
	Item   string `json:"item"`
	Action string `json:"action"`
	Value  string `json:"value,omitempty"`
}

// Ping is a liveness probe (host → plugin); Pong replies.
type Ping struct{}

// Pong confirms liveness.
type Pong struct {
	Ready bool `json:"ready"`
}

// typeKey maps a Message to its single populated key.
func typeKey(m Message) (string, error) {
	switch {
	case m.Hello != nil:
		return "hello", nil
	case m.Init != nil:
		return "init", nil
	case m.Ready != nil:
		return "ready", nil
	case m.Query != nil:
		return "query", nil
	case m.Item != nil:
		return "item", nil
	case m.Done != nil:
		return "done", nil
	case m.Error != nil:
		return "error", nil
	case m.Cancel != nil:
		return "cancel", nil
	case m.Control != nil:
		return "control", nil
	case m.State != nil:
		return "state", nil
	case m.Invoke != nil:
		return "invoke", nil
	case m.Ping != nil:
		return "ping", nil
	case m.Pong != nil:
		return "pong", nil
	}
	return "", fmt.Errorf("proto: empty message")
}

// Encode renders a message as a single JSON line (UTF-8, no trailing newline).
func Encode(m Message) ([]byte, error) {
	if _, err := typeKey(m); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Decode parses one JSON line into a Message. A malformed line or an unknown
// or empty top-level key yields an error so the caller can drop it.
func Decode(line []byte) (Message, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(line, &probe); err != nil {
		return Message{}, err
	}
	var m Message
	// Route by the single key present.
	switch {
	case probe["hello"] != nil:
		err := json.Unmarshal(probe["hello"], &m.Hello)
		return m, err
	case probe["init"] != nil:
		err := json.Unmarshal(probe["init"], &m.Init)
		return m, err
	case probe["ready"] != nil:
		m.Ready = &Ready{}
		return m, nil
	case probe["query"] != nil:
		err := json.Unmarshal(probe["query"], &m.Query)
		return m, err
	case probe["item"] != nil:
		err := json.Unmarshal(probe["item"], &m.Item)
		return m, err
	case probe["done"] != nil:
		err := json.Unmarshal(probe["done"], &m.Done)
		return m, err
	case probe["error"] != nil:
		err := json.Unmarshal(probe["error"], &m.Error)
		return m, err
	case probe["cancel"] != nil:
		err := json.Unmarshal(probe["cancel"], &m.Cancel)
		return m, err
	case probe["control"] != nil:
		err := json.Unmarshal(probe["control"], &m.Control)
		return m, err
	case probe["state"] != nil:
		err := json.Unmarshal(probe["state"], &m.State)
		return m, err
	case probe["invoke"] != nil:
		err := json.Unmarshal(probe["invoke"], &m.Invoke)
		return m, err
	case probe["ping"] != nil:
		m.Ping = &Ping{}
		return m, nil
	case probe["pong"] != nil:
		err := json.Unmarshal(probe["pong"], &m.Pong)
		return m, err
	}
	return Message{}, fmt.Errorf("proto: unknown or empty message key")
}
