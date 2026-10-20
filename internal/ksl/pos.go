// Package ksl parses the Keyshell Scene Language (KSL) into a typed,
// validated scene AST. See the Grammar section of
// docs/superpowers/specs/2026-10-20-keyshell-ui-language-design.md,
// which is normative.
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
