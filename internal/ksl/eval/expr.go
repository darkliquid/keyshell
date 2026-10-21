package eval

import (
	"fmt"
	"strconv"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// FuncVal is an evaluated effect/helper call (blur, shadow, gradients,
// tilt); the painter interprets it.
type FuncVal struct {
	At   ksl.Position
	Name string
	Args []Value
}

func (FuncVal) isValue() {}

// Eval evaluates an AST value against the environment.
func (e *Env) Eval(v ksl.Value) (Value, error) {
	switch x := v.(type) {
	case *ksl.Scalar, *ksl.StringVal, *ksl.BoolVal, *ksl.ColorVal, *ksl.Keyword,
		*ksl.ArrayVal, *ksl.InlineBlock:
		return literal(v)
	case *ksl.Ref:
		if got, ok := e.lookupRef(x.Name); ok {
			return got, nil
		}
		return nil, fmt.Errorf("%s: unknown binding $%s", x.Pos(), x.Name)
	case *ksl.RefPath:
		key := x.ID + "." + x.Prop
		if got, ok := e.RefPaths[key]; ok {
			return got, nil
		}
		return nil, fmt.Errorf("%s: unknown reference #%s", x.Pos(), key)
	case *ksl.FuncCall:
		args := make([]Value, len(x.Args))
		for i, a := range x.Args {
			av, err := e.Eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = av
		}
		if x.Name == "rand" {
			return e.evalRand(x, args)
		}
		if x.Name == "jitter" {
			return e.evalJitter(x, args)
		}
		return FuncVal{At: x.Pos(), Name: x.Name, Args: args}, nil
	case *ksl.Unary:
		xv, err := e.Eval(x.X)
		if err != nil {
			return nil, err
		}
		switch x.Op {
		case "-":
			switch n := xv.(type) {
			case Num:
				return -n, nil
			case Length:
				n.V = -n.V
				return n, nil
			}
		case "!":
			if b, ok := xv.(Bool); ok {
				return !b, nil
			}
		}
		return nil, fmt.Errorf("%s: bad operand for %s: %T", x.Pos(), x.Op, xv)
	case *ksl.Binary:
		return e.evalBinary(x)
	case *ksl.Ternary:
		cv, err := e.Eval(x.Cond)
		if err != nil {
			return nil, err
		}
		if !truthy(cv) {
			return e.Eval(x.Else)
		}
		return e.Eval(x.Then)
	case *ksl.Member:
		xv, err := e.Eval(x.X)
		if err != nil {
			return nil, err
		}
		return e.member(xv, x.Name)
	case *ksl.MethodCall:
		xv, err := e.Eval(x.X)
		if err != nil {
			return nil, err
		}
		args := make([]Value, len(x.Args))
		for i, a := range x.Args {
			av, err := e.Eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = av
		}
		return e.call(xv, x.Name, args)
	}
	return nil, fmt.Errorf("%s: unsupported value %T", v.Pos(), v)
}

// truthy implements the spec's truthiness: non-zero number, non-empty
// string, true.
func truthy(v Value) bool {
	switch x := v.(type) {
	case Bool:
		return bool(x)
	case Num:
		return x != 0
	case Str:
		return x != ""
	case Length:
		return x.V != 0
	}
	return v != nil
}

func (e *Env) evalBinary(x *ksl.Binary) (Value, error) {
	l, err := e.Eval(x.L)
	if err != nil {
		return nil, err
	}
	r, err := e.Eval(x.R)
	if err != nil {
		return nil, err
	}
	switch x.Op {
	case "&&":
		return Bool(truthy(l) && truthy(r)), nil
	case "||":
		return Bool(truthy(l) || truthy(r)), nil
	case "==":
		return Bool(equals(l, r)), nil
	case "!=":
		return Bool(!equals(l, r)), nil
	case "<", "<=", ">", ">=":
		ln, lok := numValue(l)
		rn, rok := numValue(r)
		if !lok || !rok {
			return nil, fmt.Errorf("%s: cannot compare %T with %T", x.Pos(), l, r)
		}
		switch x.Op {
		case "<":
			return Bool(ln < rn), nil
		case "<=":
			return Bool(ln <= rn), nil
		case ">":
			return Bool(ln > rn), nil
		default:
			return Bool(ln >= rn), nil
		}
	case "+":
		if ls, ok := l.(Str); ok {
			return ls + Str(stringify(r)), nil
		}
		if rs, ok := r.(Str); ok {
			return Str(stringify(l)) + rs, nil
		}
		return arith(x.Pos(), x.Op, l, r)
	}
	return arith(x.Pos(), x.Op, l, r)
}

// arith implements the numeric and length algebra.
func arith(pos ksl.Position, op string, l, r Value) (Value, error) {
	ln, lok := numValue(l)
	rn, rok := numValue(r)
	if !lok || !rok {
		return nil, fmt.Errorf("%s: %T %s %T is not defined", pos, l, op, r)
	}
	ll, lIsLen := l.(Length)
	rl, rIsLen := r.(Length)
	if lIsLen && rIsLen && ll.Unit != rl.Unit {
		return nil, fmt.Errorf("%s: %s and %s are different units", pos, ll.Unit, rl.Unit)
	}
	var out float64
	length := false
	unit := ""
	switch op {
	case "+":
		out = ln + rn
		length, unit = lIsLen || rIsLen, lenUnit(ll, rl)
	case "-":
		out = ln - rn
		length, unit = lIsLen || rIsLen, lenUnit(ll, rl)
	case "*":
		out = ln * rn
		// length * length is not a length; length * number is
		if lIsLen != rIsLen {
			length, unit = true, lenUnit(ll, rl)
		}
	case "/":
		if rn == 0 {
			return nil, fmt.Errorf("%s: division by zero", pos)
		}
		out = ln / rn
		// length / length is a plain number; length / number stays a length
		if lIsLen && !rIsLen {
			length, unit = true, ll.Unit
		}
	}
	if length {
		return Length{V: out, Unit: unit}, nil
	}
	return Num(out), nil
}

func lenUnit(a, b Length) string {
	if a.Unit != "" {
		return a.Unit
	}
	return b.Unit
}

func numValue(v Value) (float64, bool) {
	switch n := v.(type) {
	case Num:
		return float64(n), true
	case Length:
		return n.V, true
	}
	return 0, false
}

// equals is panic-safe: interface comparison panics on uncomparable
// dynamic types (Array, Props, FuncVal).
func equals(l, r Value) bool {
	switch lv := l.(type) {
	case Num:
		rv, ok := r.(Num)
		return ok && lv == rv
	case Str:
		rv, ok := r.(Str)
		return ok && lv == rv
	case Bool:
		rv, ok := r.(Bool)
		return ok && lv == rv
	case Length:
		rv, ok := r.(Length)
		return ok && lv == rv
	case Color:
		rv, ok := r.(Color)
		return ok && lv == rv
	case Kw:
		rv, ok := r.(Kw)
		return ok && lv == rv
	case ItemVal:
		rv, ok := r.(ItemVal)
		return ok && lv == rv
	case RevealVal:
		rv, ok := r.(RevealVal)
		return ok && lv == rv
	}
	return false
}

// stringify renders a value for string concatenation (Grammar Rule 19:
// numbers stringify in shortest round-trip form).
func stringify(v Value) string {
	switch x := v.(type) {
	case Str:
		return string(x)
	case Num:
		return strconv.FormatFloat(float64(x), 'g', -1, 64)
	case Length:
		return strconv.FormatFloat(x.V, 'g', -1, 64) + x.Unit
	case Bool:
		if x {
			return "true"
		}
		return "false"
	case Color:
		return fmt.Sprintf("#%02X%02X%02X%02X", x.R, x.G, x.B, x.A)
	case Kw:
		return string(x)
	}
	return fmt.Sprintf("%v", v)
}
