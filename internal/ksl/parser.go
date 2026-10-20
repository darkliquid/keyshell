package ksl

import "fmt"

// Loader loads an included scene fragment by name.
type Loader func(name string) ([]byte, error)

// Parse parses a scene with no include support.
func Parse(name string, src []byte) (*Scene, error) {
	return ParseWithLoader(name, src, nil)
}

// ParseWithLoader parses a scene, splicing includes in place (Grammar
// Rule 11). The loader resolves fragment names; a nil loader turns
// every include into an error.
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

type parser struct {
	name  string
	toks  []token
	i     int
	errs  ErrorList
	load  Loader
	stack []string // include chain for cycle detection
	scene *Scene   // top-level include splices append here
}

func (p *parser) peek() token { return p.toks[p.i] }

// here is the position of the current token.
func (p *parser) here() Position { return p.toks[p.i].pos }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

// errf records one diagnostic. Errors are capped so a pathological
// input cannot grow the list without bound; past the cap the parser
// stops consuming input.
func (p *parser) errf(pos Position, format string, args ...any) {
	if len(p.errs) >= maxParseErrors {
		return
	}
	p.errs = append(p.errs, Error{Name: p.name, Pos: pos, Msg: fmt.Sprintf(format, args...)})
	if len(p.errs) >= maxParseErrors {
		p.errs = append(p.errs, Error{Name: p.name, Pos: pos, Msg: "too many errors, stopping"})
		p.i = len(p.toks) - 1 // park on EOF
	}
}

const maxParseErrors = 100

// ensureProgress guarantees a loop iteration consumes at least one
// token: without it a token that is neither consumable nor a block
// closer (a stray `}` at top level, for instance) would loop forever,
// appending an error each time.
func (p *parser) ensureProgress(before int) {
	if p.i == before {
		t := p.peek()
		p.errf(t.pos, "skipping unexpected %s", p.describe(t))
		if t.kind != tEOF {
			p.next()
		}
	}
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
	nl := p.skipNL()
	if p.atPunct(",") {
		p.next()
		p.skipNL()
		return
	}
	t := p.peek()
	if t.kind == tEOF || p.atPunct("}") {
		return
	}
	if nl > 0 {
		return
	}
	p.errf(t.pos, "expected ',' or a newline between properties")
	p.sync()
}

func (p *parser) parseScene() *Scene {
	p.scene = &Scene{Name: p.name}
	p.skipNL()
	for p.peek().kind != tEOF {
		before := p.i
		if it := p.parseTopItem(); it != nil {
			p.scene.Items = append(p.scene.Items, it)
		}
		p.requireSep()
		p.ensureProgress(before)
	}
	return p.scene
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
// property named `state` (a colon follows).
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
		before := p.i
		switch {
		case t.kind == tIdent && t.text == "state" && p.peekIsStateBlock():
			b.states = append(b.states, p.parseNodeState())
		case t.kind == tIdent && t.text == "include":
			b.children = append(b.children, p.parseIncludeNodes()...)
		case t.kind == tIdent && p.peekIsProp():
			b.props = append(b.props, p.parseProp())
		case t.kind == tIdent:
			b.children = append(b.children, p.parseNode())
		default:
			p.errf(t.pos, "unexpected %s in block", p.describe(t))
			p.next()
		}
		p.requireSep()
		p.ensureProgress(before)
	}
}

func (p *parser) parseNode() *Node {
	start := p.here()
	name := p.next().text
	m, _ := p.parseMods() // markers here are the node's own id/classes
	b := p.parseBlock()
	return &Node{Pos: start, Name: name, ID: m.id, Classes: m.classes,
		Props: b.props, Children: b.children, States: b.states}
}

// parsePropsOnly parses `{ prop ... }`: node state blocks, style
// blocks, inline blocks and font blocks are property lists.
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
		val = p.parseAnimValue() // Grammar Rule 6: property-specific production
	} else {
		val = p.parseValue()
	}
	return &Prop{Pos: start, Key: key, Val: val}
}
