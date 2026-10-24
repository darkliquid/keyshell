package ksl

import (
	"fmt"
	"strconv"
	"strings"
)

// Scene is one parsed .ksl file. Items preserve declaration order.
type Scene struct {
	Name  string
	Items []Item
}

// Item is one top-level declaration: a node or one of the non-node
// declarations. The unexported method keeps external packages from
// implementing it.
type Item interface{ itemPos() Position }

// Positioner is anything carrying a source position.
type Positioner interface{ Pos() Position }

// Node is a scene node: `Name#id.class { ... }`.
type Node struct {
	Pos      Position
	Name     string
	ID       string
	Classes  []string
	Props    []*Prop
	Children []*Node
	States   []*StateBlock // node-level state blocks (Grammar Rule 3)
}

// Prop is `key: value`. The `animation` property's Val is *AnimVal.
type Prop struct {
	Pos Position
	Key string
	Val Value
}

// StateBlock serves both state forms (Grammar Rule 3): inside a node it
// holds Props; at scene level it holds Targets.
type StateBlock struct {
	Pos     Position
	Name    string
	Props   []*Prop
	Targets []*StyleBlock
}

// StyleBlock is a selector rule (spec §4).
type StyleBlock struct {
	Pos   Position
	Sel   Selector
	Props []*Prop
}

// Selector matches nodes by type, #id, .class, pseudo-state and one
// optional attribute.
type Selector struct {
	Pos      Position
	Type     string
	ID       string
	Classes  []string
	Pseudos  []string
	AttrName string
	AttrVal  string
}

// ThemeBlock carries theme tokens and variables, named font families and asset paths.
type ThemeBlock struct {
	Pos    Position
	Tokens []*Prop
	Fonts  []*Prop
	Assets []Value
}

// Transition maps a state name to per-property timings.
type Transition struct {
	Pos   Position
	State string
	Props []*TimingProp
}

// TimingProp is one `key: duration easing?` entry.
type TimingProp struct {
	Pos    Position
	Key    string
	Dur    Scalar
	Easing Value // *Keyword or cubic-bezier *FuncCall; nil when absent
}

// Keyframes is a named, ordered set of percentage frames.
type Keyframes struct {
	Pos    Position
	Name   string
	Frames []*Frame
}

// Frame is one `N% { props }` keyframe.
type Frame struct {
	Pos     Position
	Percent float64
	Props   []*Prop
}

// Def is `def Name { ... }`, a reusable component (spec §11).
type Def struct {
	Pos      Position
	Name     string
	Props    []*Prop
	Children []*Node
	States   []*StateBlock
}

func (n *Node) itemPos() Position       { return n.Pos }
func (b *ThemeBlock) itemPos() Position { return b.Pos }
func (b *StateBlock) itemPos() Position { return b.Pos }
func (b *StyleBlock) itemPos() Position { return b.Pos }
func (t *Transition) itemPos() Position { return t.Pos }
func (k *Keyframes) itemPos() Position  { return k.Pos }
func (d *Def) itemPos() Position        { return d.Pos }

// Typed accessors over the ordered item list.
func (s *Scene) Nodes() []*Node { return itemsOf[*Node](s) }

func (s *Scene) ThemeBlocks() []*ThemeBlock { return itemsOf[*ThemeBlock](s) }
func (s *Scene) States() []*StateBlock      { return itemsOf[*StateBlock](s) }
func (s *Scene) Styles() []*StyleBlock      { return itemsOf[*StyleBlock](s) }
func (s *Scene) Transitions() []*Transition { return itemsOf[*Transition](s) }
func (s *Scene) Keyframes() []*Keyframes    { return itemsOf[*Keyframes](s) }
func (s *Scene) Defs() []*Def               { return itemsOf[*Def](s) }

func itemsOf[T Item](s *Scene) []T {
	var out []T
	for _, it := range s.Items {
		if x, ok := it.(T); ok {
			out = append(out, x)
		}
	}
	return out
}

// Value is one parsed property value or expression operand. Every
// value carries its position for diagnostics and renders itself
// compactly for tests and error messages.
type Value interface {
	Positioner
	fmt.Stringer
}

// Scalar is a number with an optional unit: 12px, 40%, 250ms, 0.4.
type Scalar struct {
	At   Position
	N    float64
	Unit string
}

// ColorVal is a 3/4/6/8-digit hex colour, without the leading '#'.
type ColorVal struct {
	At  Position
	Hex string
}

// StringVal is a decoded string literal.
type StringVal struct {
	At Position
	S  string
}

// Ref is a binding head: $accent, $item, $results.
type Ref struct {
	At   Position
	Name string // without '$'
}

// RefPath is a cross-node reference: #band.width.
type RefPath struct {
	At   Position
	ID   string // without '#'
	Prop string
}

// Keyword is a bare identifier used as a value: tone names, alignment,
// easing names (validated per property later).
type Keyword struct {
	At   Position
	Name string
}

