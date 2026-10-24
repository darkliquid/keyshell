package eval

import (
	"fmt"
	"math"
	"strings"
)

// State is the launcher snapshot a frame resolves against.
type State struct {
	Name      string
	Open      bool
	Focused   bool
	Query     string
	Mode      string
	Selected  int
	Results   []Item
	Typing    float64 // 0..1 activity ratio
	Searching bool    // a query is in flight

	// Reveal is the host-resampled band geometry (spec §7).
	Reveal Reveal

	// Theme tokens and font registry (spec §4).
	Tokens map[string]Color
	// Vars holds theme variables and host overrides (spec §4).
	Vars   map[string]any
	Fonts  map[string]string

	// Summon is the per-open epoch for rand(); Seed feeds its draws;
	// RandCache persists between frames of one summon.
	Summon    int
	Seed      uint64
	RandCache *RandCache

	// Window size for layout (%) and scale.
	Width, Height float64
	Scale         float64

	// Panel is the interactive control panel state for the selected item.
	Panel Panel
}

// Panel is the launcher's control panel state, exposed as $panel.
type Panel struct {
	Open  bool
	Rows  []Item
	Focus int
	Dirty bool
}

// Reveal mirrors the host's per-summon band geometry.
type Reveal struct {
	Y, H, HLeft, HRight float64
	Angle, Shear        float64
	CardShear           float64
	D                   string // trapezoid outline for Path
	// FrameX, FrameY, Cw, Ch are the rotated content frame: a cw×ch
	// rectangle centred on the surface at (w/2, Yc) before the band
	// tilt is applied.
	FrameX, FrameY, Cw, Ch float64
}

// Item is the data a List template binds as $item.
type Item struct {
	ID, Title, Subtitle, Badge, Status string
	Glyph, Icon, Image, Preview        string
	Detail, Hint                       string
	StatusTone                         string
	Controls                           []ItemControl
	// Panel-row fields: a panel row is materialised as an Item so a List
	// template can render it. Kind is the row kind (action/range/toggle/
	// choice/text); Ratio/On/Text/Options carry the row value.
	Kind    string
	Ratio   float64
	On      bool
	Text    string
	Options []string
	Min     float64
	Max     float64
	Step    float64
}

// ItemControl is one item control ($item.<control>).
type ItemControl struct {
	ID      string
	Kind    string
	Label   string
	Value   string
	Ratio   float64
	Min     float64
	Max     float64
	Step    float64
	On      bool
	Options []string
	// Display steers card composition ("card" pins, "expanded" hides) and
	// Primary ranks the control for the accessory budget; both pass through
	// to the card composer unchanged.
	Display string
	Primary bool
}

// Env resolves bindings. Parent scopes chain outward: a List template
// scope wraps the scene scope so $item and $open both resolve.
type Env struct {
	st     *State
	parent *Env
	item   *Item
	// RefPaths resolves #id.prop cross-node references, keyed "id.prop".
	RefPaths map[string]Value
	vars     map[string]Value
}

func NewEnv(st *State, parent *Env) *Env {
	e := &Env{st: st, parent: parent, RefPaths: map[string]Value{}, vars: map[string]Value{}}
	if parent != nil {
		for k, v := range parent.RefPaths {
			e.RefPaths[k] = v
		}
		for k, v := range parent.vars {
			e.vars[k] = v
		}
	}
	return e
}

// Set binds a variable into the environment.
func (e *Env) Set(name string, v any) {
	if e.vars == nil {
		e.vars = make(map[string]Value)
	}
	if val, ok := toValue(v); ok {
		e.vars[name] = val
	}
}

// Get returns the value of a binding, if present.
func (e *Env) Get(name string) (Value, bool) {
	return e.lookupRef(name)
}

// ItemScope is the binding environment inside one List template
// instantiation.
func (e *Env) ItemScope(item *Item) *Env {
	return &Env{st: e.st, parent: e, item: item, RefPaths: e.RefPaths, vars: nil}
}

// ItemVal and ResultsVal wrap the structured bindings so member lookup
// is typed.
type ItemVal struct{ I *Item }
type ResultsVal struct{ Items []Item }

// RevealVal exposes the band geometry fields.
type RevealVal struct{ Reveal }

// ControlVal exposes one item control.
type ControlVal struct{ ItemControl }

// PanelVal exposes the control panel bindings.
type PanelVal struct{ Panel }

func (ItemVal) isValue()    {}
func (ResultsVal) isValue() {}
func (RevealVal) isValue()  {}
func (ControlVal) isValue() {}
func (PanelVal) isValue()   {}

func (e *Env) results() ResultsVal { return ResultsVal{Items: e.st.Results} }

