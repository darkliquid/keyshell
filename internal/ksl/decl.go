package ksl

// Declaration parsers: theme, state (both forms), transition,
// keyframes, def, include, plus the property-specific animvalue
// production (Grammar Rules 3, 6, 11).

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

// parseNodeState parses `state NAME { props }` inside a node body:
// properties only (Grammar Rule 3).
func (p *parser) parseNodeState() *StateBlock {
	start := p.here()
	p.next() // state
	sb := &StateBlock{Pos: start, Name: p.expectIdent("state name")}
	sb.Props = p.parsePropsOnly("node state block")
	return sb
}

// parseSceneState parses a top-level `state NAME { selectorblock* }`.
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

// parseSelector parses `Type#id.class:pseudo[attr="val"]`, every part
// optional.
func (p *parser) parseSelector() Selector {
	start := p.here()
	var sel Selector
	sel.Pos = start
	if p.peek().kind == tIdent {
		sel.Type = p.next().text
	}
	return p.parseSelectorTail(sel)
}

// parseSelectorTail parses the markers after the optional type.
func (p *parser) parseSelectorTail(sel Selector) Selector {
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

// parseTiming parses `scalar easing?`, where easing is a keyword or a
// cubic-bezier function.
func (p *parser) parseTiming() (Scalar, Value) {
	t := p.peek()
	if t.kind != tScalar {
		p.errf(t.pos, "expected a duration, found %s", p.describe(t))
		return Scalar{}, nil
	}
	p.next()
	dur := Scalar{At: t.pos, N: t.num, Unit: t.unit}
	var easing Value
	if nt := p.peek(); nt.kind == tIdent {
		p.next()
		if nt.text == "cubic-bezier" && p.atPunct("(") {
			easing = p.parseCubicBezier()
		} else {
			easing = &Keyword{At: nt.pos, Name: nt.text}
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
			return &FuncCall{At: start, Name: "cubic-bezier", Args: args}
		}
		p.next()
		args = append(args, &Scalar{At: t.pos, N: t.num, Unit: t.unit})
		if i < 3 && !p.expectPunct(",") {
			return &FuncCall{At: start, Name: "cubic-bezier", Args: args}
		}
	}
	p.expectPunct(")")
	return &FuncCall{At: start, Name: "cubic-bezier", Args: args}
}

func (p *parser) parseKeyframes() Item {
	at := p.peek()
	p.next() // @
	if kw := p.expectIdent("keyframes"); kw != "keyframes" {
		p.errf(at.pos, "expected @keyframes, found @%s", kw)
	}
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

// parseTopInclude splices an included fragment's top-level items into
// the scene (Grammar Rule 11) and returns nil: nothing of the include
// itself survives into the resolved scene.
func (p *parser) parseTopInclude() Item {
	inc := p.parseIncludeToken()
	if inc == nil {
		return nil
	}
	p.scene.Items = append(p.scene.Items, p.resolveInclude(inc)...)
	return nil
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

// resolveInclude parses a fragment and returns its items, with cycle
// detection on the include chain.
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
		p.appendErrs(lerrs)
		return nil
	}
	sub := &parser{
		name:  inc.Path,
		toks:  toks,
		load:  p.load,
		stack: append(append([]string{}, p.stack...), inc.Path),
	}
	subScene := sub.parseScene()
	p.appendErrs(sub.errs)
	return subScene.Items
}

// appendErrs merges a sub-parser's diagnostics, respecting the cap.
func (p *parser) appendErrs(errs ErrorList) {
	for _, e := range errs {
		if len(p.errs) >= maxParseErrors {
			return
		}
		p.errs = append(p.errs, e)
	}
}

// parseAnimValue parses the `animation` property's own production
// (Grammar Rule 6): comma-separated `name [duration] [easing]
// [iteration] [direction]` steps.
func (p *parser) parseAnimValue() Value {
	start := p.here()
	step, ok := p.parseAnimStep()
	if !ok {
		return &Keyword{At: start}
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
	return &AnimVal{At: start, Steps: steps}
}

func (p *parser) parseAnimStep() (AnimStep, bool) {
	name := p.expectIdent("animation name")
	if name == "" {
		return AnimStep{}, false
	}
	s := AnimStep{Name: name}
	if p.peek().kind == tScalar && (p.peek().unit == "ms" || p.peek().unit == "s") {
		d := p.next()
		s.Dur = &Scalar{At: d.pos, N: d.num, Unit: d.unit}
	}
	if p.peek().kind == tIdent {
		e := p.next()
		if e.text == "cubic-bezier" && p.atPunct("(") {
			s.Easing = p.parseCubicBezier()
		} else {
			s.Easing = &Keyword{At: e.pos, Name: e.text}
		}
	}
	if p.peek().kind == tIdent && p.peek().text == "infinite" {
		tok := p.next()
		s.Iteration = &Keyword{At: tok.pos, Name: "infinite"}
	} else if p.peek().kind == tScalar && p.peek().unit == "" {
		tok := p.next()
		s.Iteration = &Scalar{At: tok.pos, N: tok.num}
	}
	if p.peek().kind == tIdent && (p.peek().text == "alternate" || p.peek().text == "reverse") {
		s.Direction = p.next().text
		if p.peek().kind == tIdent && (p.peek().text == "alternate" || p.peek().text == "reverse") {
			s.Direction += " " + p.next().text
		}
	}
	return s, true
}