// BoolVal is true or false.
type BoolVal struct {
	At Position
	B  bool
}

// FuncCall covers effects and helpers: blur(8px), tilt(12deg), rand(0, 1).
type FuncCall struct {
	At   Position
	Name string
	Args []Value
}

// ArrayVal is `[ value, value, ... ]`.
type ArrayVal struct {
	At    Position
	Elems []Value
}

// InlineBlock is a property value that is itself a property block:
// `viewport: { height: 320px }`.
type InlineBlock struct {
	At    Position
	Props []*Prop
}

// Unary is -x or !x.
type Unary struct {
	At Position
	Op string
	X  Value
}

// Binary is an infix operation: + - * / == != < <= > >= && ||.
type Binary struct {
	At   Position
	Op   string
	L, R Value
}

// Ternary is `cond ? then : else`.
type Ternary struct {
	At               Position
	Cond, Then, Else Value
}

// Member is dotted access on a value: $results.len.
type Member struct {
	At   Position
	X    Value
	Name string
}

// MethodCall is a member call: $item.title.toUpper().
type MethodCall struct {
	At   Position
	X    Value
	Name string
	Args []Value
}

func (v *Scalar) Pos() Position      { return v.At }
func (v *ColorVal) Pos() Position    { return v.At }
func (v *StringVal) Pos() Position   { return v.At }
func (v *Ref) Pos() Position         { return v.At }
func (v *RefPath) Pos() Position     { return v.At }
func (v *Keyword) Pos() Position     { return v.At }
func (v *BoolVal) Pos() Position     { return v.At }
func (v *FuncCall) Pos() Position    { return v.At }
func (v *ArrayVal) Pos() Position    { return v.At }
func (v *InlineBlock) Pos() Position { return v.At }
func (v *Unary) Pos() Position       { return v.At }
func (v *Binary) Pos() Position      { return v.At }
func (v *Ternary) Pos() Position     { return v.At }
func (v *Member) Pos() Position      { return v.At }
func (v *MethodCall) Pos() Position  { return v.At }

func fmtNum(n float64) string { return strconv.FormatFloat(n, 'g', -1, 64) }

func (v *Scalar) String() string    { return fmtNum(v.N) + v.Unit }
func (v *ColorVal) String() string  { return "#" + v.Hex }
func (v *StringVal) String() string { return strconv.Quote(v.S) }
func (v *Ref) String() string       { return "$" + v.Name }
func (v *RefPath) String() string   { return "#" + v.ID + "." + v.Prop }
func (v *Keyword) String() string   { return v.Name }

func (v *BoolVal) String() string {
	if v.B {
		return "true"
	}
	return "false"
}

func (v *FuncCall) String() string {
	parts := make([]string, len(v.Args))
	for i, a := range v.Args {
		parts[i] = a.String()
	}
	return v.Name + "(" + strings.Join(parts, ", ") + ")"
}

func (v *ArrayVal) String() string {
	parts := make([]string, len(v.Elems))
	for i, e := range v.Elems {
		parts[i] = e.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (v *InlineBlock) String() string {
	parts := make([]string, len(v.Props))
	for i, pr := range v.Props {
		parts[i] = pr.Key + ": " + pr.Val.String()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func (v *Unary) String() string  { return v.Op + v.X.String() }
func (v *Binary) String() string { return "(" + v.L.String() + " " + v.Op + " " + v.R.String() + ")" }

func (v *Ternary) String() string {
	return v.Cond.String() + " ? " + v.Then.String() + " : " + v.Else.String()
}

func (v *Member) String() string { return v.X.String() + "." + v.Name }

func (v *MethodCall) String() string {
	parts := make([]string, len(v.Args))
	for i, a := range v.Args {
		parts[i] = a.String()
	}
	return v.X.String() + "." + v.Name + "(" + strings.Join(parts, ", ") + ")"
}

// AnimVal is the `animation` property value (Grammar Rule 6): one or
// more comma-separated steps.
type AnimVal struct {
	At    Position
	Steps []AnimStep
}

// AnimStep is `name [duration] [easing] [iteration] [direction]`.
type AnimStep struct {
	Name      string
	Dur       *Scalar // nil when absent
	Easing    Value   // *Keyword or cubic-bezier *FuncCall; nil when absent
	Iteration Value   // *Scalar (count) or *Keyword ("infinite"); nil when absent
	Direction string  // "", "alternate", "reverse", "alternate reverse"
}

func (v *AnimVal) Pos() Position { return v.At }

func (v *AnimVal) String() string {
	parts := make([]string, len(v.Steps))
	for i, s := range v.Steps {
		parts[i] = s.Name
		if s.Dur != nil {
			parts[i] += " " + s.Dur.String()
		}
		if s.Easing != nil {
			parts[i] += " " + s.Easing.String()
		}
		if s.Iteration != nil {
			parts[i] += " " + s.Iteration.String()
		}
		if s.Direction != "" {
			parts[i] += " " + s.Direction
		}
	}
	return strings.Join(parts, ", ")
}
