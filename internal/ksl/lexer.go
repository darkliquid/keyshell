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
		case c == '%':
			l.errf(l.here(), "stray '%%' (units attach with no space)")
			l.adv()
		default:
			l.lexOther()
		}
	}
}

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

const oneCharOps = "+-*/<>!?="
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
