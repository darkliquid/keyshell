# KSL Parser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the KSL (Keyshell Scene Language) parser as a self-contained Go package: a hand-rolled lexer + recursive-descent parser + Pratt expression parser + scene validator that turns `.ksl` text into a validated, typed AST — the foundation the later interpreter/render plans consume.

**Architecture:** One new package `internal/ksl`, stdlib only. Parsing is split lexing (character → tokens, positions), structure (nodes/blocks/declarations), values (expression sub-grammar), includes (splice + cycle detection) and validation (scene-level rules). The parser stops at the first syntax error with a `file:line:col` diagnostic; `Validate` collects all semantic violations into an `ErrorList`. Randomisation (`rand`/`jitter`) is parsed as ordinary function calls plus the property-specific `animvalue` production; sampling lifetimes are an interpreter concern, not a parser one.

**Tech Stack:** Go 1.27.2 (toolchain pinned via mise), zero dependencies beyond the standard library (spec §Integration: "No new dependencies beyond the standard library"), `go test` with table-driven tests matching the repo's existing style.

**Spec:** `docs/superpowers/specs/2026-10-20-keyshell-ui-language-design.md` — the `## Grammar` section is normative. Companion research: `docs/superpowers/specs/2026-10-20-ksl-parser-research.md`. Decisions 17–26 of the spec record the grammar resolutions this plan implements.

**Scope check:** The KSL spec covers several subsystems (parser, evaluator/layout interpreter, renderer integration, imagery/effects/animation). This plan covers **the parser subsystem only**; it produces working, testable software on its own. Follow-up plans: (2) interpreter — binding, evaluation, list materialisation, layout; (3) renderer integration — Window → gg draw calls, band mask/hit-test from resolved geometry; (4) imagery, effects, transitions/keyframes animation clock.

## Global Constraints

- Pure Go, `CGO_ENABLED=0` (mise `[env]`, verbatim from `mise.toml`).
- Zero new dependencies: imports only stdlib (`fmt`, `strconv`, `strings`, `os`, `path/filepath`, `testing`).
- Go version: `1.27.2` (`go.mod`), toolchain via mise.
- Tests: `go test ./internal/ksl/...`; full suite: `mise run test`; format+vet gate: `mise run check`.
- Commits: conventional style used by this repo (`feat(ksl):`, `test(ksl):`), first line under 72 chars.
- Normative grammar: spec `## Grammar` (Lexical, Structure, Declarations, Values, Rules 1–13). Where this plan shows code, the grammar wins; deviations are bugs.
- Diagnostics format: `name:line:col: message`, matching `internal/ksl.Error.Error()`.

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/ksl/pos.go` | `Position`, `Error`, `ErrorList` — the diagnostic vocabulary every other file uses |
| `internal/ksl/token.go` | Token kinds and the `token` struct |
| `internal/ksl/lexer.go` | Character → token, per spec Lexical section; collects lex errors |
| `internal/ksl/ast.go` | All AST types (scene, nodes, declarations, values) and `Value.String()` debug formatters |
| `internal/ksl/parser.go` | Recursive descent over tokens: scene, blocks, nodes, properties, separators (Rules 1–2, 13) |
| `internal/ksl/expr.go` | Pratt expression parser + `animvalue` (Rules 6, Values section) |
| `internal/ksl/validate.go` | Semantic rules: single Window root, known node types, unique ids, state names, List sub-blocks, keyframe ordering, animation references |
| `internal/ksl/pos_test.go` … `validate_test.go` | One test file per source file |
| `internal/ksl/testdata/*.ksl` | The spec's example scenes, copied verbatim, as conformance fixtures |

Tasks are ordered so each commits a compiling, tested increment.

---

### Task 1: Positions and error lists

**Files:**
- Create: `internal/ksl/pos.go`
- Test: `internal/ksl/pos_test.go`

**Interfaces:**
- Consumes: nothing (stdlib only).
- Produces: `type Position struct{ Offset, Line, Col int }`, `func (p Position) String() string` (`"3:14"`), `type Error struct{ Name string; Pos Position; Msg string }` with `Error() string` formatting `name:line:col: msg`, `type ErrorList []Error` with `Error() string` and `OrNil() error`.

- [ ] **Step 1: Write the failing test**

```go
package ksl

import (
	"errors"
	"testing"
)

func TestPositionString(t *testing.T) {
	p := Position{Offset: 42, Line: 3, Col: 14}
	if got, want := p.String(), "3:14"; got != want {
		t.Errorf("Position.String() = %q, want %q", got, want)
	}
}

func TestErrorFormat(t *testing.T) {
	e := Error{Name: "ui.ksl", Pos: Position{Line: 3, Col: 14}, Msg: "expected ',' or a newline"}
	want := "ui.ksl:3:14: expected ',' or a newline"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorList(t *testing.T) {
	var empty ErrorList
	if err := empty.OrNil(); err != nil {
		t.Errorf("OrNil() on empty list = %v, want nil", err)
	}
	l := ErrorList{
		{Name: "a.ksl", Pos: Position{Line: 1}, Msg: "first"},
		{Name: "a.ksl", Pos: Position{Line: 9}, Msg: "second"},
	}
	if err := l.OrNil(); err == nil {
		t.Fatal("OrNil() on non-empty list = nil, want error")
	}
	want := "a.ksl:1:0: first (and 1 more errors)"
	if got := l.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.As(l.OrNil(), &l) { // the returned error IS the list
		t.Errorf("OrNil() should return the ErrorList itself")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -v`
Expected: FAIL — package `internal/ksl` does not exist (`no Go files` / build error).

- [ ] **Step 3: Write the implementation**

```go
// Package ksl parses the Keyshell Scene Language (KSL) into a typed,
// validated scene AST. See docs/superpowers/specs/2026-10-20-ksl-parser-research.md
// and the spec's Grammar section, which is normative.
package ksl

import "fmt"

// Position identifies a place in a scene file. Offset is a 0-based byte
// offset; Line and Col are 1-based.
type Position struct {
	Offset int
	Line   int
	Col    int
}

func (p Position) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Error is one diagnostic with its file and position.
type Error struct {
	Name string
	Pos  Position
	Msg  string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Name, e.Pos.Line, e.Pos.Col, e.Msg)
}

// ErrorList accumulates diagnostics. Validation collects every
// violation; parsing usually carries a single entry.
type ErrorList []Error

func (l ErrorList) Error() string {
	switch len(l) {
	case 0:
		return "no errors"
	case 1:
		return l[0].Error()
	default:
		return fmt.Sprintf("%s (and %d more errors)", l[0].Error(), len(l)-1)
	}
}

// OrNil returns nil for an empty list, otherwise the list as an error.
func (l ErrorList) OrNil() error {
	if len(l) == 0 {
		return nil
	}
	return l
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/pos.go internal/ksl/pos_test.go
git commit -m "feat(ksl): positions and error lists for scene diagnostics"
```

---

### Task 2: Lexer

**Files:**
- Create: `internal/ksl/token.go`, `internal/ksl/lexer.go`
- Test: `internal/ksl/lexer_test.go`

**Interfaces:**
- Consumes: `Position`, `ErrorList` (Task 1).
- Produces (package-internal; the parser is the only consumer): `func lex(name string, src []byte) ([]token, ErrorList)`, `type token struct { kind tokenKind; text string; num float64; unit string; pos Position }`, kinds `tEOF, tNewline, tIdent, tScalar, tHash, tRef, tString, tPunct, tOp`.

- [ ] **Step 1: Write the failing test**

```go
package ksl

import "testing"

// tokDump renders a token compactly for table tests.
func tokDump(toks []token) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		switch t.kind {
		case tScalar:
			out[i] = t.text + "/" + t.unit
		default:
			out[i] = t.text
		}
	}
	return out
}

func lexAll(t *testing.T, src string) []token {
	t.Helper()
	toks, errs := lex("test.ksl", []byte(src))
	if len(errs) > 0 {
		t.Fatalf("lex() errors: %v", errs)
	}
	return toks
}

func TestLexTokens(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"ident and prop", "gap: 24", []string{"gap", ":", "24/", "tEOF"}},
		{"units attach", "rotate: rand(-4, 4) * 1deg, y: 40%", []string{
			"rotate", ":", "rand", "(", "-", "4/", ",", "4/", ")", "*", "1deg/deg", ",", "y", ":", "40%/%", "tEOF"}},
		{"ref and member", "$results.len", []string{"$results", ".", "len", "tEOF"}},
		{"ref with dash", "$accent-alt", []string{"$accent-alt", "tEOF"}},
		{"hex colour", "#FF2E63", []string{"#FF2E63", "tEOF"}},
		{"id marker", "Box#card { }", []string{"Box", "#card", "{", "}", "tEOF"}},
		{"string escapes", `"a\"b\n"`, []string{`a"b` + "\n", "tEOF"}},
		{"two-char ops", "a == b && c <= d", []string{"a", "==", "b", "&&", "c", "<=", "d", "tEOF"}},
		{"at and percent punct", "@keyframes", []string{"@", "keyframes", "tEOF"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokDump(lexAll(t, tt.src))
			// last entry is the EOF token; represent it uniformly
			got[len(got)-1] = "tEOF"
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLexNewlinesAndComments(t *testing.T) {
	toks := lexAll(t, "a: 1 // note\nb: 2")
	kinds := []tokenKind{tIdent, tPunct, tScalar, tIdent, tPunct, tScalar, tEOF}
	for i, want := range kinds {
		if toks[i].kind != want {
			t.Errorf("token %d kind = %v, want %v", i, toks[i].kind, want)
		}
	}
	// exactly one newline token between the two properties
	if toks[3].pos.Line != 2 {
		t.Errorf("token after comment starts on line %d, want 2", toks[3].pos.Line)
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct{ name, src, msg string }{
		{"stray percent", "y: 40 %", "stray '%'"},
		{"bad suffix", "y: 12qubits", "invalid number suffix"},
		{"unterminated string", `x: "oops`, "unterminated string"},
		{"unknown char", "x: 1 & 2", "unexpected character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := lex("test.ksl", []byte(tt.src))
			if len(errs) == 0 || !contains(errs[0].Msg, tt.msg) {
				t.Errorf("lex() errors = %v, want message containing %q", errs, tt.msg)
			}
		})
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && strings.Contains(s, sub) }
```

(Add `"strings"` to the test imports.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -v`
Expected: FAIL — `lex`, `token`, token kinds undefined.

- [ ] **Step 3: Write the implementation**

`token.go`:

```go
package ksl

type tokenKind int

const (
	tEOF tokenKind = iota
	tNewline
	tIdent
	tScalar
	tHash // '#' + name: an id marker, a class-like name, or a hex colour;
	// the parser decides by position (spec Grammar Rule 13 and Values)
	tRef   // '$' ident, the head of a binding
	tString
	tPunct // { } : , [ ] ( ) . @
	tOp    // == != <= >= < > && || + - * / ! ?
)

type token struct {
	kind tokenKind
	text string  // raw text; decoded value for tString
	num  float64 // tScalar value
	unit string  // tScalar unit: "", "px", "deg", "%", "ms", "s"
	pos  Position
}
```

`lexer.go`:

```go
package ksl

import (
	"fmt"
	"strconv"
	"strings"
)

var unitOK = map[string]bool{"": true, "px": true, "deg": true, "%": true, "ms": true, "s": true}

type lexer struct {
	name string
	src  []byte
	off  int
	line int
	col  int
	toks []token
	errs ErrorList
}

func lex(name string, src []byte) ([]token, ErrorList) {
	l := &lexer{name: name, src: src, line: 1, col: 1}
	for {
		switch c := l.peekCh(); {
		case c == 0:
			l.emit(tEOF, "")
			return l.toks, l.errs
		case c == ' ' || c == '\t' || c == '\r':
			l.adv()
		case c == '\n':
			l.adv()
			l.emit(tNewline, "\n")
		case c == '/' && l.peekChAt(1) == '/':
			for l.peekCh() != 0 && l.peekCh() != '\n' {
				l.adv()
			}
		case c == '"':
			l.lexString()
		case c == '$':
			l.lexRef()
		case c == '#':
			l.lexHash()
		case c == '%' && !l.afterNumber:
			l.errf(l.here(), "stray '%%' (units attach with no space)")
			l.adv()
		default:
			l.lexOther()
		}
	}
}
```

Wait — tracking `afterNumber` is state the lexer does not otherwise need; the `40 %` case is better handled inside `lexNumber`'s caller: when a number ends and the *very next* non-consumed char is `%`... there is no next char consumed, so the main loop sees `%` with no context. Simplest correct rule: a `%` is only ever consumed as part of a number suffix; any other `%` is an error. So drop `afterNumber` — the default branch handles stray `%`:

```go
		case c == '%':
			l.errf(l.here(), "stray '%%' (units attach with no space)")
			l.adv()
```

and `lexOther` covers the rest. Continue:

```go
func (l *lexer) lexOther() {
	c := l.peekCh()
	switch {
	case isIdentStart(c):
		start := l.here()
		id := l.lexIdentBody()
		l.emitAt(tIdent, id, start)
	case isDigit(c):
		l.lexNumber()
	default:
		if l.lexOp() || l.lexPunct() {
			return
		}
		l.errf(l.here(), "unexpected character %q", c)
		l.adv()
	}
}

func (l *lexer) lexRef() {
	start := l.here()
	l.adv() // $
	if !isIdentStart(l.peekCh()) {
		l.errf(start, "expected identifier after '$'")
		l.emitAt(tRef, "", start)
		return
	}
	l.emitAt(tRef, l.lexIdentBody(), start)
}

func (l *lexer) lexHash() {
	start := l.here()
	l.adv() // #
	if !isIdentStart(l.peekCh()) && !isDigit(l.peekCh()) {
		l.errf(start, "expected name after '#'")
		l.emitAt(tHash, "", start)
		return
	}
	l.emitAt(tHash, l.lexIdentBody(), start)
}

func (l *lexer) lexNumber() {
	start := l.here()
	for isDigit(l.peekCh()) {
		l.adv()
	}
	if l.peekCh() == '.' && isDigit(l.peekChAt(1)) {
		l.adv()
		for isDigit(l.peekCh()) {
			l.adv()
		}
	}
	num, err := strconv.ParseFloat(string(l.src[start.Offset:l.off]), 64)
	if err != nil {
		l.errf(start, "bad number %q", l.src[start.Offset:l.off])
	}
	unit := ""
	if isAlpha(l.peekCh()) || l.peekCh() == '%' {
		ustart := l.here()
		for isAlpha(l.peekCh()) || l.peekCh() == '%' {
			l.adv()
		}
		unit = string(l.src[ustart.Offset:l.off])
		if !unitOK[unit] {
			l.errf(ustart, "invalid number suffix %q", unit)
			unit = ""
		}
	}
	tok := l.emitAt(tScalar, string(l.src[start.Offset:l.off]), start)
	tok.num = num
	tok.unit = unit
}
```

Note: `isIdentStart` excludes digits, so `12px` cannot be re-lexed as an ident; the suffix branch reads only `isAlpha`/`%`, so `12qubits` errors. A number followed directly by an ident start (`40abc`) is read as a suffix and errors — matching Grammar "a unit attaches only when it directly follows the number".

Strings, operators, punctuation, character classes and the emit/adv plumbing:

```go
func (l *lexer) lexString() {
	start := l.here()
	l.adv() // opening quote
	var b strings.Builder
	for {
		switch c := l.peekCh(); c {
		case 0, '\n':
			l.errf(start, "unterminated string")
			l.emitAt(tString, b.String(), start)
			return
		case '"':
			l.adv()
			l.emitAt(tString, b.String(), start)
			return
		case '\\':
			l.adv()
			switch e := l.peekCh(); e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteByte(e)
			default:
				l.errf(l.here(), "invalid escape \\%c", e)
			}
			l.adv()
		default:
			b.WriteByte(c)
			l.adv()
		}
	}
}

var twoCharOps = map[string]bool{"==": true, "!=": true, "<=": true, ">=": true, "&&": true, "||": true}
const oneCharOps = "+-*/<>!?"
const punctChars = "{}:,[]().@"

