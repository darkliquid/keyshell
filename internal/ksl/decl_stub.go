package ksl

// Stubs completed in Task 5 (declarations).

func (p *parser) parseTheme() Item      { return p.unimplemented("theme") }
func (p *parser) parseSceneState() Item { return p.unimplemented("state") }

// parseNodeState is needed by parseBlock already; a props-only block.
func (p *parser) parseNodeState() *StateBlock {
	start := p.here()
	p.next() // state
	sb := &StateBlock{Pos: start, Name: p.expectIdent("state name")}
	sb.Props = p.parsePropsOnly("node state block")
	return sb
}

func (p *parser) parseTransition() Item { return p.unimplemented("transition") }
func (p *parser) parseKeyframes() Item  { return p.unimplemented("keyframes") }
func (p *parser) parseDef() Item        { return p.unimplemented("def") }
func (p *parser) parseTopInclude() Item { return p.unimplemented("include") }

func (p *parser) parseAnimValue() Value {
	t := p.peek()
	p.errf(t.pos, "animation values are not implemented yet")
	p.sync()
	return &Keyword{At: t.pos}
}

func (p *parser) unimplemented(what string) Item {
	t := p.peek()
	p.errf(t.pos, "%s declarations are not implemented yet", what)
	p.sync()
	return nil
}

// here is the position of the current token (formalised in Task 6).

// parseIncludeNodes is completed in Task 6 (include splicing).
func (p *parser) parseIncludeNodes() []*Node { return nil }
