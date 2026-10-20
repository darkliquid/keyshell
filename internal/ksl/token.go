package ksl

type tokenKind int

const (
	tEOF tokenKind = iota
	tNewline
	tIdent
	tScalar
	tHash // '#' + name: an id marker or a hex colour; the parser
	// decides by position (Grammar Rules 8 and 13)
	tRef    // '$' ident, the head of a binding
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
