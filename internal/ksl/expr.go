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
		return &InlineBlock{At: t.pos, Props: p.parsePropsOnly("inline block")}
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
	return &Ternary{At: cond.Pos(), Cond: cond, Then: thenV, Else: elseV}
}

// binaryLevels lists the operator levels, loosest first.
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
		l = &Binary{At: l.Pos(), Op: op, L: l, R: r}
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
		return &Unary{At: start, Op: op, X: p.parseUnary()}
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() Value {
	x := p.parsePrimary()
	if _, isRefPath := x.(*RefPath); isRefPath {
		// Grammar: a refpath is exactly `#id.prop`; it is not
		// postfixable, so `.b` in `#id.prop.a` is left as an error
		// for the separator check.
		return x
	}
	for p.atPunct(".") {
		dot := p.next()
		name := p.expectIdent("member name after '.'")
		if name == "" {
			return x
		}
		if p.atPunct("(") {
			x = &MethodCall{At: dot.pos, X: x, Name: name, Args: p.parseArgs()}
		} else {
			x = &Member{At: dot.pos, X: x, Name: name}
		}
	}
	return x
}

func (p *parser) parsePrimary() Value {
	t := p.peek()
	switch t.kind {
	case tScalar:
		p.next()
		return &Scalar{At: t.pos, N: t.num, Unit: t.unit}
	case tString:
		p.next()
		return &StringVal{At: t.pos, S: t.text}
	case tHash:
		p.next()
		return p.parseHashValue(t)
	case tRef:
		p.next()
		return &Ref{At: t.pos, Name: t.text}
	case tIdent:
		p.next()
		switch t.text {
		case "true":
			return &BoolVal{At: t.pos, B: true}
		case "false":
			return &BoolVal{At: t.pos, B: false}
		}
		if p.atPunct("(") {
			return &FuncCall{At: t.pos, Name: t.text, Args: p.parseArgs()}
		}
		return &Keyword{At: t.pos, Name: t.text}
	}
	if t.kind == tPunct && t.text == "(" {
		p.next()
		v := p.parseValue()
		p.expectPunct(")")
		return v
	}
	p.errf(t.pos, "expected a value, found %s", p.describe(t))
	p.next()
	return &Keyword{At: t.pos}
}

// parseHashValue resolves '#' by position: `#id.prop` is a cross-node
// reference, a 3/4/6/8-digit hex run is a colour (Grammar Rules 8, 20).
func (p *parser) parseHashValue(h token) Value {
	if p.atPunct(".") {
		p.next()
		prop := p.expectIdent("property name after '#id.'")
		return &RefPath{At: h.pos, ID: h.text, Prop: prop}
	}
	if isHexRun(h.text) {
		switch len(h.text) {
		case 3, 4, 6, 8:
			return &ColorVal{At: h.pos, Hex: h.text}
		}
	}
	p.errf(h.pos, "expected '.prop' after '#%s', or a 3/4/6/8-digit hex colour", h.text)
	return &Keyword{At: h.pos, Name: h.text}
}

func isHexRun(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
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

func (p *parser) parseArray() Value {
	start := p.here()
	p.next() // [
	var elems []Value
	for {
		p.skipNL()
		if p.peek().kind == tEOF {
			p.errf(p.peek().pos, "unterminated array")
			return &ArrayVal{At: start, Elems: elems}
		}
		if p.atPunct("]") {
			p.next()
			return &ArrayVal{At: start, Elems: elems}
		}
		elems = append(elems, p.parseValue())
		p.skipNL()
		if !p.atPunct(",") {
			continue // loop head re-checks for ']' or errors via EOF/other
		}
		p.next() // trailing comma allowed before ']'
	}
}
