// Package calc ports the calculator grammar from the QML implementation's
// calc.js: a small recursive-descent parser for arithmetic with constants,
// functions, unary signs, exponentiation and modulo. It never evaluates; it
// parses, so there is no injection surface.
package calc

import (
	"math"
	"strconv"
	"strings"
)

// Eval parses and evaluates expression, returning the formatted result and
// whether the expression was a valid, finite calculation. Formatting matches
// the original: thousands grouping, up to 10 decimals, and scientific notation
// for very large or very small magnitudes.
func Eval(expression string) (string, bool) {
	text := strings.TrimSpace(expression)
	if text == "" {
		return "", false
	}
	value, err := evaluate(text)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", false
	}
	return format(value, 10), true
}

type parser struct {
	text string
	pos  int
}

func evaluate(expression string) (float64, error) {
	p := &parser{text: expression}
	v, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.pos != len(p.text) {
		return 0, errNotConsumed
	}
	return v, nil
}

var errNotConsumed = &parseError{"trailing characters"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return "calc: " + e.msg }

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.text) && (p.text[p.pos] == ' ' || p.text[p.pos] == '\t' || p.text[p.pos] == '\n' || p.text[p.pos] == '\r') {
		p.pos++
	}
}

func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	seenDot := false
	for p.pos < len(p.text) {
		ch := p.text[p.pos]
		if isDigit(ch) {
			p.pos++
			continue
		}
		if ch == '.' && !seenDot {
			seenDot = true
			p.pos++
			continue
		}
		break
	}
	value, err := parseFloat(p.text[start:p.pos])
	if err != nil {
		return 0, err
	}
	return value, nil
}

func parseFloat(s string) (float64, error) {
	if s == "" || s == "." {
		return 0, &parseError{"bad number"}
	}
	var v float64
	var div float64 = 10
	seenDot := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if seenDot {
				return 0, &parseError{"bad number"}
			}
			seenDot = true
			continue
		}
		if !isDigit(c) {
			return 0, &parseError{"bad number"}
		}
		d := float64(c - '0')
		if seenDot {
			v += d / div
			div *= 10
		} else {
			v = v*10 + d
		}
	}
	return v, nil
}

func (p *parser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.text) && isLetter(p.text[p.pos]) {
		p.pos++
	}
	return strings.ToLower(p.text[start:p.pos])
}

func (p *parser) parseArguments() ([]float64, error) {
	var args []float64
	p.skipSpaces()
	if p.pos < len(p.text) && p.text[p.pos] == ')' {
		p.pos++
		return args, nil
	}
	for {
		v, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		p.skipSpaces()
		if p.pos >= len(p.text) {
			return nil, &parseError{"expected , or )"}
		}
		switch p.text[p.pos] {
		case ',':
			p.pos++
		case ')':
			p.pos++
			return args, nil
		default:
			return nil, &parseError{"expected , or )"}
		}
	}
}

func (p *parser) parsePrimary() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.text) {
		return 0, &parseError{"unexpected end"}
	}
	ch := p.text[p.pos]

	switch {
	case ch == '(':
		p.pos++
		v, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.text) || p.text[p.pos] != ')' {
			return 0, &parseError{"expected )"}
		}
		p.pos++
		return v, nil
	case isDigit(ch) || ch == '.':
		return p.parseNumber()
	case isLetter(ch):
		name := p.parseIdentifier()
		p.skipSpaces()
		if p.pos < len(p.text) && p.text[p.pos] == '(' {
			p.pos++
			args, err := p.parseArguments()
			if err != nil {
				return 0, err
			}
			return callFunction(name, args)
		}
		if v, ok := constants[name]; ok {
			return v, nil
		}
		return 0, &parseError{"unknown identifier:" + name}
	default:
		return 0, &parseError{"unexpected character"}
	}
}

func (p *parser) parseUnary() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.text) {
		return 0, &parseError{"unexpected end"}
	}
	switch p.text[p.pos] {
	case '-':
		p.pos++
		v, err := p.parseUnary()
		return -v, err
	case '+':
		p.pos++
		return p.parseUnary()
	}
	return p.parsePrimary()
}

func (p *parser) parsePower() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.pos < len(p.text) && p.text[p.pos] == '^' {
		p.pos++
		exp, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *parser) parseTerm() (float64, error) {
	value, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.text) {
			return value, nil
		}
		switch p.text[p.pos] {
		case '*':
			p.pos++
			v, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			value *= v
		case '/':
			p.pos++
			v, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			value /= v
		case '%':
			p.pos++
			v, err := p.parsePower()
			if err != nil {
				return 0, err
			}
			value = math.Mod(value, v)
		default:
			return value, nil
		}
	}
}

func (p *parser) parseExpression() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.text) {
			return value, nil
		}
		switch p.text[p.pos] {
		case '+':
			p.pos++
			v, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value += v
		case '-':
			p.pos++
			v, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= v
		default:
			return value, nil
		}
	}
}

var constants = map[string]float64{
	"pi": math.Pi,
	"e":  math.E,
}

func callFunction(name string, args []float64) (float64, error) {
	switch name {
	case "sqrt":
		if len(args) == 1 {
			return math.Sqrt(args[0]), nil
		}
	case "abs":
		if len(args) == 1 {
			return math.Abs(args[0]), nil
		}
	case "round":
		if len(args) == 1 {
			return math.Round(args[0]), nil
		}
	case "floor":
		if len(args) == 1 {
			return math.Floor(args[0]), nil
		}
	case "ceil":
		if len(args) == 1 {
			return math.Ceil(args[0]), nil
		}
	case "pow":
		if len(args) == 2 {
			return math.Pow(args[0], args[1]), nil
		}
	case "min":
		if len(args) > 0 {
			m := args[0]
			for _, a := range args[1:] {
				if a < m {
					m = a
				}
			}
			return m, nil
		}
	case "max":
		if len(args) > 0 {
			m := args[0]
			for _, a := range args[1:] {
				if a > m {
					m = a
				}
			}
			return m, nil
		}
	}
	return 0, &parseError{"bad call: " + name}
}

// format and groupThousands replicate the formatting of the original.
func groupThousands(text string) string {
	negative := strings.HasPrefix(text, "-")
	body := text
	if negative {
		body = text[1:]
	}
	dot := strings.IndexByte(body, '.')
	digits := body
	rest := ""
	if dot != -1 {
		digits = body[:dot]
		rest = body[dot:]
	}
	var grouped strings.Builder
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteByte(digits[i])
	}
	prefix := ""
	if negative {
		prefix = "-"
	}
	return prefix + grouped.String() + rest
}

func format(value float64, maxDecimals int) string {
	if maxDecimals < 0 {
		maxDecimals = 10
	}
	abs := math.Abs(value)
	if abs != 0 && (abs >= 1e15 || abs < 1e-9) {
		return formatFloatG(value)
	}
	text := strconvFormatFloat(value, maxDecimals)
	if idx := strings.IndexByte(text, '.'); idx != -1 {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	return groupThousands(text)
}

func strconvFormatFloat(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	return s
}

func formatFloatG(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
