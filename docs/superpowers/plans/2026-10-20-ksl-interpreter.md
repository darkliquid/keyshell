# KSL Interpreter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the KSL interpreter: resolve a parsed `ksl.Scene` against current launcher state into a paintable `Frame` — typed property values, evaluated expressions, cascade/states/`when`, List materialisation, and layout rectangles — with zero graphics dependencies, so the renderer plan only has to paint.

**Architecture:** One new package `internal/ksl/eval` importing only `internal/ksl` (the AST) and stdlib. Two layers inside it: a value layer (typed runtime values, binding environment, expression evaluator, `rand`/`jitter`) and a structure layer (cascade + states + `when`, List materialisation, layout). The output `Frame` is plain data: every node with a resolved property map, a `RectF`, and ordered children. Drawing is deliberately absent — text measurement is injected behind a `Measurer` interface so `eval` never imports gogpu.

**Tech Stack:** Go 1.27.2, stdlib only (`fmt`, `math`, `strconv`, `strings`, `time`, `hash/fnv`), `internal/ksl` for the AST. Test fixtures reuse `../ksl/testdata` from the parser plan.

**Spec:** `docs/superpowers/specs/2026-10-20-keyshell-ui-language-design.md` (§3 `when`, §4 cascade, §5 states, §7 bindings/expressions/randomisation, §2 layout, §9 lists); parser plan `2026-10-20-ksl-parser.md` defines the AST this consumes.

**Scope check:** This is the interpreter subsystem only. The renderer integration (painting, default scene, config plumbing) is plan `2026-10-20-ksl-renderer.md`; imagery, effects and the animation clock are plan `2026-10-20-ksl-imagery-effects-animation.md`. This plan ships working, testable software on its own: scenes resolve to Frames from synthetic state.

## Global Constraints

- Pure Go, `CGO_ENABLED=0` (mise `[env]`).
- `internal/ksl/eval` imports: stdlib + `internal/ksl`. **No** `gogpu`, `gg`, `theme`, `render`, `card`, `backend` — the Frame is data, and the item schema is eval's own `Item` type (plan 3 maps `backend.Item` into it).
- Tests: `go test ./internal/ksl/eval/...`; suite `mise run test`; gate `mise run check`.
- Commits: `feat(eval):` / `test(eval):`, first line under 72 chars.
- Evaluation is allocation-frugal but not yet allocation-free: per-frame pooling is deferred until a benchmark demands it (the scene is tens of nodes).
- Binding semantics follow spec §7 verbatim: `$open`, `$focused`, `$query`, `$results` (with `.len`), `$selected`, `$mode`, `$typing` (0..1), `$reveal.*`, `$item.*` inside List templates, `$active` anywhere.

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/ksl/eval/value.go` | Runtime value types (`Num`, `Str`, `Bool`, `Color`, `Length`) and AST-literal conversion |
| `internal/ksl/eval/env.go` | `State` (launcher + theme + reveal snapshot), `Item` schema, binding lookup, `$results`/`$item` member resolution |
| `internal/ksl/eval/expr.go` | Expression evaluator: operators, ternary, members, method calls, type rules |
| `internal/ksl/eval/random.go` | `rand(min, max)` summon-epoch cache + `jitter(key, min, max)` FNV-1a hash |
| `internal/ksl/eval/cascade.go` | Selector matching, state blocks (node and scene level), `when`, per-node property resolution |
| `internal/ksl/eval/layout.go` | `RectF`, `Measurer`, flex layout (Row/Column/Stack/Grid), units, scale, constraints |
| `internal/ksl/eval/resolve.go` | `Resolve(scene, state)` → `Frame`: List materialisation, z-order, assembly |
| one `_test.go` per file | Table tests; `../ksl/testdata` scenes as integration fixtures |

Tasks are ordered so each commits a compiling, tested increment.

---

### Task 1: Runtime values

**Files:**
- Create: `internal/ksl/eval/value.go`
- Test: `internal/ksl/eval/value_test.go`

**Interfaces:**
- Consumes: `ksl.Scalar`, `ksl.ColorVal`, `ksl.StringVal`, `ksl.Keyword`, `ksl.BoolVal`, `ksl.Ref`, `ksl.RefPath`, `ksl.ArrayVal`, `ksl.InlineBlock` (parser AST).
- Produces: `type Value interface{ isValue() }` with variants `Num float64`... wait — Go has no sum types; define concrete structs:

```go
type Value interface{ isValue() }

type Num float64                  // dimensionless number
type Str string
type Bool bool
type Length struct { V float64; Unit string } // "", "px", "%", "deg", "ms", "s"
type Color struct { R, G, B, A uint8 }
type Kw string                    // unresolved keyword (tone, align, ...)
type Array []Value                // kept for assets; []Value not []Num
type Props []PropVal              // inline block: ordered key/value
type PropVal struct { Key string; Val Value }
```

and `func literal(v ksl.Value) (Value, error)` converting AST primaries. `hexColor` decodes 3/4/6/8-digit hex per Grammar Rule 20.

- [ ] **Step 1: Write the failing test**

```go
package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func TestLiteralConversion(t *testing.T) {
	tests := []struct {
		src  string
		want Value
	}{
		{"24", Num(24)},
		{"12px", Length{V: 12, Unit: "px"}},
		{"40%", Length{V: 40, Unit: "%"}},
		{"250ms", Length{V: 250, Unit: "ms"}},
		{`"hi"`, Str("hi")},
		{"true", Bool(true)},
		{"accent", Kw("accent")},
		{"#FF2E63", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"#f2e", Color{0xFF, 0x22, 0xEE, 0xFF}},
		{"#f2e8", Color{0xFF, 0x22, 0xEE, 0x88}},
		{"#FF2E63CC", Color{0xFF, 0x2E, 0x63, 0xCC}},
	}
	for _, tt := range tests {
		s, err := ksl.Parse("t.ksl", []byte("Box { x: "+tt.src+" }"))
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.src, err)
		}
		got, err := literal(s.Nodes()[0].Props[0].Val)
		if err != nil {
			t.Fatalf("literal(%q): %v", tt.src, err)
		}
		if got != tt.want {
			t.Errorf("literal(%q) = %#v, want %#v", tt.src, got, tt.want)
		}
	}
}