func (e *Env) lookupRef(name string) (Value, bool) {
	if e.item != nil && name == "item" {
		return ItemVal{e.item}, true
	}
	if e.st != nil {
		switch name {
		case "open":
			return Bool(e.st.Open), true
		case "focused":
			return Bool(e.st.Focused), true
		case "query":
			return Str(e.st.Query), true
		case "results":
			return e.results(), true
		case "selected":
			return Num(e.st.Selected), true
		case "mode":
			return Str(e.st.Mode), true
		case "typing":
			return Num(e.st.Typing), true
		case "searching":
			return Bool(e.st.Searching), true
		case "active":
			if e.st.Selected >= 0 && e.st.Selected < len(e.st.Results) {
				return ItemVal{&e.st.Results[e.st.Selected]}, true
			}
			return Bool(false), true
		case "reveal":
			return RevealVal{e.st.Reveal}, true
		case "panel":
			return PanelVal{e.st.Panel}, true
		case "width":
			return Num(e.st.Width), true
		case "height":
			return Num(e.st.Height), true
		}
	}
	if e.vars != nil {
		if v, ok := e.vars[name]; ok {
			return v, true
		}
	}
	if e.st != nil && e.st.Tokens != nil {
		if c, ok := e.st.Tokens[name]; ok {
			return c, true
		}
	}
	if e.st != nil && e.st.Vars != nil {
		if v, ok := e.st.Vars[name]; ok {
			if val, ok := toValue(v); ok {
				return val, true
			}
		}
	}
	if e.parent != nil {
		return e.parent.lookupRef(name)
	}
	return nil, false
}

func (e *Env) member(v Value, name string) (Value, error) {
	switch x := v.(type) {
	case ResultsVal:
		if name == "len" {
			return Num(len(x.Items)), nil
		}
	case ItemVal:
		switch name {
		case "id":
			return Str(x.I.ID), nil
		case "title":
			return Str(x.I.Title), nil
		case "subtitle":
			return Str(x.I.Subtitle), nil
		case "badge":
			return Str(x.I.Badge), nil
		case "status":
			return Str(x.I.Status), nil
		case "glyph":
			return Str(x.I.Glyph), nil
		case "icon":
			return Str(x.I.Icon), nil
		case "image":
			return Str(x.I.Image), nil
		case "preview":
			return Str(x.I.Preview), nil
		case "detail":
			return Str(x.I.Detail), nil
		case "hint":
			return Str(x.I.Hint), nil
		case "statusTone":
			return Str(x.I.StatusTone), nil
		case "kind":
			return Str(x.I.Kind), nil
		case "ratio":
			return Num(x.I.Ratio), nil
		case "on":
			return Bool(x.I.On), nil
		case "min":
			return Num(x.I.Min), nil
		case "max":
			return Num(x.I.Max), nil
		case "step":
			return Num(x.I.Step), nil
		case "text":
			return Str(x.I.Text), nil
		case "options":
			out := make(Array, len(x.I.Options))
			for i, o := range x.I.Options {
				out[i] = Str(o)
			}
			return out, nil
		}
		for i := range x.I.Controls {
			if x.I.Controls[i].ID == name {
				return ControlVal{x.I.Controls[i]}, nil
			}
		}
	case ControlVal:
		switch name {
		case "kind":
			return Str(x.Kind), nil
		case "label":
			return Str(x.Label), nil
		case "value":
			return Str(x.Value), nil
		case "ratio":
			return Num(x.Ratio), nil
		case "on":
			return Bool(x.On), nil
		case "options":
			out := make(Array, len(x.Options))
			for i, o := range x.Options {
				out[i] = Str(o)
			}
			return out, nil
		}
	case PanelVal:
		switch name {
		case "open":
			return Bool(x.Open), nil
		case "focus":
			return Num(x.Focus), nil
		case "dirty":
			return Bool(x.Dirty), nil
		case "rows":
			return ResultsVal{Items: x.Rows}, nil
		}
	case RevealVal:
		switch name {
		case "frameX":
			return Num(x.FrameX), nil
		case "frameY":
			return Num(x.FrameY), nil
		case "cw":
			return Num(x.Cw), nil
		case "ch":
			return Num(x.Ch), nil
		case "y":
			return Num(x.Y), nil
		case "h":
			return Num(x.H), nil
		case "hLeft":
			return Num(x.HLeft), nil
		case "hRight":
			return Num(x.HRight), nil
		case "angle":
			return Length{V: x.Angle * 180 / math.Pi, Unit: "deg"}, nil
		case "shear":
			return Num(x.Shear), nil
		case "cardShear":
			return Num(x.CardShear), nil
		case "d":
			return Str(x.D), nil
		}
	}
	return nil, fmt.Errorf("no member %q on %T", name, v)
}

func (e *Env) call(v Value, name string, args []Value) (Value, error) {
	switch name {
	case "toUpper":
		if s, ok := v.(Str); ok {
			return Str(strings.ToUpper(string(s))), nil
		}
	case "toLower":
		if s, ok := v.(Str); ok {
			return Str(strings.ToLower(string(s))), nil
		}
	case "len":
		if s, ok := v.(Str); ok {
			return Num(len(s)), nil
		}
	case "round":
		if n, ok := v.(Num); ok {
			return Num(math.Round(float64(n))), nil
		}
	}
	return nil, fmt.Errorf("no method %q on %T", name, v)
}