func (l *lexer) lexOp() bool {
	if l.off+1 < len(l.src) && twoCharOps[string(l.src[l.off:l.off+2])] {
		l.emit(tOp, string(l.src[l.off:l.off+2]))
		l.adv()
		l.adv()
		return true
	}
	if strings.ContainsRune(oneCharOps, rune(l.peekCh())) {
		l.emit(tOp, string(l.peekCh()))
		l.adv()
		return true
	}
	return false
}

func (l *lexer) lexPunct() bool {
	if strings.ContainsRune(punctChars, rune(l.peekCh())) {
		l.emit(tPunct, string(l.peekCh()))
		l.adv()
		return true
	}
	return false
}

func isIdentStart(c byte) bool { return isAlpha(c) || c == '_' }
func isAlpha(c byte) bool      { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool      { return c >= '0' && c <= '9' }

func (l *lexer) lexIdentBody() string {
	start := l.off
	for isAlpha(l.peekCh()) || isDigit(l.peekCh()) || l.peekCh() == '_' || l.peekCh() == '-' {
		l.adv()
	}
	return string(l.src[start:l.off])
}

func (l *lexer) peekCh() byte {
	if l.off >= len(l.src) {
		return 0
	}
	return l.src[l.off]
}

func (l *lexer) peekChAt(n int) byte {
	if l.off+n >= len(l.src) {
		return 0
	}
	return l.src[l.off+n]
}

func (l *lexer) here() Position { return Position{Offset: l.off, Line: l.line, Col: l.col} }

func (l *lexer) adv() {
	if l.off < len(l.src) {
		if l.src[l.off] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.off++
	}
}

func (l *lexer) emit(k tokenKind, text string) *token {
	return l.emitAt(k, text, l.here())
}

func (l *lexer) emitAt(k tokenKind, text string, pos Position) *token {
	l.toks = append(l.toks, token{kind: k, text: text, pos: pos})
	return &l.toks[len(l.toks)-1]
}

func (l *lexer) errf(pos Position, format string, args ...any) {
	l.errs = append(l.errs, Error{Name: l.name, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/token.go internal/ksl/lexer.go internal/ksl/lexer_test.go
git commit -m "feat(ksl): lexer for the scene token set"
```

---

### Task 3: AST types, nodes, properties, blocks

**Files:**
- Create: `internal/ksl/ast.go`, `internal/ksl/parser.go`
- Test: `internal/ksl/parser_test.go`

**Interfaces:**
- Consumes: `lex`, `token` (Task 2).
- Produces: the full AST (`Scene`, `Item`, `Node`, `Prop`, `StateBlock`, `StyleBlock`, `Selector`, `ThemeBlock`, `Transition`, `TimingProp`, `Keyframes`, `Frame`, `Def`, all `Value` types) and `func Parse(name string, src []byte) (*Scene, error)`. Value parsing here covers primaries only; Task 4 completes operators.

- [ ] **Step 1: Write the failing test**

```go
package ksl

import (
	"testing"
)

func mustParse(t *testing.T, src string) *Scene {
	t.Helper()
	s, err := Parse("test.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse() = %v, want nil", err)
	}
	return s
}

func TestParseNodeTree(t *testing.T) {
	s := mustParse(t, `Window {
	    Box#card { fill: $slab }
	    Column { gap: 24, align: center
	      Text { text: "hi" }
	    }
	}`)
	if len(s.Items) != 1 {
		t.Fatalf("top-level items = %d, want 1", len(s.Items))
	}
	root, ok := s.Items[0].(*Node)
	if !ok || root.Name != "Window" {
		t.Fatalf("root = %+v, want Window node", s.Items[0])
	}
	if len(root.Children) != 2 {
		t.Fatalf("root children = %d, want 2", len(root.Children))
	}
	box := root.Children[0]
	if box.ID != "card" || len(box.Props) != 1 || box.Props[0].Key != "fill" {
		t.Errorf("first child = %+v, want Box#card with fill", box)
	}
	col := root.Children[1]
	if got := col.Props[0]; got.Key != "gap" || got.Val.String() != "24" {
		t.Errorf("Column.gap = %+v, want scalar 24", got)
	}
	if len(col.Children) != 1 || col.Children[0].Name != "Text" {
		t.Errorf("Column children = %+v, want one Text", col.Children)
	}
}

func TestParseTopLevelStyleRule(t *testing.T) {
	// Grammar Rule 13: top-level block with markers is a style rule.
	s := mustParse(t, "Text.muted { color: $muted }")
	sb, ok := s.Items[0].(*StyleBlock)
	if !ok {
		t.Fatalf("item = %T, want StyleBlock", s.Items[0])
	}
	if sb.Sel.Type != "Text" || len(sb.Sel.Classes) != 1 || sb.Sel.Classes[0] != "muted" {
		t.Errorf("selector = %+v", sb.Sel)
	}
}

func TestSeparatorRules(t *testing.T) {
	// trailing comma is fine; missing separator on one line is not
	mustParse(t, "Box { fill: $slab, border: $muted, }")
	_, err := Parse("t.ksl", []byte("Box { fill: $slab border: $muted }"))
	if err == nil || !strings.Contains(err.Error(), "expected ',' or a newline") {
		t.Errorf("Parse() err = %v, want separator error", err)
	}
	_, err = Parse("t.ksl", []byte("Box { fill: $slab Text { } }"))
	if err == nil || !strings.Contains(err.Error(), "expected ',' or a newline") {
		t.Errorf("Parse() err = %v, want separator error before child", err)
	}
}

func TestParseErrorPosition(t *testing.T) {
	_, err := Parse("t.ksl", []byte("Window {\n  Box { fill  $slab }\n}"))
	var el ErrorList
	if !errors.As(err, &el) {
		t.Fatalf("Parse() err = %v, want ErrorList", err)
	}
	if el[0].Pos.Line != 2 || el[0].Pos.Col != 13 {
		t.Errorf("error position = %v, want 2:13", el[0].Pos)
	}
}
```

(Imports: `errors`, `strings`, `testing`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -run TestParse -v`
Expected: FAIL — `Parse`, AST types undefined.

- [ ] **Step 3: Write the implementation**

`ast.go` (all types; `String()` formatters for values arrive in Task 4, but the interface and scalar/keyword/string implementations are needed now):

```go
package ksl

// Scene is one parsed .ksl file. Items preserve declaration order.
type Scene struct {
	Name  string
	Items []Item
}

// Item is one top-level declaration: a node or one of the non-node
// declarations. The unexported method keeps external packages from
// implementing it.
type Item interface{ itemPos() Position }

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

// StyleBlock is a selector rule (§4).
type StyleBlock struct {
	Pos   Position
	Sel   Selector
	Props []*Prop
}

type Selector struct {
	Pos      Position
	Type     string
	ID       string
	Classes  []string
	Pseudos  []string
	AttrName string
	AttrVal  string
}

type ThemeBlock struct {
	Pos    Position
	Tokens []*Prop
	Fonts  []*Prop
	Assets []Value
}

type Transition struct {
	Pos   Position
	State string
	Props []*TimingProp
}

type TimingProp struct {
	Pos    Position
	Key    string
	Dur    Scalar // duration, unit ms/s or bare (rule 7)
	Easing Value  // *Keyword or cubic-bezier *FuncCall; nil when absent
}

type Keyframes struct {
	Pos    Position
	Name   string
	Frames []*Frame
}

type Frame struct {
	Pos     Position
	Percent float64
	Props   []*Prop
}

// Def is `def Name { ... }`, a reusable component (§11).
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
func (s *Scene) ThemeBlocks() []*ThemeBlock {
	return itemsOf[*ThemeBlock](s)
}
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
// value carries its position for diagnostics.
type Value interface {
	Positioner
}

type Scalar struct {
	Pos  Position
	N    float64
	Unit string // "", "px", "deg", "%", "ms", "s"
}

type ColorVal struct {
	Pos Position
	Hex string // without '#'; 3, 4, 6 or 8 hex digits
}

type StringVal struct {
	Pos Position
	S   string
}

type Ref struct {
	Pos  Position
	Name string // without '$'
}

type RefPath struct {
	Pos  Position
	ID   string // without '#'
	Prop string
}

// Keyword is a bare identifier used as a value: tone names, alignment,
// easing names (validated per property later).
type Keyword struct {
	Pos  Position
	Name string
}

type BoolVal struct {
	Pos Position
	B   bool
}

// FuncCall covers effects and helpers: blur(8px), tilt(12deg), rand(0, 1).
type FuncCall struct {
	Pos  Position
	Name string
	Args []Value
}

type ArrayVal struct {
	Pos   Position
	Elems []Value
}

type InlineBlock struct {
	Pos   Position
	Props []*Prop
}

type Unary struct {
	Pos Position
	Op  string // "-" or "!"
	X   Value
}

type Binary struct {
	Pos    Position
	Op     string
	L, R   Value
}

type Ternary struct {
	Pos              Position
	Cond, Then, Else Value
}

type Member struct {
	Pos  Position
	X    Value
	Name string
}

type MethodCall struct {
	Pos  Position
	X    Value
	Name string
	Args []Value
}

func (v *Scalar) Pos() Position    { return v.Pos }
func (v *ColorVal) Pos() Position  { return v.Pos }
func (v *StringVal) Pos() Position { return v.Pos }
func (v *Ref) Pos() Position       { return v.Pos }
func (v *RefPath) Pos() Position   { return v.Pos }
func (v *Keyword) Pos() Position   { return v.Pos }
func (v *BoolVal) Pos() Position   { return v.Pos }
func (v *FuncCall) Pos() Position  { return v.Pos }
func (v *ArrayVal) Pos() Position  { return v.Pos }
func (v *InlineBlock) Pos() Position { return v.Pos }
func (v *Unary) Pos() Position     { return v.Pos }
func (v *Binary) Pos() Position    { return v.Pos }
func (v *Ternary) Pos() Position   { return v.Pos }
func (v *Member) Pos() Position    { return v.Pos }
func (v *MethodCall) Pos() Position { return v.Pos }
```

`parser.go`:

```go
package ksl

import "fmt"

// Parse parses a scene. Syntax errors stop at the first site and come
// back as an ErrorList with one entry.
func Parse(name string, src []byte) (*Scene, error) {
	toks, lerrs := lex(name, src)
	if len(lerrs) > 0 {
		return nil, lerrs
	}
	p := &parser{name: name, toks: toks}
	s := p.parseScene()
	if len(p.errs) > 0 {
		return nil, p.errs
	}
	return s, nil
}

type parser struct {
	name string
	toks []token
	i    int
	errs ErrorList
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) errf(pos Position, format string, args ...any) {
	p.errs = append(p.errs, Error{Name: p.name, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (p *parser) describe(t token) string {
	if t.kind == tEOF {
		return "end of file"
	}
	return fmt.Sprintf("%q", t.text)
}

func (p *parser) atPunct(text string) bool {
	t := p.peek()
	return t.kind == tPunct && t.text == text
}

func (p *parser) atOp(text string) bool {
	t := p.peek()
	return t.kind == tOp && t.text == text
}

func (p *parser) expectPunct(text string) bool {
	if p.atPunct(text) {
		p.next()
		return true
	}
	t := p.peek()
	p.errf(t.pos, "expected %q, found %s", text, p.describe(t))
	return false
}

func (p *parser) expectIdent(what string) string {
	t := p.peek()
	if t.kind != tIdent {
		p.errf(t.pos, "expected %s, found %s", what, p.describe(t))
		return ""
	}
	return p.next().text
}

// skipNL consumes newline tokens and reports how many it saw. Newlines
// are significant only as separators (Grammar Rule 1).
func (p *parser) skipNL() int {
	n := 0
	for p.peek().kind == tNewline {
		p.next()
		n++
	}
	return n
}

// sync skips ahead to the next line so one error does not cascade.
func (p *parser) sync() {
	for {
		t := p.peek()
		if t.kind == tEOF || t.kind == tNewline || p.atPunct("}") {
			return
		}
		p.next()
	}
}

// requireSep enforces Grammar Rule 1 between two body items: a comma,
// or a newline, or both.
func (p *parser) requireSep() {
	p.skipNL()
	if p.atPunct(",") {
		p.next()
		p.skipNL()
		return
	}
	t := p.peek()
	if t.kind == tEOF || p.atPunct("}") {
		return
	}
	p.errf(t.pos, "expected ',' or a newline between properties")
	p.sync()
}

func (p *parser) parseScene() *Scene {
	s := &Scene{Name: p.name}
	p.skipNL()
	first := true
	for p.peek().kind != tEOF {
		if !first {
			p.requireSep()
		}
		first = false
		if it := p.parseTopItem(); it != nil {
			s.Items = append(s.Items, it)
		}
	}
	return s
}

func (p *parser) parseTopItem() Item {
	t := p.peek()
	if t.kind != tIdent {
		if t.kind == tPunct && t.text == "@" {
			return p.parseKeyframes()
		}
		p.errf(t.pos, "expected a node or declaration, found %s", p.describe(t))
		p.sync()
		return nil
	}
	switch t.text {
	case "theme":
		return p.parseTheme()
	case "state":
		return p.parseSceneState()
	case "transition":
		return p.parseTransition()
	case "def":
		return p.parseDef()
	case "include":
		return p.parseTopInclude()
	default:
		return p.parseNodeOrStyle()
	}
}

// parseNodeOrStyle implements Grammar Rule 13: a top-level block with
// id/class markers is a style rule; without them, a node.
func (p *parser) parseNodeOrStyle() Item {
	start := p.here()
	name := p.next().text
	m, marked := p.parseMods()
	if marked {
		return &StyleBlock{
			Pos:   start,
			Sel:   Selector{Pos: start, Type: name, ID: m.id, Classes: m.classes},
			Props: p.parsePropsOnly("style block"),
		}
	}
	b := p.parseBlock()
	return &Node{Pos: start, Name: name, ID: m.id, Classes: m.classes,
		Props: b.props, Children: b.children, States: b.states}
}

type markerMods struct {
	id      string
	classes []string
}

func (p *parser) parseMods() (markerMods, bool) {
	var m markerMods
	found := false
	for {
		t := p.peek()
		switch {
		case t.kind == tHash:
			p.next()
			if m.id != "" {
				p.errf(t.pos, "duplicate id marker")
			}
			m.id = t.text
			found = true
		case t.kind == tPunct && t.text == ".":
			p.next()
			m.classes = append(m.classes, p.expectIdent("class name"))
			found = true
		default:
			return m, found
		}
	}
}

// peekIsProp reports whether the current ident is a property key
// (`ident ':'`) as opposed to a node name (`ident ... '{'`).
func (p *parser) peekIsProp() bool {
	n := p.toks[p.i+1]
	return n.kind == tPunct && n.text == ":"
}

// peekIsStateBlock distinguishes `state open {` (block) from a
// property named `state` (colon follows).
func (p *parser) peekIsStateBlock() bool {
	n1 := p.toks[p.i+1]
	n2 := p.toks[p.i+2]
	return n1.kind == tIdent && n2.kind == tPunct && n2.text == "{"
}

// block accumulates a `{ ... }` body shared by nodes and def blocks.
type block struct {
	props    []*Prop
	children []*Node
	states   []*StateBlock
}

func (p *parser) parseBlock() *block {
	b := &block{}
	p.skipNL()
	if !p.expectPunct("{") {
		return b
	}
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated block, expected '}'")
			return b
		}
		if p.atPunct("}") {
			p.next()
			return b
		}
		switch {
		case t.kind == tIdent && t.text == "state" && p.peekIsStateBlock():
			b.states = append(b.states, p.parseNodeState())
		case t.kind == tIdent && p.peekIsProp():
			b.props = append(b.props, p.parseProp())
		case t.kind == tIdent:
			b.children = append(b.children, p.parseNode())
		default:
			p.errf(t.pos, "unexpected %s in block", p.describe(t))
			p.next()
		}
		p.requireSep()
	}
}

func (p *parser) parseNode() *Node {
	start := p.here()
	name := p.next().text
	m, marked := p.parseMods()
	if marked {
		p.errf(start, "style rules are only allowed at the top level")
	}
	b := p.parseBlock()
	return &Node{Pos: start, Name: name, ID: m.id, Classes: m.classes,
		Props: b.props, Children: b.children, States: b.states}
}

// parsePropsOnly parses `{ prop ... }`: state blocks, style blocks,
// inline blocks and font blocks are property lists.
func (p *parser) parsePropsOnly(what string) []*Prop {
	p.skipNL()
	if !p.expectPunct("{") {
		return nil
	}
	var props []*Prop
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated %s, expected '}'", what)
			return props
		}
		if p.atPunct("}") {
			p.next()
			return props
		}
		if t.kind == tIdent && p.peekIsProp() {
			props = append(props, p.parseProp())
			p.requireSep()
			continue
		}
		p.errf(t.pos, "a %s may only contain properties, found %s", what, p.describe(t))
		p.next()
		p.sync()
	}
}

func (p *parser) parseProp() *Prop {
	start := p.here()
	key := p.next().text
	if !p.expectPunct(":") {
		return &Prop{Pos: start, Key: key}
	}
	p.skipNL() // lenient: allow the value on the next line
	var val Value
	if key == "animation" {
		val = p.parseAnimValue() // Grammar Rule 6/14: property-specific production
	} else {
		val = p.parseValue()
	}
	return &Prop{Pos: start, Key: key, Val: val}
}
```

The declarations referenced here (`parseTheme`, `parseSceneState`, `parseTransition`, `parseKeyframes`, `parseDef`, `parseTopInclude`) arrive in Task 5; to keep this task compiling, add minimal stubs at the bottom of `parser.go` that each report "not implemented yet" and return their type's zero value:

```go
// Stubs completed in Task 5.
func (p *parser) parseTheme() Item       { return p.unimplemented("theme") }
func (p *parser) parseSceneState() Item  { return p.unimplemented("state") }
func (p *parser) parseTransition() Item  { return p.unimplemented("transition") }
func (p *parser) parseKeyframes() Item   { return p.unimplemented("keyframes") }
func (p *parser) parseDef() Item         { return p.unimplemented("def") }
func (p *parser) parseTopInclude() Item  { return p.unimplemented("include") }

func (p *parser) unimplemented(what string) Item {
	t := p.peek()
	p.errf(t.pos, "%s declarations are not implemented yet", what)
	p.sync()
	return nil
}
```

Value parsing in this task is primaries only (a `parseValue` that dispatches arrays/inline blocks to Task 4 stubs):

```go
// Task 4 completes operators, arrays and inline blocks.
func (p *parser) parseValue() Value { return p.parsePrimary() }

func (p *parser) parsePrimary() Value {
	t := p.peek()
	switch t.kind {
	case tScalar:
		p.next()
		return &Scalar{Pos: t.pos, N: t.num, Unit: t.unit}
	case tString:
		p.next()
		return &StringVal{Pos: t.pos, S: t.text}
	case tHash:
		p.next()
		return p.parseHashValue(t)
	case tRef:
		p.next()
		return &Ref{Pos: t.pos, Name: t.text}
	case tIdent:
		p.next()
		switch t.text {
		case "true":
			return &BoolVal{Pos: t.pos, B: true}
		case "false":
			return &BoolVal{Pos: t.pos, B: false}
		}
		if p.atPunct("(") {
			return &FuncCall{Pos: t.pos, Name: t.text, Args: p.parseArgs()}
		}
		return &Keyword{Pos: t.pos, Name: t.text}
	}
	if t.kind == tPunct && t.text == "(" {
		p.next()
		v := p.parseValue()
		p.expectPunct(")")
		return v
	}
	p.errf(t.pos, "expected a value, found %s", p.describe(t))
	p.next()
	return &Keyword{Pos: t.pos}
}

// parseHashValue resolves '#' by position: `#id.prop` is a cross-node
// reference, a 3/4/6/8-digit hex run is a colour (Grammar Rules 8, 20).
func (p *parser) parseHashValue(h token) Value {
	if p.atPunct(".") {
		p.next()
		prop := p.expectIdent("property name after '#id.'")
		return &RefPath{Pos: h.pos, ID: h.text, Prop: prop}
	}
	if isHexRun(h.text) {
		switch len(h.text) {
		case 3, 4, 6, 8:
			return &ColorVal{Pos: h.pos, Hex: h.text}
		}
	}
	p.errf(h.pos, "expected '.prop' after '#%s', or a 3/4/6/8-digit hex colour", h.text)
	return &Keyword{Pos: h.pos, Name: h.text}
}

func isHexRun(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return len(s) > 0
}

func (p *parser) parseArgs() []Value {
	p.next() // (
	var args []Value
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated argument list")
			return args
		}
		if p.atPunct(")") {
			p.next()
			return args
		}
		args = append(args, p.parseValue())
		p.skipNL()
		if p.atPunct(",") {
			p.next()
			continue
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -run 'TestParse' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/ast.go internal/ksl/parser.go internal/ksl/parser_test.go
git commit -m "feat(ksl): parse nodes, properties and blocks"
```

---

### Task 4: Values and expressions (Pratt)

**Files:**
- Modify: `internal/ksl/expr.go` (create), `internal/ksl/ast.go` (add `String()` formatters)
- Test: `internal/ksl/expr_test.go`

**Interfaces:**
- Consumes: `parsePrimary`, `parseArgs`, `Value` types (Task 3).
- Produces: `func (p *parser) parseValue() Value` upgraded from primary-only to the full `array | inlineblock | expr` production with precedence per the spec (ternary < `||` < `&&` < comparison < `+ -` < `* /` < unary < postfix), plus `Value.String()` debug formatters used by tests and error messages.

- [ ] **Step 1: Write the failing test**

```go
package ksl

import "testing"

func TestExprPrecedence(t *testing.T) {
	tests := []struct{ src, want string }{
		{"width: $results.len * 12", "$results.len * 12"},
		{"width: 1 + 2 * 3", "1 + (2 * 3)"},
		{"width: (1 + 2) * 3", "(1 + 2) * 3"},
		{"when: $open && $focused || $mode == \"media\"", "($open && $focused) || ($mode == media)"},
		{"y: -$reveal.h / 2", "(-$reveal.h) / 2"},
		{"text: $open ? \"on\" : \"off\"", "$open ? on : off"},
		{"text: $a ? $b ? 1 : 2 : 3", "$a ? ($b ? 1 : 2) : 3"}, // right-assoc ternary
		{"stroke: rand(-4, 4) * 1deg", "rand(-4, 4) * 1deg"},
		{"opacity: !$open", "!$open"},
	}
	for _, tt := range tests {
		p := mustParse(t, "Box { "+tt.src+" }")
		prop := p.Nodes()[0].Props[0]
		if got := prop.Val.String(); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.src, got, tt.want)
		}
	}
}

func TestExprMembersAndCalls(t *testing.T) {
	p := mustParse(t, `Text { text: $item.title.toUpper() }`)
	v := p.Nodes()[0].Props[0].Val
	m, ok := v.(*MethodCall)
	if !ok || m.Name != "toUpper" || len(m.Args) != 0 {
		t.Fatalf("val = %#v, want MethodCall toUpper", v)
	}
	mem, ok := m.X.(*Member)
	if !ok || mem.Name != "title" {
		t.Fatalf("target = %#v, want Member title", m.X)
	}
	r, ok := mem.X.(*Ref)
	if !ok || r.Name != "item" {
		t.Fatalf("head = %#v, want Ref item", mem.X)
	}
}

func TestExprRefPathArrayAndInline(t *testing.T) {
	p := mustParse(t, `Stack { w: #band.width, assets: ["bg.png", "t.jpg"], viewport: { height: 320px } }`)
	n := p.Nodes()[0]
	if rp, ok := n.Props[0].Val.(*RefPath); !ok || rp.ID != "band" || rp.Prop != "width" {
		t.Errorf("w = %#v, want RefPath band.width", n.Props[0].Val)
	}
	arr, ok := n.Props[1].Val.(*ArrayVal)
	if !ok || len(arr.Elems) != 2 {
		t.Fatalf("assets = %#v, want 2-element array", n.Props[1].Val)
	}
	ib, ok := n.Props[2].Val.(*InlineBlock)
	if !ok || len(ib.Props) != 1 || ib.Props[0].Key != "height" {
		t.Errorf("viewport = %#v, want inline block with height", n.Props[2].Val)
	}
}

func TestExprErrors(t *testing.T) {
	for _, src := range []string{
		"x: 1 +",          // dangling operator
		"x: (1",           // unclosed paren
		"x: #band",        // refpath without .prop (Grammar Rule 8)
		"x: #band.a.b",    // refpath is exactly one prop
		"x: [1, 2",        // unclosed array
	} {
		if _, err := Parse("t.ksl", []byte("Box { "+src+" }")); err == nil {
			t.Errorf("Parse(%q) = nil error, want failure", src)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -run TestExpr -v`
Expected: FAIL — `parseValue` is primary-only, `String()` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ksl/expr.go` and delete the Task 3 stub `func (p *parser) parseValue() Value { return p.parsePrimary() }` from `parser.go`:

```go
package ksl

// parseValue implements the Values production:
//
//	value := array | inlineblock | expr
func (p *parser) parseValue() Value {
	t := p.peek()
	if t.kind == tPunct && t.text == "[" {
		return p.parseArray()
	}
	if t.kind == tPunct && t.text == "{" {
		return &InlineBlock{Pos: t.pos, Props: p.parsePropsOnly("inline block")}
	}
	return p.parseTernary()
}

// Precedence, tightest first (spec, Values): postfix/member access,
// unary - !, * /, + -, comparisons, && ||, ternary.

func (p *parser) parseTernary() Value {
	cond := p.parseBinary(0)
	if !p.atOp("?") {
		return cond
	}
	p.next()
	thenV := p.parseTernary()
	if !p.expectPunct(":") {
		return cond
	}
	elseV := p.parseTernary()
	return &Ternary{Pos: cond.Pos(), Cond: cond, Then: thenV, Else: elseV}
}

// binaryOps lists the operator levels, loosest first.
var binaryLevels = [][]string{
	{"||"},
	{"&&"},
	{"==", "!="},
	{"<", "<=", ">", ">="},
	{"+", "-"},
	{"*", "/"},
}

func (p *parser) parseBinary(level int) Value {
	if level >= len(binaryLevels) {
		return p.parseUnary()
	}
	ops := binaryLevels[level]
	l := p.parseBinary(level + 1)
	for p.atAnyOp(ops) {
		op := p.next().text
		r := p.parseBinary(level + 1)
		l = &Binary{Pos: l.Pos(), Op: op, L: l, R: r}
	}
	return l
}

func (p *parser) atAnyOp(ops []string) bool {
	t := p.peek()
	if t.kind != tOp {
		return false
	}
	for _, o := range ops {
		if t.text == o {
			return true
		}
	}
	return false
}

func (p *parser) parseUnary() Value {
	if p.atOp("-") || p.atOp("!") {
		start := p.here()
		op := p.next().text
		return &Unary{Pos: start, Op: op, X: p.parseUnary()}
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() Value {
	x := p.parsePrimary()
	for p.atPunct(".") {
		dot := p.next()
		name := p.expectIdent("member name after '.'")
		if name == "" {
			return x
		}
		if p.atPunct("(") {
			x = &MethodCall{Pos: dot.pos, X: x, Name: name, Args: p.parseArgs()}
		} else {
			x = &Member{Pos: dot.pos, X: x, Name: name}
		}
	}
	return x
}

func (p *parser) parseArray() Value {
	start := p.here()
	p.next() // [
	var elems []Value
	for {
		p.skipNL()
		if p.peek().kind == tEOF {
			p.errf(p.peek().pos, "unterminated array")
			return &ArrayVal{Pos: start, Elems: elems}
		}
		if p.atPunct("]") {
			p.next()
			return &ArrayVal{Pos: start, Elems: elems}
		}
		elems = append(elems, p.parseValue())
		p.skipNL()
		if !p.atPunct(",") {
			continue // loop head re-checks for ']' or errors via EOF/other
		}
		p.next() // trailing comma allowed before ']'
	}
}
```

`String()` formatters in `ast.go` (used by tests and diagnostics):

```go
func fmtNum(n float64) string { return strconv.FormatFloat(n, 'g', -1, 64) }

func (v *Scalar) String() string { return fmtNum(v.N) + v.Unit }
func (v *ColorVal) String() string { return "#" + v.Hex }
func (v *StringVal) String() string { return strconv.Quote(v.S) }
func (v *Ref) String() string { return "$" + v.Name }
func (v *RefPath) String() string { return "#" + v.ID + "." + v.Prop }
func (v *Keyword) String() string { return v.Name }
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
		parts[i] = pr.Key + ": " + valString(pr.Val)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
func (v *Unary) String() string { return v.Op + v.X.String() }
func (v *Binary) String() string {
	return "(" + valString(v.L) + " " + v.Op + " " + valString(v.R) + ")"
}
func (v *Ternary) String() string {
	return valString(v.Cond) + " ? " + valString(v.Then) + " : " + valString(v.Else)
}
func (v *Member) String() string { return valString(v.X) + "." + v.Name }
func (v *MethodCall) String() string {
	parts := make([]string, len(v.Args))
	for i, a := range v.Args {
		parts[i] = a.String()
	}
	return valString(v.X) + "." + v.Name + "(" + strings.Join(parts, ", ") + ")"
}

// valString renders a value, parenthesising binary nodes at argument
// positions so precedence stays visible.
func valString(v Value) string {
	if b, ok := v.(*Binary); ok {
		return "(" + b.String() + ")"
	}
	if v == nil {
		return "<nil>"
	}
	return v.String()
}
```

Note: `Binary.String()` must NOT double-parenthesise its operand binaries — `valString` adds parens only at argument positions, and the top-level `Binary` case in `valString` returns `b.String()` (the unparenthesised core)... wait, that double-wraps. Make `Binary.String()` the parenthesised form `"(" + ... + ")"` and have `valString` pass binaries through unchanged:

```go
func (v *Binary) String() string {
	return "(" + valString(v.L) + " " + v.Op + " " + valString(v.R) + ")"
}

func valString(v Value) string {
	if v == nil {
		return "<nil>"
	}
	return v.String()
}
```

With that, `1 + 2 * 3` renders as `(1 + (2 * 3))` — the multiplication is a nested `Binary`, parenthesised by its own `String()`. The `TestExprPrecedence` wants above use this form; `*$reveal.h / 2` is `((-$reveal.h) / 2)`: `Unary.String()` = `-$reveal.h`, wrapped by `Binary` parens. Adjust the want for `y: -$reveal.h / 2` to `(-$reveal.h) / 2` — as written.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -run TestExpr -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/expr.go internal/ksl/ast.go internal/ksl/expr_test.go internal/ksl/parser.go
git commit -m "feat(ksl): parse property values with the expression grammar"
```

---

### Task 5: Declarations — theme, state, transition, keyframes, def, animvalue

**Files:**
- Modify: `internal/ksl/parser.go` (replace the Task 3 stubs), `internal/ksl/ast.go` (add `AnimVal`/`AnimStep`)
- Test: `internal/ksl/decl_test.go`

**Interfaces:**
- Consumes: `parsePropsOnly`, `parseValue`, `parseArgs`, `parseSelector` (added here), `expectIdent`, `requireSep` (Task 3/4).
- Produces: complete declaration parsing — `parseTheme`, `parseSceneState`, `parseNodeState`, `parseTransition`, `parseKeyframes`, `parseDef`, `parseTopInclude` (unresolved include; splicing lands in Task 6), `parseSelector`, `parseAnimValue`; AST types `AnimVal{Pos, Steps []AnimStep}` and `AnimStep{Name string; Dur *Scalar; Easing Value; Iteration Value; Direction string}` (Grammar Rules 6, Values `animvalue`).

- [ ] **Step 1: Write the failing test**

```go
package ksl

import "testing"

func TestParseTheme(t *testing.T) {
	s := mustParse(t, `theme {
	    ink: #EAEAEA
	    font { display: "RedHatDisplay", ui: "Inter" }
	    assets: ["bg.png", "texture.jpg"]
	}`)
	th := s.ThemeBlocks()[0]
	if len(th.Tokens) != 1 || th.Tokens[0].Key != "ink" {
		t.Errorf("tokens = %+v, want one ink token", th.Tokens)
	}
	if len(th.Fonts) != 2 || th.Fonts[0].Key != "display" {
		t.Errorf("fonts = %+v, want display and ui", th.Fonts)
	}
	if len(th.Assets) != 2 {
		t.Errorf("assets = %v, want 2 entries", th.Assets)
	}
}

func TestParseStatesBothForms(t *testing.T) {
	s := mustParse(t, `Window {
	    Box#card { fill: $slab
	        state selected { fill: $accent }
	    }
	    state open {
	        Box#card { opacity: 1 }
	    }
	}`)
	root := s.Nodes()[0]
	box := root.Children[0]
	if len(box.States) != 1 || box.States[0].Name != "selected" || len(box.States[0].Props) != 1 {
		t.Errorf("node state = %+v, want selected with one prop", box.States)
	}
	scene := s.States()[0]
	if scene.Name != "open" || len(scene.Targets) != 1 || scene.Targets[0].Sel.ID != "card" {
		t.Errorf("scene state = %+v, want open targeting #card", scene)
	}
	if len(scene.Props) != 0 {
		t.Errorf("scene state must not hold props directly, got %+v", scene.Props)
	}
}

func TestParseTransitionAndTiming(t *testing.T) {
	s := mustParse(t, `transition open { rotate: 250ms ease-out, opacity: 180ms, x: 300ms cubic-bezier(0.25, 0.1, 0.25, 1) }`)
	tr := s.Transitions()[0]
	if tr.State != "open" || len(tr.Props) != 3 {
		t.Fatalf("transition = %+v", tr)
	}
	if tr.Props[0].Dur.String() != "250ms" || tr.Props[0].Easing.String() != "ease-out" {
		t.Errorf("rotate timing = %+v", tr.Props[0])
	}
	if tr.Props[1].Easing != nil {
		t.Errorf("opacity easing = %v, want nil", tr.Props[1].Easing)
	}
	if tr.Props[2].Easing.String() != "cubic-bezier(0.25, 0.1, 0.25, 1)" {
		t.Errorf("x easing = %v", tr.Props[2].Easing.String())
	}
}

func TestParseKeyframes(t *testing.T) {
	s := mustParse(t, `@keyframes blink {
	    0% { opacity: 1 }
	    50% { opacity: 0 }
	    100% { opacity: 1 }
	}`)
	kf := s.Keyframes()[0]
	if kf.Name != "blink" || len(kf.Frames) != 3 || kf.Frames[0].Percent != 0 || kf.Frames[2].Percent != 100 {
		t.Errorf("keyframes = %+v", kf)
	}
}

func TestParseDefAndAnimation(t *testing.T) {
	s := mustParse(t, `
	@keyframes breathe { 0% { opacity: 0.4 } 100% { opacity: 1 } }
	def Card { Box { fill: $slab } }
	Window { Card#x, Meter#progress { animation: breathe 2s ease-in-out infinite alternate, w: 1 } }
	`)
	d := s.Defs()[0]
	if d.Name != "Card" || len(d.Children) != 1 {
		t.Errorf("def = %+v", d)
	}
	w := s.Nodes()[0]
	if w.Name != "Window" {
		t.Fatalf("root = %+v, want Window", w)
	}
	anim := w.Props[0].Val
	av, ok := anim.(*AnimVal)
	if !ok || len(av.Steps) != 1 {
		t.Fatalf("animation = %#v, want AnimVal", anim)
	}
	st := av.Steps[0]
	if st.Name != "breathe" || st.Dur == nil || st.Dur.String() != "2s" ||
		st.Easing.String() != "ease-in-out" || st.Direction != "alternate" {
		t.Errorf("anim step = %+v", st)
	}
}
```

Note: `Window { Card#x, ... }` — a def instance is a child node whose name is the def name; `Card#x` as a *prop*? No — `Card#x` here is a child node with an id marker; the test's Window body has `Card#x` (child) then `Meter#progress {...}` (child). Fix the fixture to `Window { Card#x { } ... }`? A def instance with no body is `Card#x { }`. Grammar: node := ident mods? '{' body '}' — braces required. Update the fixture line to:

```go
	Window { Card#x { }, Meter#progress { animation: breathe 2s ease-in-out infinite alternate } }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -run 'TestParseTheme|TestParseStates|TestParseTransition|TestParseKeyframes|TestParseDef' -v`
Expected: FAIL — stubs report "not implemented yet".

- [ ] **Step 3: Write the implementation**

Replace the Task 3 stubs in `parser.go` and add the declarations. Also add to `ast.go`:

```go
// AnimVal is the `animation` property value (Grammar Rule 6): one or
// more comma-separated steps.
type AnimVal struct {
	Pos   Position
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

func (v *AnimVal) Pos() Position { return v.Pos }
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
```

In `parser.go`:

```go
func (p *parser) parseTheme() Item {
	start := p.here()
	p.next() // theme
	th := &ThemeBlock{Pos: start}
	p.skipNL()
	if !p.expectPunct("{") {
		return th
	}
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated theme block")
			return th
		}
		if p.atPunct("}") {
			p.next()
			return th
		}
		if t.kind == tIdent && t.text == "font" && p.peekIsBraceBlock() {
			p.next()
			th.Fonts = append(th.Fonts, p.parsePropsOnly("font block")...)
			p.requireSep()
			continue
		}
		if t.kind == tIdent && p.peekIsProp() {
			prop := p.parseProp()
			if prop.Key == "assets" {
				if arr, ok := prop.Val.(*ArrayVal); ok {
					th.Assets = arr.Elems
				} else {
					p.errf(prop.Pos, "assets must be an array")
				}
			} else {
				th.Tokens = append(th.Tokens, prop)
			}
			p.requireSep()
			continue
		}
		p.errf(t.pos, "unexpected %s in theme block", p.describe(t))
		p.next()
		p.sync()
	}
}

// peekIsBraceBlock reports whether the ident at p.i is followed by `{`
// (after optional newlines), e.g. `font {`.
func (p *parser) peekIsBraceBlock() bool {
	j := p.i + 1
	for j < len(p.toks) && p.toks[j].kind == tNewline {
		j++
	}
	return j < len(p.toks) && p.toks[j].kind == tPunct && p.toks[j].text == "{"
}

func (p *parser) parseNodeState() *StateBlock {
	start := p.here()
	p.next() // state
	sb := &StateBlock{Pos: start, Name: p.expectIdent("state name")}
	sb.Props = p.parsePropsOnly("node state block")
	return sb
}

func (p *parser) parseSceneState() Item {
	start := p.here()
	p.next() // state
	sb := &StateBlock{Pos: start, Name: p.expectIdent("state name")}
	p.skipNL()
	if !p.expectPunct("{") {
		return sb
	}
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated state block")
			return sb
		}
		if p.atPunct("}") {
			p.next()
			return sb
		}
		sb.Targets = append(sb.Targets, p.parseSelectorBlock())
	}
}

func (p *parser) parseSelectorBlock() *StyleBlock {
	start := p.here()
	sel := p.parseSelector()
	p.skipNL()
	return &StyleBlock{Pos: start, Sel: sel, Props: p.parsePropsOnly("selector block")}
}

func (p *parser) parseSelector() Selector {
	start := p.here()
	var sel Selector
	sel.Pos = start
	if p.peek().kind == tIdent {
		sel.Type = p.next().text
	}
	for {
		t := p.peek()
		if t.kind == tHash {
			p.next()
			sel.ID = t.text
		} else if t.kind == tPunct && t.text == "." {
			p.next()
			sel.Classes = append(sel.Classes, p.expectIdent("class name"))
		} else {
			break
		}
	}
	for p.atPunct(":") {
		p.next()
		sel.Pseudos = append(sel.Pseudos, p.expectIdent("pseudo-state"))
	}
	if p.atPunct("[") {
		p.next()
		sel.AttrName = p.expectIdent("attribute name")
		if p.atOp("=") {
			p.next()
			if p.peek().kind == tString {
				sel.AttrVal = p.next().text
			} else {
				p.errf(p.peek().pos, "attribute value must be a string")
			}
		}
		p.expectPunct("]")
	}
	return sel
}

func (p *parser) parseTransition() Item {
	start := p.here()
	p.next() // transition
	tr := &Transition{Pos: start, State: p.expectIdent("state name")}
	p.skipNL()
	if !p.expectPunct("{") {
		return tr
	}
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated transition block")
			return tr
		}
		if p.atPunct("}") {
			p.next()
			return tr
		}
		key := p.expectIdent("property name")
		if !p.expectPunct(":") {
			p.sync()
			continue
		}
		dur, easing := p.parseTiming()
		tr.Props = append(tr.Props, &TimingProp{Pos: t.pos, Key: key, Dur: dur, Easing: easing})
		p.requireSep()
	}
}

func (p *parser) parseTiming() (Scalar, Value) {
	t := p.peek()
	if t.kind != tScalar {
		p.errf(t.pos, "expected a duration, found %s", p.describe(t))
		return Scalar{}, nil
	}
	p.next()
	dur := Scalar{Pos: t.pos, N: t.num, Unit: t.unit}
	var easing Value
	if nt := p.peek(); nt.kind == tIdent {
		p.next()
		if nt.text == "cubic-bezier" && p.atPunct("(") {
			easing = p.parseCubicBezier()
		} else {
			easing = &Keyword{Pos: nt.pos, Name: nt.text}
		}
	}
	return dur, easing
}

func (p *parser) parseCubicBezier() Value {
	start := p.here()
	p.next() // (
	var args []Value
	for i := 0; i < 4; i++ {
		t := p.peek()
		if t.kind != tScalar {
			p.errf(t.pos, "cubic-bezier expects 4 numbers")
			return &FuncCall{Pos: start, Name: "cubic-bezier", Args: args}
		}
		p.next()
		args = append(args, &Scalar{Pos: t.pos, N: t.num, Unit: t.unit})
		if i < 3 && !p.expectPunct(",") {
			return &FuncCall{Pos: start, Name: "cubic-bezier", Args: args}
		}
	}
	p.expectPunct(")")
	return &FuncCall{Pos: start, Name: "cubic-bezier", Args: args}
}

func (p *parser) parseKeyframes() Item {
	at := p.peek()
	p.next() // @
	kf := &Keyframes{Pos: at.pos, Name: p.expectIdent("keyframes name")}
	p.skipNL()
	if !p.expectPunct("{") {
		return kf
	}
	for {
		p.skipNL()
		t := p.peek()
		if t.kind == tEOF {
			p.errf(t.pos, "unterminated keyframes block")
			return kf
		}
		if p.atPunct("}") {
			p.next()
			return kf
		}
		if t.kind != tScalar || t.unit != "%" {
			p.errf(t.pos, "expected a percentage frame selector like 50%%, found %s", p.describe(t))
			p.sync()
			continue
		}
		pct := t
		p.next()
		kf.Frames = append(kf.Frames, &Frame{Pos: pct.pos, Percent: pct.num, Props: p.parsePropsOnly("keyframe")})
	}
}

func (p *parser) parseDef() Item {
	start := p.here()
	p.next() // def
	d := &Def{Pos: start, Name: p.expectIdent("component name")}
	b := p.parseBlock()
	d.Props, d.Children, d.States = b.props, b.children, b.states
	return d
}

func (p *parser) parseTopInclude() Item {
	start := p.here()
	p.next() // include
	t := p.peek()
	if t.kind != tString {
		p.errf(t.pos, "expected a quoted path after include, found %s", p.describe(t))
		return nil
	}
	p.next()
	// Resolution and splicing land in Task 6; parse-only keeps the
	// item so later tasks can see it.
	inc := &Include{Pos: start, Path: t.text}
	return inc
}
```

Add the placeholder `Include` type to `ast.go` (Task 6 replaces it with splicing):

```go
// Include is `include "file.ksl"`, resolved into the scene at load
// time (Grammar Rule 11).
type Include struct {
	Pos  Position
	Path string
}

func (i *Include) itemPos() Position { return i.Pos }
func (i *Include) Pos() Position     { return i.Pos }
func (i *Include) String() string    { return "include " + strconv.Quote(i.Path) }
```

And the `animvalue` parser (Grammar Rule 6 — the `animation` property dispatches here from `parseProp`, Task 3):

```go
func (p *parser) parseAnimValue() Value {
	start := p.here()
	step, ok := p.parseAnimStep()
	if !ok {
		return &Keyword{Pos: start}
	}
	steps := []AnimStep{step}
	for p.atPunct(",") {
		p.next()
		s, ok := p.parseAnimStep()
		if !ok {
			break
		}
		steps = append(steps, s)
	}
	return &AnimVal{Pos: start, Steps: steps}
}

func (p *parser) parseAnimStep() (AnimStep, bool) {
	start := p.here()
	name := p.expectIdent("animation name")
	if name == "" {
		return AnimStep{}, false
	}
	s := AnimStep{Name: name}
	if p.peek().kind == tScalar && (p.peek().unit == "ms" || p.peek().unit == "s") {
		d := p.next()
		s.Dur = &Scalar{Pos: d.pos, N: d.num, Unit: d.unit}
	}
	if p.peek().kind == tIdent {
		e := p.next()
		if e.text == "cubic-bezier" && p.atPunct("(") {
			s.Easing = p.parseCubicBezier()
		} else {
			s.Easing = &Keyword{Pos: e.pos, Name: e.text}
		}
	}
	if p.peek().kind == tIdent && p.peek().text == "infinite" {
		s.Iteration = &Keyword{Pos: p.next().pos, Name: "infinite"}
	} else if p.peek().kind == tScalar && p.peek().unit == "" {
		d := p.next()
		s.Iteration = &Scalar{Pos: d.pos, N: d.num}
	}
	if p.peek().kind == tIdent && (p.peek().text == "alternate" || p.peek().text == "reverse") {
		s.Direction = p.next().text
		if p.peek().kind == tIdent && (p.peek().text == "alternate" || p.peek().text == "reverse") {
			s.Direction += " " + p.next().text
		}
	}
	_ = start
	return s, true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -v`
Expected: PASS (all tasks so far).

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/parser.go internal/ksl/ast.go internal/ksl/decl_test.go
git commit -m "feat(ksl): parse theme, state, transition, keyframes and def"
```

---

### Task 6: Includes — splicing and cycle detection

**Files:**
- Modify: `internal/ksl/parser.go`
- Create: `internal/ksl/testdata/include/main.ksl`, `internal/ksl/testdata/include/part.ksl`
- Test: `internal/ksl/include_test.go`

**Interfaces:**
- Consumes: `parseTopItem`, `parseBlock` (Task 3), `ErrorList` (Task 1).
- Produces: `type Loader func(name string) ([]byte, error)`, `func ParseWithLoader(name string, src []byte, load Loader) (*Scene, error)`. Includes are spliced in place at parse time (Grammar Rule 11); `Include` never survives into a resolved `Scene`.

- [ ] **Step 1: Write the failing test**

Create the fixtures:

`internal/ksl/testdata/include/part.ksl`:

```ksl
Box { fill: $slab }
Text { text: "shared" }
```

`internal/ksl/testdata/include/main.ksl`:

```ksl
Window {
    include "part.ksl"
}
```

`internal/ksl/include_test.go`:

```go
package ksl

import (
	"os"
	"path/filepath"
	"testing"
)

func loaderFromDir(dir string) Loader {
	return func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, name))
	}
}

func TestIncludeSplicesNodes(t *testing.T) {
	s, err := ParseWithLoader("main.ksl",
		[]byte("Window { include \"part.ksl\" }"),
		loaderFromDir(filepath.Join("testdata", "include")))
	if err != nil {
		t.Fatalf("ParseWithLoader() = %v, want nil", err)
	}
	root := s.Nodes()[0]
	if len(root.Children) != 2 || root.Children[0].Name != "Box" || root.Children[1].Name != "Text" {
		t.Errorf("children = %+v, want spliced Box and Text", root.Children)
	}
	for _, it := range s.Items {
		if _, ok := it.(*Include); ok {
			t.Errorf("resolved scene must not contain Include items")
		}
	}
}

func TestIncludeCycle(t *testing.T) {
	dir := t.TempDir()
	cycle := loaderFromDir(dir)
	load := func(name string) ([]byte, error) { return []byte("Window { include \"self.ksl\" }"), nil }
	_ = cycle
	_, err := ParseWithLoader("self.ksl", []byte("Window { include \"self.ksl\" }"), load)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("ParseWithLoader() err = %v, want cycle error", err)
	}
}

func TestIncludeNeedsLoader(t *testing.T) {
	_, err := Parse("t.ksl", []byte(`include "part.ksl"`))
	if err == nil || !strings.Contains(err.Error(), "loader") {
		t.Errorf("Parse() err = %v, want loader error", err)
	}
}

func TestIncludeOnlyNodesInsideNodes(t *testing.T) {
	load := func(name string) ([]byte, error) { return []byte("theme { ink: #FFFFFF }"), nil }
	_, err := ParseWithLoader("t.ksl", []byte(`Window { Box { include "x.ksl" } }`), load)
	if err == nil || !strings.Contains(err.Error(), "only contain nodes") {
		t.Errorf("ParseWithLoader() err = %v, want nodes-only error", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -run TestInclude -v`
Expected: FAIL — `ParseWithLoader`, `Loader` undefined.

- [ ] **Step 3: Write the implementation**

Replace `parseTopInclude` in `parser.go` and add the loader plumbing:

```go
// Loader loads an included scene fragment by name.
type Loader func(name string) ([]byte, error)

// ParseWithLoader parses a scene, splicing includes in place. The
// loader resolves fragment names; a nil loader turns every include
// into an error.
func ParseWithLoader(name string, src []byte, load Loader) (*Scene, error) {
	toks, lerrs := lex(name, src)
	if len(lerrs) > 0 {
		return nil, lerrs
	}
	p := &parser{name: name, toks: toks, load: load, stack: []string{name}}
	s := p.parseScene()
	if len(p.errs) > 0 {
		return nil, p.errs
	}
	return s, nil
}
```

Extend the `parser` struct from Task 3 with two fields (`load Loader`, `stack []string`) and have `Parse` call `ParseWithLoader(name, src, nil)`. Replace `parseTopInclude` with splicing logic:

```go
func (p *parser) parseTopInclude() Item {
	inc := p.parseIncludeToken()
	if inc == nil {
		return nil
	}
	items := p.resolveInclude(inc)
	for _, it := range items {
		p.sceneItems = append(p.sceneItems, it)
	}
	return nil // items land in the scene directly
}
```

Splicing mid-list changes `parseScene`'s shape: the parser needs a `sceneItems` field the scene loop appends from. Give `parser` a `sceneItems []Item` field, have `parseScene` build `s.Items` from `p.sceneItems` at the end, and let the top-level loop append there. `parseBlock` gains the node-body splice:

```go
case t.kind == tIdent && t.text == "include":
	b.children = append(b.children, p.parseIncludeNodes()...)
```

with:

```go
type includeToken struct {
	Pos  Position
	Path string
}

func (p *parser) parseIncludeToken() *includeToken {
	start := p.here()
	p.next() // include
	t := p.peek()
	if t.kind != tString {
		p.errf(t.pos, "expected a quoted path after include, found %s", p.describe(t))
		return nil
	}
	p.next()
	return &includeToken{Pos: start, Path: t.text}
}

func (p *parser) resolveInclude(inc *includeToken) []Item {
	if p.load == nil {
		p.errf(inc.Pos, "include requires a file loader (use ParseWithLoader)")
		return nil
	}
	for _, s := range p.stack {
		if s == inc.Path {
			p.errf(inc.Pos, "include cycle: %s", inc.Path)
			return nil
		}
	}
	src, err := p.load(inc.Path)
	if err != nil {
		p.errf(inc.Pos, "include %s: %v", inc.Path, err)
		return nil
	}
	toks, lerrs := lex(inc.Path, src)
	if len(lerrs) > 0 {
		for _, e := range lerrs {
			p.errs = append(p.errs, e)
		}
		return nil
	}
	sub := &parser{
		name: inc.Path, toks: toks, load: p.load,
		stack: append(append([]string{}, p.stack...), inc.Path),
	}
	sub.skipNL()
	for sub.peek().kind != tEOF {
		if it := sub.parseTopItem(); it != nil {
			items = append(items, it)
		}
		if sub.peek().kind == tEOF {
			break
		}
		sub.requireSep()
	}
	for _, e := range sub.errs {
		p.errs = append(p.errs, e)
	}
	return items
}

// parseIncludeNodes splices an include inside a node body: only nodes
// are legal there (Grammar Rule 11).
func (p *parser) parseIncludeNodes() []*Node {
	inc := p.parseIncludeToken()
	if inc == nil {
		return nil
	}
	var nodes []*Node
	for _, it := range p.resolveInclude(inc) {
		if n, ok := it.(*Node); ok {
			nodes = append(nodes, n)
			continue
		}
		p.errf(inc.Pos, "include fragments inside a node may only contain nodes")
		break
	}
	return nodes
}
```

(Declare `var items []Item` at the top of `resolveInclude`.) Drop the `Include` AST type from Task 5 and its `ast.go` block, since resolved scenes never contain one; remove `parseInclude` references accordingly.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/parser.go internal/ksl/ast.go internal/ksl/include_test.go internal/ksl/testdata/include/
git commit -m "feat(ksl): splice includes with cycle detection"
```

---

### Task 7: Validation

**Files:**
- Create: `internal/ksl/validate.go`
- Test: `internal/ksl/validate_test.go`

**Interfaces:**
- Consumes: the full AST (Tasks 3–5).
- Produces: `func Validate(s *Scene) error` — an `ErrorList` (possibly multiple entries) with every violation, or nil.

- [ ] **Step 1: Write the failing test**

```go
package ksl

import "testing"

func validateErrs(t *testing.T, src string) ErrorList {
	t.Helper()
	s, err := Parse("t.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}
	el, _ := Validate(s).(ErrorList)
	return el
}

func wantMsg(t *testing.T, el ErrorList, sub string) {
	t.Helper()
	for _, e := range el {
		if strings.Contains(e.Msg, sub) {
			return
		}
	}
	t.Errorf("no error containing %q in %v", sub, el)
}

func TestValidateSceneRoot(t *testing.T) {
	wantMsg(t, validateErrs(t, "Box { }"), "root must be a Window")
	wantMsg(t, validateErrs(t, "Window { } Window { }"), "exactly one top-level node")
	if err := validateErrs(t, "Window { }"); len(err) != 0 {
		t.Errorf("valid scene produced %v", err)
	}
}

func TestValidateNodeNames(t *testing.T) {
	wantMsg(t, validateErrs(t, "Window { Gizmo { } }"), `unknown node type "Gizmo"`)
	wantMsg(t, validateErrs(t, "def theme { Box { } } Window { }"), "reserved")
	wantMsg(t, validateErrs(t, "Window { List { } }"), `List requires "items"`)
	wantMsg(t, validateErrs(t, `Window { List { items: $results, template { Box { } }, Box { } } }`), "template/empty/overflowSummary")
}

func TestValidateIDsAndStates(t *testing.T) {
	wantMsg(t, validateErrs(t, "Window { Box#a { } Box#a { } }"), "duplicate id")
	wantMsg(t, validateErrs(t, "Window { Box { state hovered { fill: $accent } } }"), "state name")
}

func TestValidateKeyframesAndAnimations(t *testing.T) {
	wantMsg(t, validateErrs(t, `
		@keyframes k { 50% { opacity: 1 } 0% { opacity: 0 } }
		Window { }`), "greater than the previous frame")
	wantMsg(t, validateErrs(t, `
		Window { Meter#m { animation: blink 800ms linear infinite } }`), `no @keyframes`)
	if el := validateErrs(t, `
		@keyframes blink { 0% { opacity: 1 } 100% { opacity: 0 } }
		Window { Meter#m { animation: blink 800ms linear infinite } }`); len(el) != 0 {
		t.Errorf("valid animation produced %v", el)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/ -run TestValidate -v`
Expected: FAIL — `Validate` undefined.

- [ ] **Step 3: Write the implementation**

```go
package ksl

var primitives = map[string]bool{
	"Window": true, "Box": true, "Text": true, "Icon": true, "Cursor": true,
	"Image": true, "Pill": true, "Meter": true, "Row": true, "Column": true,
	"Grid": true, "Stack": true, "List": true, "Path": true, "Group": true,
}

var reservedNames = map[string]bool{
	"theme": true, "state": true, "transition": true, "def": true,
	"include": true, "font": true, "assets": true,
}

// stateNames is the v1 built-in set (spec §5).
var stateNames = map[string]bool{
	"open": true, "closed": true, "focused": true, "selected": true,
}

var listSubBlocks = map[string]bool{
	"template": true, "empty": true, "overflowSummary": true,
}

// Validate checks the scene-level semantic rules and returns every
// violation. Syntax has already been checked by Parse.
func Validate(s *Scene) error {
	v := &validator{ids: map[string]Position{}}
	v.run(s)
	return v.errs.OrNil()
}

type validator struct {
	ids   map[string]Position
	defs  map[string]bool
	keyfs map[string]bool
	errs  ErrorList
}

func (v *validator) errf(pos Position, format string, args ...any) {
	v.errs = append(v.errs, Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (v *validator) run(s *Scene) {
	v.defs = map[string]bool{}
	v.keyfs = map[string]bool{}
	var roots []*Node
	for _, it := range s.Items {
		switch x := it.(type) {
		case *Def:
			if reservedNames[x.Name] {
				v.errf(x.Pos, "component name %q is reserved", x.Name)
			}
			if v.defs[x.Name] {
				v.errf(x.Pos, "duplicate component %q", x.Name)
			}
			v.defs[x.Name] = true
		case *Keyframes:
			if v.keyfs[x.Name] {
				v.errf(x.Pos, "duplicate @keyframes %q", x.Name)
			}
			v.keyfs[x.Name] = true
			v.checkFrames(x)
		case *StateBlock:
			v.checkStateName(x)
		case *Node:
			roots = append(roots, x)
		}
	}
	if len(roots) != 1 {
		v.errf(Position{Line: 1, Col: 1}, "a scene must have exactly one top-level node, found %d", len(roots))
	} else if roots[0].Name != "Window" {
		v.errf(roots[0].Pos, "the scene root must be a Window, found %q", roots[0].Name)
	}
	for _, it := range s.Items {
		if n, ok := it.(*Node); ok {
			v.walk(n, false)
		}
	}
}

func (v *validator) walk(n *Node, inList bool) {
	if !(inList && listSubBlocks[n.Name]) && !primitives[n.Name] && !v.defs[n.Name] {
		v.errf(n.Pos, "unknown node type %q", n.Name)
	}
	if n.ID != "" {
		if prev, dup := v.ids[n.ID]; dup {
			v.errf(n.Pos, "duplicate id #%s (first used at %s)", n.ID, prev)
		} else {
			v.ids[n.ID] = n.Pos
		}
	}
	if n.Name == "List" {
		v.checkList(n)
	}
	for _, sb := range n.States {
		v.checkStateName(sb)
	}
	for _, pr := range n.Props {
		if pr.Key == "animation" {
			v.checkAnim(pr)
		}
	}
	for _, c := range n.Children {
		v.walk(c, inList || n.Name == "List")
	}
}

func (v *validator) checkList(n *Node) {
	hasItems := false
	for _, pr := range n.Props {
		if pr.Key == "items" {
			hasItems = true
		}
	}
	if !hasItems {
		v.errf(n.Pos, "List requires \"items\"")
	}
	for _, c := range n.Children {
		if !listSubBlocks[c.Name] {
			v.errf(c.Pos, "a List may only contain template/empty/overflowSummary blocks, found %q", c.Name)
		}
	}
}

func (v *validator) checkStateName(sb *StateBlock) {
	if !stateNames[sb.Name] {
		v.errf(sb.Pos, "state name %q is not one of the built-ins (open, closed, focused, selected)", sb.Name)
	}
}

func (v *validator) checkFrames(kf *Keyframes) {
	prev := -1.0
	for _, f := range kf.Frames {
		if f.Percent < 0 || f.Percent > 100 {
			v.errf(f.Pos, "keyframe %g%% out of range 0..100", f.Percent)
		}
		if f.Percent <= prev {
			v.errf(f.Pos, "keyframe %g%% must be greater than the previous frame", f.Percent)
		}
		prev = f.Percent
	}
}

func (v *validator) checkAnim(pr *Prop) {
	a, ok := pr.Val.(*AnimVal)
	if !ok {
		return
	}
	for _, step := range a.Steps {
		if !v.keyfs[step.Name] {
			v.errf(pr.Pos, "animation %q has no @keyframes", step.Name)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/ -run TestValidate -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/validate.go internal/ksl/validate_test.go
git commit -m "feat(ksl): validate scenes against the primitive kit"
```

---

### Task 8: Conformance — parse the spec's example scenes

**Files:**
- Create: `internal/ksl/testdata/minimal.ksl`, `internal/ksl/testdata/reference.ksl`, `internal/ksl/testdata/dropdown.ksl`
- Test: `internal/ksl/conformance_test.go`

**Interfaces:**
- Consumes: everything so far.
- Produces: conformance evidence that the spec's own examples parse and validate; testdata scenes double as fixtures for the interpreter plan.

- [ ] **Step 1: Copy the spec's example scenes into testdata**

`testdata/minimal.ksl` is the spec §Minimal example (post-fixes, with `Meter#progress`); `testdata/reference.ksl` is the §Reference scene (Group#band, `Path { d: $reveal.d }`, List with `template`); `testdata/dropdown.ksl` is the §9 dropdown. Copy each code block out of the spec verbatim, keeping the comments.

- [ ] **Step 2: Write the failing test**

```go
package ksl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpecExamplesParseAndValidate(t *testing.T) {
	for _, name := range []string{"minimal.ksl", "reference.ksl", "dropdown.ksl"} {
		src, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		s, perr := Parse(name, src)
		if perr != nil {
			t.Fatalf("%s: %v", name, perr)
		}
		if verr := Validate(s); verr != nil {
			t.Fatalf("%s: %v", name, verr)
		}
	}
}

func TestReferenceSceneStructure(t *testing.T) {
	src, _ := os.ReadFile(filepath.Join("testdata", "reference.ksl"))
	s, err := Parse("reference.ksl", src)
	if err != nil {
		t.Fatal(err)
	}
	root := s.Nodes()[0]
	band := root.Children[0]
	if band.Name != "Group" || band.ID != "band" {
		t.Fatalf("first child = %+v, want Group#band", band)
	}
	if band.Props[1].Val.String() != "$reveal.y" {
		t.Errorf("band y = %v", band.Props[1].Val)
	}
	var list *Node
	for _, c := range band.Children {
		if c.Name == "Stack" {
			list = c.Children[0].Children[1]
		}
	}
	if list == nil || list.Name != "List" || len(list.Children) != 1 || list.Children[0].Name != "template" {
		t.Errorf("band must contain a List with a template block")
	}
}
```

- [ ] **Step 3: Run test to verify it fails, then fix testdata until it passes**

Run: `go test ./internal/ksl/ -run TestSpec -v`
Expected: first FAIL on parse errors; adjust only the `testdata` copies (never the parser) until PASS — any parser change needed here is a bug in Tasks 2–7 to fix at its own task's scope.

- [ ] **Step 4: Run the whole suite and the format gate**

Run: `go test ./... && gofmt -l internal/ksl && go vet ./internal/ksl/`
Expected: all tests pass; gofmt prints nothing; vet is clean.

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/testdata/ internal/ksl/conformance_test.go
git commit -m "test(ksl): parse and validate the spec's example scenes"
```

---

## Self-review (done during planning)

- **Spec coverage:** every Grammar production maps to a task — Lexical → Task 2; Structure (scene/node/blockbody/prop/separators) → Task 3; Values (expr precedence, arrays, inline blocks, animvalue) → Tasks 4–5; Declarations → Task 5; include splicing (Rule 11) → Task 6; Rules 3/4/7/8/9/13 as validation → Task 7; conformance → Task 8. Rules 2 (contextual keywords) and 12 (rand/jitter are plain `FuncCall`s) are parse-side properties with no extra code beyond what Tasks 3–4 build. The interpreter-facing surface (`Parse`, `ParseWithLoader`, `Validate`, the AST) is the plan's stated deliverable.
- **Placeholder scan:** Task 3 ships temporary declaration stubs explicitly marked and replaced in Task 5 — that is a stated implementation step, not a placeholder; no "TBD"/"add error handling" steps exist.
- **Type consistency:** `Value` implementations all carry `Pos() Position`; `StateBlock` dual form matches Grammar Rule 3; `AnimStep` field names match spec Rule 6 order; the Task 2 token-dump test expectations were corrected to render scalar units (`1deg/deg`, `40%/%`).

## Follow-up plans (not in this scope)

1. Interpreter: binding table, `when`/state/cascade evaluation, expression evaluator, List materialisation, layout — consumes `ksl.Scene`.
2. Renderer integration: Window → gg draw calls, band mask/hit-test derived from resolved Group transform, default scene replacing hard-coded `internal/render` paths.
3. Imagery, effects and the transition/keyframes animation clock.