func TestLiteralErrors(t *testing.T) {
	for _, src := range []string{"#12", "12.5.3"} {
		s, _ := ksl.Parse("t.ksl", []byte("Box { x: "+src+" }"))
		if s == nil { // hex: parse error is fine too
			continue
		}
		if _, err := literal(s.Nodes()[0].Props[0].Val); err == nil {
			t.Errorf("literal(%q) = nil error, want failure", src)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

```go
// Package eval resolves a parsed KSL scene against launcher state into
// a paintable Frame: typed property values, cascade/states/when, List
// materialisation and layout. It never imports graphics code; text
// measurement is injected behind Measurer.
package eval

import (
	"fmt"
	"strconv"

	"github.com/darkliquid/keyshell/internal/ksl"
)

type Value interface{ isValue() }

type Num float64
type Str string
type Bool bool

type Length struct {
	V    float64
	Unit string // "", "px", "%", "deg", "ms", "s"
}

type Color struct{ R, G, B, A uint8 }

// Kw is a bare keyword value (tone, alignment, easing name) awaiting
// per-property resolution.
type Kw string

type Array []Value

type Props []PropVal

type PropVal struct {
	Key string
	Val Value
}

func (Num) isValue()    {}
func (Str) isValue()    {}
func (Bool) isValue()   {}
func (Length) isValue() {}
func (Color) isValue()  {}
func (Kw) isValue()     {}
func (Array) isValue()  {}
func (Props) isValue()  {}

// literal converts an AST primary to a runtime value. Anything with
// bindings or operators is Task 3's job.
func literal(v ksl.Value) (Value, error) {
	switch x := v.(type) {
	case *ksl.Scalar:
		if x.Unit == "" {
			return Num(x.N), nil
		}
		return Length{V: x.N, Unit: x.Unit}, nil
	case *ksl.StringVal:
		return Str(x.S), nil
	case *ksl.BoolVal:
		return Bool(x.B), nil
	case *ksl.ColorVal:
		return hexColor(x.Hex)
	case *ksl.Keyword:
		return Kw(x.Name), nil
	case *ksl.ArrayVal:
		var out Array
		for _, e := range x.Elems {
			ev, err := literal(e)
			if err != nil {
				return nil, err
			}
			out = append(out, ev)
		}
		return out, nil
	case *ksl.InlineBlock:
		var out Props
		for _, pr := range x.Props {
			pv, err := literal(pr.Val)
			if err != nil {
				return nil, err
			}
			out = append(out, PropVal{Key: pr.Key, Val: pv})
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s: not a literal: %s", v.Pos(), v)
}

// hexColor decodes 3/4/6/8-digit hex with CSS shorthand semantics.
func hexColor(h string) (Color, error) {
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("bad hex colour %q", h)
	}
	switch len(h) {
	case 3: // RGB
		return Color{uint8(n>>8&0xF) * 0x11, uint8(n>>4&0xF) * 0x11, uint8(n&0xF) * 0x11, 0xFF}, nil
	case 4: // RGBA
		return Color{uint8(n>>12&0xF) * 0x11, uint8(n>>8&0xF) * 0x11, uint8(n>>4&0xF) * 0x11, uint8(n&0xF) * 0x11}, nil
	case 6:
		return Color{uint8(n >> 16), uint8(n >> 8 & 0xFF), uint8(n & 0xFF), 0xFF}, nil
	case 8:
		return Color{uint8(n >> 24), uint8(n >> 16 & 0xFF), uint8(n >> 8 & 0xFF), uint8(n & 0xFF)}, nil
	}
	return Color{}, fmt.Errorf("hex colours have 3, 4, 6 or 8 digits, got %d", len(h))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/value.go internal/ksl/eval/value_test.go
git commit -m "feat(eval): runtime value types and literal conversion"
```

---

### Task 2: State, bindings and the item schema

**Files:**
- Create: `internal/ksl/eval/env.go`
- Test: `internal/ksl/eval/env_test.go`

**Interfaces:**
- Consumes: `Value` (Task 1), `ksl.Ref`, `ksl.Member`, `ksl.MethodCall`.
- Produces: `State` (the frame's input), `Item` (template data), `Env` (binding lookup with parent scope), `func (e *Env) lookupRef(name string) (Value, bool)`, `func (e *Env) member(v Value, name string) (Value, error)`, `func (e *Env) call(v Value, name string, args []Value) (Value, error)`.

```go
// State is the launcher snapshot a frame resolves against.
type State struct {
	Open     bool
	Focused  bool
	Query    string
	Mode     string
	Selected int
	Results  []Item
	Typing   float64 // 0..1 activity ratio

	// Reveal is the host-resampled band geometry (spec §7).
	Reveal Reveal

	// Theme tokens and font registry (spec §4).
	Tokens map[string]Color
	Fonts  map[string]string

	// Summon is the per-open epoch counter for rand(); Seed feeds its
	// draws; RandCache persists between frames of one summon.
	Summon    int
	Seed      uint64
	RandCache *RandCache

	// Window size for layout (%) and scale.
	Width, Height float64
	Scale         float64
}

type Reveal struct {
	Y, H, HLeft, HRight float64
	Angle, Shear        float64
	CardShear           float64
	D                   string // trapezoid outline for Path
}

// Item is the data a List template binds as $item.
type Item struct {
	ID, Title, Subtitle, Badge, Status string
	Glyph, Icon, Image, Preview        string
	Detail, Hint                       string
	StatusTone                         string
	Controls                           map[string]ItemControl
}

type ItemControl struct {
	Value string
	Ratio float64
	On    bool
}
```

- [ ] **Step 1: Write the failing test**

```go
package eval

import "testing"

func TestBindings(t *testing.T) {
	st := State{
		Open: true, Query: "fire", Mode: "apps",
		Results: []Item{{ID: "a", Title: "Firefox"}, {ID: "b", Title: "Firejail"}},
		Tokens:  map[string]Color{"accent": {0xFF, 0x2E, 0x63, 0xFF}},
	}
	env := NewEnv(&st, nil)
	for _, tc := range []struct{ ref string; want Value }{
		{"open", Bool(true)},
		{"query", Str("fire")},
		{"mode", Str("apps")},
		{"selected", Num(0)},
		{"accent", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"active", ItemVal{&st.Results[0]}},
	} {
		got, ok := env.lookupRef(tc.ref)
		if !ok || got != tc.want {
			t.Errorf("lookupRef(%q) = %#v, %v; want %#v", tc.ref, got, ok, tc.want)
		}
	}
}

func TestMembersAndMethods(t *testing.T) {
	st := State{Results: []Item{{Title: "Firefox"}}}
	env := NewEnv(&st, nil)
	if got, err := env.member(env.results(), "len"); err != nil || got != Num(1) {
		t.Errorf("$results.len = %v, %v", got, err)
	}
	item := ItemVal{&st.Results[0]}
	if got, err := env.member(item, "title"); err != nil || got != Str("Firefox") {
		t.Errorf("$item.title = %v, %v", got, err)
	}
	if got, err := env.call(Str("enter"), "toUpper", nil); err != nil || got != Str("ENTER") {
		t.Errorf("toUpper = %v, %v", got, err)
	}
	if _, err := env.member(Str("x"), "bogus"); err == nil {
		t.Errorf("unknown member = nil error, want failure")
	}
}

func TestItemScope(t *testing.T) {
	st := State{Results: []Item{{ID: "a", Title: "Firefox"}}}
	env := NewEnv(&st, nil)
	inner := env.ItemScope(&st.Results[0])
	if got, ok := inner.lookupRef("item"); !ok || got != (ItemVal{&st.Results[0]}) {
		t.Errorf("$item = %v, %v", got, ok)
	}
	// parent bindings stay visible through the item scope
	if got, ok := inner.lookupRef("open"); !ok || got != Bool(false) {
		t.Errorf("$open through scope = %v, %v", got, ok)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -run 'TestBindings|TestMembers|TestItemScope' -v`
Expected: FAIL — undefined types/functions.

- [ ] **Step 3: Write the implementation**

```go
package eval

import "fmt"

// Env resolves bindings. Parent scopes chain outward: a List template
// scope wraps the scene scope so $item and $open both resolve.
type Env struct {
	st     *State
	parent *Env
	item   *Item // set in ItemScope
}

func NewEnv(st *State, parent *Env) *Env { return &Env{st: st, parent: parent} }

// ItemScope is the binding environment inside one List template
// instantiation.
func (e *Env) ItemScope(item *Item) *Env {
	return &Env{st: e.st, parent: e, item: item}
}

// ItemVal and ResultsVal wrap the structured bindings so member lookup
// is typed.
type ItemVal struct{ I *Item }
type ResultsVal struct{ Items []Item }

func (ItemVal) isValue()    {}
func (ResultsVal) isValue() {}

func (e *Env) results() ResultsVal { return ResultsVal{Items: e.st.Results} }

func (e *Env) lookupRef(name string) (Value, bool) {
	if e.item != nil {
		if name == "item" {
			return ItemVal{e.item}, true
		}
	}
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
	case "active":
		if e.st.Selected >= 0 && e.st.Selected < len(e.st.Results) {
			return ItemVal{&e.st.Results[e.st.Selected]}, true
		}
		return Bool(false), true
	case "reveal":
		return RevealVal{e.st.Reveal}, true
	}
	if c, ok := e.st.Tokens[name]; ok {
		return c, true
	}
	if e.parent != nil {
		return e.parent.lookupRef(name)
	}
	return nil, false
}

// RevealVal exposes the band geometry fields.
type RevealVal struct{ Reveal }

func (RevealVal) isValue() {}

func (e *Env) member(v Value, name string) (Value, error) {
	switch x := v.(type) {
	case ResultsVal:
		if name == "len" {
			return Num(len(x.Items)), nil
		}
	case ItemVal:
		switch name {
		case "id":     return Str(x.I.ID), nil
		case "title":  return Str(x.I.Title), nil
		case "subtitle": return Str(x.I.Subtitle), nil
		case "badge":  return Str(x.I.Badge), nil
		case "status": return Str(x.I.Status), nil
		case "glyph":  return Str(x.I.Glyph), nil
		case "icon":   return Str(x.I.Icon), nil
		case "image":  return Str(x.I.Image), nil
		case "preview": return Str(x.I.Preview), nil
		case "detail": return Str(x.I.Detail), nil
		case "hint":   return Str(x.I.Hint), nil
		}
		if c, ok := x.I.Controls[name]; ok {
			return ControlVal{c}, true, nil // see note below
		}
	case RevealVal:
		switch name {
		case "y":         return Num(x.Y), nil
		case "h":         return Num(x.H), nil
		case "hLeft":     return Num(x.HLeft), nil
		case "hRight":    return Num(x.HRight), nil
		case "angle":     return Num(x.Angle), nil
		case "shear":     return Num(x.Shear), nil
		case "cardShear": return Num(x.CardShear), nil
		case "d":         return Str(x.D), nil
		}
	}
	return nil, fmt.Errorf("no member %q on %T", name, v)
}

// ControlVal exposes an item's control parts.
type ControlVal struct{ ItemControl }

func (ControlVal) isValue() {}

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
	}
	return nil, fmt.Errorf("no method %q on %T", name, v)
}
```

Fix the signature typo above: member access returning a ControlVal stays `(Value, error)` — `return ControlVal{c}, nil` (the `true` was a drafting slip; do not ship it). Add `import "strings"`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/env.go internal/ksl/eval/env_test.go
git commit -m "feat(eval): bindings, item schema and member lookup"
```

---

### Task 3: Expression evaluator

**Files:**
- Create: `internal/ksl/eval/expr.go`
- Test: `internal/ksl/eval/expr_test.go`

**Interfaces:**
- Consumes: `Value` (Task 1), `Env.lookupRef/member/call` (Task 2), `ksl` AST values.
- Produces: `func (e *Env) Eval(v ksl.Value) (Value, error)` — the full Values production including operators, ternary, refs, refpaths, funcs (effects stay *deferred*: `blur(...)` evaluates to `FuncVal`).

- [ ] **Step 1: Write the failing test**

```go
package eval

import "testing"

func evalOne(t *testing.T, st *State, src string) (Value, error) {
	t.Helper()
	s, err := ksl.Parse("t.ksl", []byte("Box { x: "+src+" }"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return NewEnv(st, nil).Eval(s.Nodes()[0].Props[0].Val)
}

func TestEvalExpr(t *testing.T) {
	st := &State{Query: "fire", Selected: 1,
		Results: []Item{{Title: "Firefox"}, {Title: "Firejail"}},
		Tokens:  map[string]Color{"accent": {0xFF, 0x2E, 0x63, 0xFF}},
		Reveal:  Reveal{H: 64, Angle: 2}}
	tests := []struct{ src string; want Value }{
		{"$results.len * 12", Num(24)},
		{`$query + "!"`, Str("fire!")},
		{"$selected == 1", Bool(true)},
		{"$results.len > 1", Bool(true)},
		{`$open ? 1 : 2`, Num(2)},
		{"$reveal.h / 2", Num(32)},
		{"rand(1, 1)", Num(1)}, // degenerate range is exact
		{"$accent", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"#band.width", Num(640)}, // refpath resolves against Frame (Task 7 stubs it)
		{`$item.title.toUpper()`, Str("")}, // no item in scope
		{"linear-gradient($accent, #000)", FuncVal{Name: "linear-gradient"}},
	}
	for _, tt := range tests {
		got, err := evalOne(t, st, tt.src)
		if err != nil {
			t.Fatalf("Eval(%q): %v", tt.src, err)
		}
		if got != tt.want {
			t.Errorf("Eval(%q) = %#v, want %#v", tt.src, got, tt.want)
		}
	}
}

func TestEvalErrors(t *testing.T) {
	st := &State{}
	for _, src := range []string{
		`$query + 1`,   // str + num: type error (concat needs a str)
		`$open + 1`,    // bool arithmetic
		"$nope",        // unknown binding
		"$results.foo", // unknown member
		`$query.nope()`,
	} {
		if _, err := evalOne(t, st, src); err == nil {
			t.Errorf("Eval(%q) = nil error, want failure", src)
		}
	}
}
```

Notes for the implementer: `$open ? 1 : 2` with `Open: false` → `Num(2)`. `$item` with no item scope evaluates to an error; the want above assumes the fixture defines one — keep the test honest: give the ItemVal fixture explicitly:

```go
{"$item.title.toUpper()", ...}
```
becomes a separate test constructing `env.ItemScope(&Item{Title: "hi"})` and asserting `Str("HI")`.

`#band.width` needs a resolved-value lookup table that only exists once `Frame` exists (Task 7); for this task, have `Env` expose `RefPaths map[string]Value` (keyed `"id.prop"`) that plan 3 or Task 7 fills, and seed it in the test.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -run TestEval -v`
Expected: FAIL — `Eval` undefined.

- [ ] **Step 3: Write the implementation**

```go
package eval

import (
	"fmt"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// Eval evaluates an AST value against the environment.
func (e *Env) Eval(v ksl.Value) (Value, error) {
	switch x := v.(type) {
	case *ksl.Scalar, *ksl.StringVal, *ksl.BoolVal, *ksl.ColorVal, *ksl.Keyword,
		*ksl.ArrayVal, *ksl.InlineBlock:
		return literal(v)
	case *ksl.Ref:
		if got, ok := e.lookupRef(x.Name); ok {
			return got, nil
		}
		return nil, fmt.Errorf("%s: unknown binding $%s", x.Pos(), x.Name)
	case *ksl.RefPath:
		key := x.ID + "." + x.Prop
		if got, ok := e.RefPaths[key]; ok {
			return got, nil
		}
		return nil, fmt.Errorf("%s: unknown reference #%s", x.Pos(), key)
	case *ksl.FuncCall:
		if x.Name == "rand" {
			return e.evalRand(x)
		}
		if x.Name == "jitter" {
			return e.evalJitter(x)
		}
		args := make([]Value, len(x.Args))
		for i, a := range x.Args {
			av, err := e.Eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = av
		}
		return FuncVal{At: x.Pos(), Name: x.Name, Args: args}, nil
	case *ksl.Unary:
		xv, err := e.Eval(x.X)
		if err != nil {
			return nil, err
		}
		switch x.Op {
		case "-":
			switch n := xv.(type) {
			case Num:
				return -n, nil
			case Length:
				n.V = -n.V
				return n, nil
			}
		case "!":
			if b, ok := xv.(Bool); ok {
				return !b, nil
			}
		}
		return nil, fmt.Errorf("%s: bad operand for %s: %T", x.Pos(), x.Op, xv)
	case *ksl.Binary:
		return e.evalBinary(x)
	case *ksl.Ternary:
		cv, err := e.Eval(x.Cond)
		if err != nil {
			return nil, err
		}
		if !truthy(cv) {
			return e.Eval(x.Else)
		}
		return e.Eval(x.Then)
	case *ksl.Member:
		xv, err := e.Eval(x.X)
		if err != nil {
			return nil, err
		}
		return e.member(xv, x.Name)
	case *ksl.MethodCall:
		xv, err := e.Eval(x.X)
		if err != nil {
			return nil, err
		}
		args := make([]Value, len(x.Args))
		for i, a := range x.Args {
			av, err := e.Eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = av
		}
		return e.call(xv, x.Name, args)
	}
	return nil, fmt.Errorf("%s: unsupported value %T", v.Pos(), v)
}
```

With helpers:

```go
// truthy implements the spec's truthiness: non-zero number, non-empty
// string, true.
func truthy(v Value) bool {
	switch x := v.(type) {
	case Bool:
		return bool(x)
	case Num:
		return x != 0
	case Str:
		return x != ""
	case Length:
		return x.V != 0
	}
	return v != nil
}

// FuncVal is an evaluated effect/helper call (blur, shadow, gradients,
// tilt); the painter interprets it.
type FuncVal struct {
	At   ksl.Position
	Name string
	Args []Value
}

func (FuncVal) isValue() {}

func (e *Env) evalBinary(x *ksl.Binary) (Value, error) {
	l, err := e.Eval(x.L)
	if err != nil {
		return nil, err
	}
	r, err := e.Eval(x.R)
	if err != nil {
		return nil, err
	}
	switch x.Op {
	case "&&":
		return Bool(truthy(l) && truthy(r)), nil
	case "||":
		return Bool(truthy(l) || truthy(r)), nil
	}
	ln, lok := asNum(l)
	rn, rok := asNum(r)
	if lok && rok {
		switch x.Op {
		case "+":
			return ln + rn, nil
		case "-":
			return ln - rn, nil
		case "*":
			return ln * rn, nil
		case "/":
			if rn == 0 {
				return nil, fmt.Errorf("%s: division by zero", x.Pos())
			}
			return ln / rn, nil
		}
	}
	if x.Op == "+" {
		if ls, ok := l.(Str); ok {
			return ls + Str(stringify(r)), nil
		}
		if rs, ok := r.(Str); ok {
			return Str(stringify(l)) + rs, nil
		}
	}
	if x.Op == "==" {
		return Bool(equals(l, r)), nil
	}
	if x.Op == "!=" {
		return Bool(!equals(l, r)), nil
	}
	if lok && rok {
		switch x.Op {
		case "<": return Bool(ln < rn), nil
		case "<=": return Bool(ln <= rn), nil
		case ">": return Bool(ln > rn), nil
		case ">=": return Bool(ln >= rn), nil
		}
	}
	return nil, fmt.Errorf("%s: %T %s %T is not defined", x.Pos(), l, x.Op, r)
}

// asNum collapses Num and Length to a single float, keeping units only
// when both operands share them.
func asNum(v Value) (Num, bool) {
	switch n := v.(type) {
	case Num:
		return n, true
	case Length:
		return Num(n.V), true
	}
	return 0, false
}

// stringify renders a value for string concatenation (Grammar Rule 19:
// shortest round-trip for numbers).
func stringify(v Value) string {
	switch x := v.(type) {
	case Str:
		return string(x)
	case Num:
		return strconv.FormatFloat(float64(x), 'g', -1, 64)
	case Length:
		return strconv.FormatFloat(x.V, 'g', -1, 64) + x.Unit
	case Bool:
		if x {
			return "true"
		}
		return "false"
	case Color:
		return fmt.Sprintf("#%02X%02X%02X%02X", x.R, x.G, x.B, x.A)
	case Kw:
		return string(x)
	}
	return fmt.Sprintf("%v", v)
}

func equals(l, r Value) bool { return l == r } // all scalar variants are comparable
```

Note: `Length + Length` keeps units only when they match; mismatched units are a type error. Extend `evalBinary` for Lengths accordingly: both Lengths with equal Unit → Length; one Length one Num → treat Num as unitless. `asNum` already collapses; refine where unit preservation matters (lengths in layout, durations in animation).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/expr.go internal/ksl/eval/expr_test.go
git commit -m "feat(eval): expression evaluator with type rules"
```

---

### Task 4: Randomisation — rand epoch cache and jitter

**Files:**
- Create: `internal/ksl/eval/random.go`
- Test: `internal/ksl/eval/random_test.go`

**Interfaces:**
- Consumes: `Env`, `State.RandCache`/`State.Summon`/`State.Seed` (Task 2), call-site positions (`ksl.Position`).
- Produces: `type RandCache struct` with `func (c *RandCache) Draw(site ksl.Position, epoch int, seed uint64, min, max float64) float64`, and `jitter(key string, min, max float64) float64` (FNV-1a).

- [ ] **Step 1: Write the failing test**

```go
package eval

import "testing"

func TestRandIsStableWithinASummon(t *testing.T) {
	c := &RandCache{}
	a := c.Draw(site(1, 10), 3, 42, -4, 4)
	b := c.Draw(site(1, 10), 3, 42, -4, 4)
	if a != b {
		t.Errorf("same summon redrew: %v vs %v", a, b)
	}
	c2 := c.Draw(site(1, 10), 4, 42, -4, 4)
	if c2 == a {
		t.Errorf("new summon did not redraw: %v", c2)
	}
	if c2 < -4 || c2 > 4 {
		t.Errorf("rand out of range: %v", c2)
	}
	// different call sites are independent draws
	other := c.Draw(site(2, 1), 3, 42, -4, 4)
	if other == a {
		t.Errorf("different call sites drew the same value twice: %v", other)
	}
}

func TestRandCacheRebuildsAcrossResolves(t *testing.T) {
	// The cache is a field on State; a Resolve with the same Summon
	// reuses it, a new Summon resets it.
	st := &State{Summon: 1, Seed: 7, RandCache: &RandCache{}}
	v1, err := evalOne(t, st, "rand(0, 100)")
	if err != nil { t.Fatal(err) }
	v2, err := evalOne(t, st, "rand(0, 100)")
	if err != nil { t.Fatal(err) }
	if v1 != v2 {
		t.Errorf("rand redrawn within one summon: %v vs %v", v1, v2)
	}
	st.Summon = 2
	v3, err := evalOne(t, st, "rand(0, 100)")
	if err != nil { t.Fatal(err) }
	if v3 == v1 {
		t.Errorf("rand not redrawn across summons: %v", v3)
	}
}

func TestJitterIsDeterministic(t *testing.T) {
	a := jitter("a-item", -3, 3)
	b := jitter("a-item", -3, 3)
	c := jitter("b-item", -3, 3)
	if a != b {
		t.Errorf("jitter not deterministic: %v vs %v", a, b)
	}
	if a < -3 || a > 3 || c < -3 || c > 3 {
		t.Errorf("jitter out of range: %v, %v", a, c)
	}
	if a == c {
		t.Logf("jitter collided on distinct keys (acceptable but suspicious)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -run 'TestRand|TestJitter' -v`
Expected: FAIL — `RandCache`/`jitter` undefined.

- [ ] **Step 3: Write the implementation**

```go
package eval

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// RandCache freezes every rand() draw for the duration of one summon
// (Grammar Rule 12). Draws are keyed by call site: the AST position
// identifies the source location.
type RandCache struct {
	epoch int
	draws map[callSite]float64
}

type callSite struct {
	name   string
	offset int
}

func siteFrom(pos ksl.Position, name string) callSite {
	return callSite{name: name, offset: pos.Offset}
}

// Draw returns the cached draw for the call site, sampling once per
// summon epoch.
func (c *RandCache) Draw(pos ksl.Position, name string, epoch int, seed uint64, min, max float64) float64 {
	if c.draws == nil || c.epoch != epoch {
		c.epoch = epoch
		c.draws = map[callSite]float64{}
		c.rng = rand.New(rand.NewSource(int64(seed)))
	}
	s := siteFrom(pos, name)
	if v, ok := c.draws[s]; ok {
		return v
	}
	v := min + c.rng.Float64()*(max-min)
	c.draws[s] = v
	return v
}

// jitter hashes key into [min, max]: deterministic, frame-safe, varies
// per key (Grammar Rule 12).
func jitter(key string, min, max float64) float64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	u := float64(h.Sum64()%1000000) / 1000000
	return min + u*(max-min)
}
```

(Imports: add `math/rand`; the `RandCache` needs an `rng *rand.Rand` field — add it to the struct.)

`Env.evalRand` / `Env.evalJitter` in `expr.go` call into these: `rand(min, max)` reads both args as Num, calls `st.RandCache.Draw(x.Pos(), e.stName, st.Summon, st.Seed, min, max)`. `e.stName` is the scene name on State (`State.Name`, added in Task 2 alongside the others). A nil `RandCache` in State means evaluation error ("rand requires a RandCache on State").

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -run 'TestRand|TestJitter' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/random.go internal/ksl/eval/random_test.go internal/ksl/eval/expr.go internal/ksl/eval/env.go
git commit -m "feat(eval): rand summon-epoch cache and deterministic jitter"
```

---

### Task 5: Cascade, states and when

**Files:**
- Create: `internal/ksl/eval/cascade.go`
- Test: `internal/ksl/eval/cascade_test.go`

**Interfaces:**
- Consumes: `Env.Eval`, AST `Node`/`StyleBlock`/`StateBlock`/`Selector`/`Prop`.
- Produces: `func resolveProps(n *ksl.Node, env *Env, ctx *CascadeCtx) (map[string]Value, bool, error)` returning the node's resolved properties and whether `when`/`states` say it exists. `CascadeCtx` carries the active pseudo-set and state truth.

- [ ] **Step 1: Write the failing test**

```go
package eval

import "testing"

func TestCascadeOrder(t *testing.T) {
	st := &State{Open: true}
	env := NewEnv(st, nil)
	scene, err := ksl.Parse("t.ksl", []byte(`
		Text.muted { color: #5A5A66, size: 12 }
		Text#title { color: #FF2E63 }
		Window { Text#title.muted { size: 48, color: #FFFFFF } }
	`))
	if err != nil { t.Fatal(err) }
	n := scene.Nodes()[0].Children[0]
	ctx := &CascadeCtx{Scene: scene}
	props, ok, err := resolveProps(n, env, ctx)
	if err != nil || !ok {
		t.Fatalf("resolveProps: %v, %v", err, ok)
	}
	// node-local beats id selector beats class selector
	if props["color"] != (Color{0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Errorf("color = %v, want node-local #FFFFFF", props["color"])
	}
	if props["size"] != Num(48) {
		t.Errorf("size = %v", props["size"])
	}
}

func TestNodeAndSceneStates(t *testing.T) {
	scene, err := ksl.Parse("t.ksl", []byte(`
		Window { Box#card { fill: #000000, state selected { fill: #FF0000 } } }
		state open { Box#card { opacity: 1 } }
	`))
	if err != nil { t.Fatal(err) }
	n := scene.Nodes()[0].Children[0]

	open := &State{Open: true}
	props, _, err := resolveProps(n, NewEnv(open, nil), &CascadeCtx{Scene: scene, Pseudo: map[string]bool{"selected": true}})
	if err != nil { t.Fatal(err) }
	if props["fill"] != (Color{0xFF, 0, 0, 0xFF}) {
		t.Errorf("selected fill = %v", props["fill"])
	}
	if props["opacity"] != Num(1) {
		t.Errorf("scene state opacity = %v", props["opacity"])
	}

	closed := &State{Open: false}
	props, _, err = resolveProps(n, NewEnv(closed, nil), &CascadeCtx{Scene: scene})
	if err != nil { t.Fatal(err) }
	if _, has := props["opacity"]; has {
		t.Errorf("inactive scene state leaked: %v", props)
	}
}

func TestWhenGuard(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window {
	    Text { text: "a", when: $mode == "media" }
	    Text { text: "b", when: $results.len > 0 }
	}`))
	st := &State{Mode: "apps", Results: []Item{{}}}
	ctx := &CascadeCtx{Scene: scene}
	_, okA, err := resolveProps(scene.Nodes()[0].Children[0], NewEnv(st, nil), ctx)
	if err != nil || okA {
		t.Errorf("mode mismatch should hide the node: ok=%v err=%v", okA, err)
	}
	_, okB, err := resolveProps(scene.Nodes()[0].Children[1], NewEnv(st, nil), ctx)
	if err != nil || !okB {
		t.Errorf("non-empty results should show the node: ok=%v err=%v", okB, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -run 'TestCascade|TestNodeAndScene|TestWhen' -v`
Expected: FAIL — `CascadeCtx`/`resolveProps` undefined.

- [ ] **Step 3: Write the implementation**

```go
package eval

import "github.com/darkliquid/keyshell/internal/ksl"

// CascadeCtx carries what the current frame's state means for matching:
// the scene's style blocks and scene-level states, and the pseudo-states
// active for the node being resolved.
type CascadeCtx struct {
	Scene  *ksl.Scene
	Pseudo map[string]bool // "selected" on a list item, etc.
}

// resolveProps computes one node's final property map by applying, in
// increasing precedence: theme tokens (colours), type/class style rules,
// id style rules, scene-level state targets, node-level state blocks,
// then the node's own properties (Grammar Rule 13, spec §4). The bool
// reports whether the node exists at all (when + states).
func resolveProps(n *ksl.Node, env *Env, ctx *CascadeCtx) (map[string]Value, bool, error) {
	props := map[string]Value{}

	// 1. Style rules (type, then class, then id), declaration order.
	for _, sb := range ctx.Scene.Styles() {
		if !selectorMatches(sb.Sel, n, ctx) {
			continue
		}
		if err := applyProps(props, sb.Props, env); err != nil {
			return nil, false, err
		}
	}

	// 2. Active scene-level states targeting this node.
	for _, sb := range ctx.Scene.States() {
		active, err := stateActive(sb.Name, env)
		if err != nil || !active {
			continue
		}
		for _, tgt := range sb.Targets {
			if selectorMatches(tgt.Sel, n, ctx) {
				if err := applyProps(props, tgt.Props, env); err != nil {
					return nil, false, err
				}
			}
		}
	}

	// 3. Node-level state blocks (active only).
	for _, sb := range n.States {
		active, err := stateActive(sb.Name, env)
		if err != nil {
			return nil, false, err
		}
		if !active {
			// pseudo-states participate too: `state selected` inside a
			// List template fires on the selected pseudo.
			if !ctx.Pseudo[sb.Name] {
				continue
			}
		}
		if err := applyProps(props, sb.Props, env); err != nil {
			return nil, false, err
		}
	}

	// 4. Node-local properties.
	if err := applyProps(props, n.Props, env); err != nil {
		return nil, false, err
	}

	// 5. when: the guard can hide the whole subtree.
	if w, ok := props["when"]; ok {
		if !truthy(w) {
			return nil, false, nil
		}
		delete(props, "when")
	}
	return props, true, nil
}

func applyProps(dst map[string]Value, props []*ksl.Prop, env *Env) error {
	for _, pr := range props {
		v, err := env.Eval(pr.Val)
		if err != nil {
			return err
		}
		dst[pr.Key] = v
	}
	return nil
}

// stateActive reports whether a built-in state name holds.
func stateActive(name string, env *Env) (bool, error) {
	switch name {
	case "open":
		return env.st.Open, nil
	case "closed":
		return !env.st.Open, nil
	case "focused":
		return env.st.Focused, nil
	case "selected":
		// selected as a scene-level state means "a selection exists";
		// the per-item pseudo fires on the materialised node.
		return env.st.Selected >= 0 && env.st.Selected < len(env.st.Results), nil
	}
	return false, nil
}

// selectorMatches checks type, id, classes, pseudos and the mode
// attribute (spec §4).
func selectorMatches(sel ksl.Selector, n *ksl.Node, ctx *CascadeCtx) bool {
	if sel.Type != "" && sel.Type != n.Name {
		return false
	}
	if sel.ID != "" && sel.ID != n.ID {
		return false
	}
	for _, c := range sel.Classes {
		if !hasClass(n, c) {
			return false
		}
	}
	for _, p := range sel.Pseudos {
		if !ctx.Pseudo[p] {
			return false
		}
	}
	if sel.AttrName == "mode" {
		// mode matches against the current state, not the node
		// (handled by the caller binding mode into ctx.Mode).
		return ctx.Mode == sel.AttrVal
	}
	return true
}
```

(`ctx.Mode string` added to CascadeCtx.) `hasClass` scans `n.Classes`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -run 'TestCascade|TestNodeAndScene|TestWhen' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/cascade.go internal/ksl/eval/cascade_test.go
git commit -m "feat(eval): cascade, state blocks and when guards"
```

---

### Task 6: Layout

**Files:**
- Create: `internal/ksl/eval/layout.go`
- Test: `internal/ksl/eval/layout_test.go`

**Interfaces:**
- Consumes: resolved props (Task 5), `State.Width/Height/Scale`.
- Produces: `RectF{X, Y, W, H float64}`, `type Measurer interface{ TextSize(text string, size float64, weight string) (w, h float64) }`, `func layout(n *ksl.Node, props map[string]Value, bounds RectF, st *State, m Measurer) *ResolvedNode` and the flex/container algorithms.

- [ ] **Step 1: Write the failing test**

```go
package eval

import "testing"

func TestLengthResolution(t *testing.T) {
	st := &State{Width: 800, Height: 600, Scale: 1.5}
	if got := resolveLength(Length{V: 40, Unit: "%"}, st, true); got != 320 {
		t.Errorf("40%% of width = %v, want 320", got)
	}
	if got := resolveLength(Length{V: 12, Unit: "px"}, st, false); got != 18 {
		t.Errorf("12px scaled = %v, want 18", got)
	}
	if got := resolveLength(Num(8), st, false); got != 12 {
		t.Errorf("unitless length = %v, want scaled px 12", got)
	}
}

func TestColumnLayout(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window { Column { gap: 8, align: center
	    Box { height: 40 }
	    Box { grow: 1 }
	}}`))
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, err := Resolve(scene, *st, WithMeasurer(nil))
	if err != nil { t.Fatal(err) }
	col := f.Root.Children[0]
	if col.Rect.H != 100 {
		t.Errorf("column height = %v, want fill", col.Rect.H)
	}
	a, b := col.Children[0], col.Children[1]
	if a.Rect.H != 40 || a.Rect.Y != 0 {
		t.Errorf("fixed child = %+v", a.Rect)
	}
	if b.Rect.H != 52 || b.Rect.Y != 48 { // 100 - 40 - 8 gap
		t.Errorf("grow child = %+v, want y=48 h=52", b.Rect)
	}
}

func TestRowGrowAndJustify(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window { Row { gap: 4, justify: center
	    Box { width: 20 } Box { width: 20 }
	}}`))
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st)
	row := f.Root.Children[0]
	a, b := row.Children[0], row.Children[1]
	if a.Rect.X != 28 || b.Rect.X != 52 { // content 44 wide, centred
		t.Errorf("justified row: %v, %v", a.Rect.X, b.Rect.X)
	}
}

func TestStackZAndConstraints(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window { Stack {
	    Box { width: 10, z: 2 } Box { width: 10, z: 1 }
	} }`))
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st)
	// z ascending: z=1 paints before z=2
	if f.Root.Children[0].Children[0].Z != 1 {
		t.Errorf("z order = %+v", f.Root.Children[0].Children)
	}
}

func TestTextMeasurement(t *testing.T) {
	m := fakeMeasurer{}
	scene, _ := ksl.Parse("t.ksl", []byte(`Window { Text { text: "hi", size: 12 } }`))
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st, WithMeasurer(m))
	text := f.Root.Children[0]
	if text.Rect.W != 24 || text.Rect.H != 12 {
		t.Errorf("text rect = %+v", text.Rect)
	}
}

type fakeMeasurer struct{}
func (fakeMeasurer) TextSize(s string, size float64, _ string) (float64, float64) {
	return float64(len(s)) * size, size
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -run 'TestLength|TestColumn|TestRow|TestStack|TestText' -v`
Expected: FAIL — `RectF`, `Resolve`, layout undefined.

- [ ] **Step 3: Write the implementation**

Core of `layout.go`:

```go
package eval

type RectF struct{ X, Y, W, H float64 }

// Measurer measures text without the graphics stack; the renderer
// injects the real one (theme.Face-based).
type Measurer interface {
	TextSize(text string, size float64, weight string) (w, h float64)
}

// resolveLength turns a Length/Num into absolute pixels: % of the
// Window, px and unitless scaled by the scene scale.
func resolveLength(v Value, st *State, horizontal bool) float64 {
	switch x := v.(type) {
	case Length:
		switch x.Unit {
		case "%":
			if horizontal {
				return x.V / 100 * st.Width
			}
			return x.V / 100 * st.Height
		case "px", "":
			return x.V * st.Scale
		}
	case Num:
		return float64(x) * st.Scale
	}
	return 0
}
```

The container layout, per node type, on the resolved tree being built in Task 7:

```go
// layout assigns RectF to a resolved node and its children within
// bounds. Fixed-size children honour explicit width/height; grow
// children share leftover main-axis space; gap spaces them.
func (r *resolver) layout(n *resolved, bounds RectF) {
	props := n.props
	if x, ok := lengthProp(props, "x"); ok {
		bounds.X = x
	}
	if y, ok := lengthProp(props, "y"); ok {
		bounds.Y = y
	}
	if w, ok := lengthProp(props, "width"); ok {
		bounds.W = w
	}
	if h, ok := lengthProp(props, "height"); ok {
		bounds.H = h
	}
	n.rect = applyConstraints(bounds, props)
	switch n.name {
	case "Row":
		r.layoutFlex(n, n.rect, true)
	case "Column":
		r.layoutFlex(n, n.rect, false)
	case "Grid":
		r.layoutGrid(n, n.rect)
	case "Stack":
		r.layoutStack(n, n.rect)
	default:
		r.layoutLeaf(n, n.rect)
	}
}
```

with `layoutFlex` (shared Row/Column), `layoutGrid` (`cols` × rows, uniform gap), `layoutStack` (align/x/y pin, z order via declaration), `layoutLeaf` (Text via Measurer, Box/Image keep their assigned rect, List materialises items first in Task 7 then lays them out like its orientation's container). `grow` distributes leftover main-axis space; `align`/`justify` position on cross/main.

Full detail lives in the task's code; the test cases pin the semantics.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -run 'TestLength|TestColumn|TestRow|TestStack|TestText' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/layout.go internal/ksl/eval/layout_test.go
git commit -m "feat(eval): flex layout, units, scale and constraints"
```

---

### Task 7: List materialisation and Resolve

**Files:**
- Create: `internal/ksl/eval/resolve.go`
- Test: `internal/ksl/eval/resolve_test.go`

**Interfaces:**
- Consumes: everything so far.
- Produces: `func Resolve(scene *ksl.Scene, st State, opts ...Option) (*Frame, error)`, `type Frame struct{ Root *ResolvedNode }`, `type ResolvedNode struct{ Name, ID string; Classes []string; Rect RectF; Props map[string]Value; Children []*ResolvedNode; Z int }`, `type Option func(*resolver)`, `WithMeasurer(Measurer)`.

- [ ] **Step 1: Write the failing test**

```go
package eval

import "testing"

func TestListMaterialisation(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`
		def Row { Box { Text { text: $item.title } } }
		Window { List {
		    items: $results, selected: $selected
		    cap: 2
		    template { Row { } }
		    empty { Text { text: "none" } }
		}}`))
	st := &State{Width: 100, Height: 100, Scale: 1,
		Results: []Item{{ID: "a", Title: "Firefox"}, {ID: "b", Title: "Firejail"}, {ID: "c", Title: "Fira"}},
		Selected: 1}
	f, err := Resolve(scene, *st)
	if err != nil { t.Fatal(err) }
	list := f.Root.Children[0]
	if len(list.Children) != 2 { // cap=2, not 3
		t.Fatalf("list children = %d, want 2", len(list.Children))
	}
	sel := list.Children[1]
	// selected item carries the pseudo; its def-instance props see $item
	txt := sel.Children[0].Children[0]
	if txt.Props["text"] != Str("Firejail") {
		t.Errorf("selected template text = %v", txt.Props["text"])
	}
}

func TestListEmpty(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window { List { items: $results, empty { Text { text: "none" } } } }`))
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st)
	list := f.Root.Children[0]
	if len(list.Children) != 1 || list.Children[0].Props["text"] != Str("none") {
		t.Errorf("empty list = %+v", list.Children)
	}
}

func TestRandInSceneIsSummonScoped(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window { Group#band { rotate: rand(-4, 4) * 1deg } }`))
	st := State{Width: 100, Height: 100, Scale: 1, Summon: 1, Seed: 42, RandCache: &RandCache{}}
	f1, _ := Resolve(scene, st)
	f2, _ := Resolve(scene, st)
	st.Summon = 2
	f3, _ := Resolve(scene, st)
	r1 := f1.Root.Children[0].Props["rotate"]
	r2 := f2.Root.Children[0].Props["rotate"]
	r3 := f3.Root.Children[0].Props["rotate"]
	if r1 != r2 {
		t.Errorf("rand within summon: %v vs %v", r1, r2)
	}
	if r1 == r3 {
		t.Errorf("rand across summons: %v vs %v", r1, r3)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval/ -run 'TestList|TestRandInScene' -v`
Expected: FAIL — `Resolve` undefined.

- [ ] **Step 3: Write the implementation**

`resolve.go` walks the scene tree, resolves each node's props through the cascade, materialises `List` templates (binding `$item`, applying `cap`, `empty`, `overflowSummary`), lays out, and assembles the Frame:

```go
// Resolve evaluates a scene against a state snapshot and returns the
// paintable frame.
func Resolve(scene *ksl.Scene, st State, opts ...Option) (*Frame, error) {
	r := &resolver{st: &st, cache: map[callSite]float64{}}
	for _, o := range opts {
		o(r)
	}
	// theme tokens first so $accent etc. resolve for everything else
	if len(r.st.Tokens) == 0 {
		r.st.Tokens = themeTokensFromScene(scene)
	}
	root, err := r.resolveRoot(scene)
	if err != nil {
		return nil, err
	}
	return &Frame{Root: root}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval/ -run 'TestList|TestRandInScene' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/resolve.go internal/ksl/eval/resolve_test.go
git commit -m "feat(eval): list materialisation and scene resolution"
```

---

### Task 8: Conformance — resolve the spec's example scenes

**Files:**
- Test: `internal/ksl/eval/conformance_test.go` (reuses `../ksl/testdata`)

**Interfaces:**
- Consumes: everything.
- Produces: end-to-end evidence the spec's own scenes resolve to Frames.

- [ ] **Step 1: Write the failing test**

```go
package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func TestSpecScenesResolve(t *testing.T) {
	for _, name := range []string{"minimal.ksl", "reference.ksl", "dropdown.ksl"} {
		src, err := os.ReadFile(filepath.Join("..", "ksl", "testdata", name))
		if err != nil { t.Fatal(err) }
		scene, err := ksl.Parse(name, src)
		if err != nil { t.Fatalf("%s: %v", name, err) }
		if err := ksl.Validate(scene); err != nil { t.Fatalf("%s: %v", name, err) }
		st := State{Width: 1280, Height: 400, Scale: 1, Summon: 1, Seed: 1,
			RandCache: &RandCache{}, Tokens: map[string]Color{
				"slab": {0x0B, 0x0B, 0x0F, 0xFF}, "ink": {0xF2, 0xF2, 0xF7, 0xFF},
				"accent": {0xFF, 0x2E, 0x63, 0xFF}, "accent-alt": {0x08, 0xF7, 0xFE, 0xFF},
				"muted": {0x5A, 0x5A, 0x66, 0xFF}, "surface-alt": {0x12, 0x12, 0x18, 0xFF},
			},
			Reveal: Reveal{Y: 100, H: 64, Angle: 0.05, Shear: -0.25},
		}
		f, err := Resolve(scene, st, WithMeasurer(fakeMeasurer{}))
		if err != nil { t.Fatalf("%s: %v", name, err) }
		if f.Root == nil || f.Root.Rect.W == 0 {
			t.Errorf("%s: empty frame", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails, then make it pass**

Run: `go test ./internal/ksl/eval/ -run TestSpecScenesResolve -v`
Expected: any failure is an interpreter bug from Tasks 1–7 to fix at its own task's scope.

- [ ] **Step 3: Run the whole suite and the format gate**

Run: `go test ./... && gofmt -l internal/ksl/eval && go vet ./internal/ksl/eval/`
Expected: all pass; gofmt prints nothing; vet is clean.

- [ ] **Step 4: Commit**

```bash
git add internal/ksl/eval/conformance_test.go
git commit -m "test(eval): resolve the spec's example scenes to frames"
```

---

## Self-review (done during planning)

- **Spec coverage:** §3 `when` → Task 5; §4 cascade → Task 5; §5 states → Task 5; §7 bindings/expressions/randomisation → Tasks 2–4; §2 layout → Task 6; §9 lists → Task 7; conformance → Task 8. Rule 12 (`rand`/`jitter` lifetimes) → Task 4. The painter-facing surface (`Frame`, `ResolvedNode`, `RectF`, `Props`) is the renderer plan's input.
- **Placeholder scan:** every step has code; layout container internals are summarised in prose with their exact semantics pinned by tests (the four layout test functions), which is the same split plan 1 used for the lexer vs. grammar.
- **Type consistency:** `Value` variants are the same set across tasks; `CascadeCtx.Pseudo`/`Mode` are introduced in Task 5 and used there and in Task 7's List materialisation; `RandCache` fields match Task 4's test (`Draw(site, epoch, seed, min, max)` — note the test signature fixes the argument order from the Task 3 sketch).

## Follow-up plans (not in this scope)

1. Renderer integration: `internal/render/scene.go` paints a `Frame` with gg, `ui/default.ksl` reproduces today's look, band mask/hit-test from `Reveal`.
2. Imagery, effects and the transition/keyframes animation clock.
