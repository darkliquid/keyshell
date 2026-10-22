// Package eval resolves a parsed KSL scene against launcher state into
// a paintable Frame: typed property values, cascade/states/when, List
// materialisation and layout. It never imports graphics code; text
// measurement is injected behind Measurer.
package eval

import (
	"fmt"
	"strconv"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// Value is a resolved runtime value.
type Value interface{ isValue() }

type Num float64
type Str string
type Bool bool

// Length is a dimensioned value: px, %, deg, ms or s.
type Length struct {
	V    float64
	Unit string
}

// Color is 8-bit RGBA.
type Color struct{ R, G, B, A uint8 }

// Kw is a bare keyword value (tone, alignment, easing name) awaiting
// per-property resolution.
type Kw string

// Array is a value list (assets, gradient stops).
type Array []Value

// Props is an inline property block.
type Props []PropVal

type PropVal struct {
	Key string
	Val Value
}

func (Num) isValue()    {}
func (Str) isValue()    {}
func (Bool) isValue()   {}
func (Length) isValue() {}
func (Color) isValue()  {}
func (Kw) isValue()     {}
func (Array) isValue()  {}
func (Props) isValue()  {}

// literal converts an AST literal to a runtime value. Bindings and
// operators are Eval's job.
func literal(v ksl.Value) (Value, error) {
	switch x := v.(type) {
	case *ksl.Scalar:
		if x.Unit == "" {
			return Num(x.N), nil
		}
		return Length{V: x.N, Unit: x.Unit}, nil
	case *ksl.StringVal:
		return Str(x.S), nil
	case *ksl.BoolVal:
		return Bool(x.B), nil
	case *ksl.ColorVal:
		return hexColor(x.Hex)
	case *ksl.Keyword:
		return Kw(x.Name), nil
	case *ksl.ArrayVal:
		out := make(Array, len(x.Elems))
		for i, e := range x.Elems {
			ev, err := literal(e)
			if err != nil {
				return nil, err
			}
			out[i] = ev
		}
		return out, nil
	case *ksl.InlineBlock:
		out := make(Props, len(x.Props))
		for i, pr := range x.Props {
			pv, err := literal(pr.Val)
			if err != nil {
				return nil, err
			}
			out[i] = PropVal{Key: pr.Key, Val: pv}
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s: not a literal: %s", v.Pos(), v)
}

// hexColor decodes 3/4/6/8-digit hex with CSS shorthand semantics
// (Grammar Rule 20).
func hexColor(h string) (Color, error) {
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("bad hex colour %q", h)
	}
	switch len(h) {
	case 3: // RGB
		return Color{uint8(n>>8&0xF) * 0x11, uint8(n>>4&0xF) * 0x11, uint8(n&0xF) * 0x11, 0xFF}, nil
	case 4: // RGBA
		return Color{uint8(n>>12&0xF) * 0x11, uint8(n>>8&0xF) * 0x11, uint8(n>>4&0xF) * 0x11, uint8(n&0xF) * 0x11}, nil
	case 6:
		return Color{uint8(n >> 16), uint8(n >> 8 & 0xFF), uint8(n & 0xFF), 0xFF}, nil
	case 8:
		return Color{uint8(n >> 24), uint8(n >> 16 & 0xFF), uint8(n >> 8 & 0xFF), uint8(n & 0xFF)}, nil
	}
	return Color{}, fmt.Errorf("hex colours have 3, 4, 6 or 8 digits, got %d", len(h))
}
