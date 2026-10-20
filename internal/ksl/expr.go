package ksl

// parseValue implements the Values production:
//
//	value := array | inlineblock | expr
//
// Task 4 completes operators, arrays and inline blocks; this file
// starts with primaries only.
func (p *parser) parseValue() Value { return p.parsePrimary() }

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
