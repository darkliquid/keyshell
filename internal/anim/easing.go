// Package anim evaluates the KSL animation model: easings, tweens,
// keyframes and the frame clock. It is pure and clock-injected.
package anim

import (
	"fmt"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

// Easing maps t in [0,1] to an eased fraction.
type Easing func(t float64) float64

// ParseEasing maps an eval value (Keyword or cubic-bezier FuncCall) to
// an easing function.
func ParseEasing(v eval.Value) (Easing, error) {
	switch x := v.(type) {
	case eval.Kw:
		switch x {
		case "linear":
			return linear, nil
		case "ease":
			return cubicBezierEasing(0.25, 0.1, 0.25, 1), nil
		case "ease-in":
			return cubicBezierEasing(0.42, 0, 1, 1), nil
		case "ease-out":
			return cubicBezierEasing(0, 0, 0.58, 1), nil
		case "ease-in-out":
			return cubicBezierEasing(0.42, 0, 0.58, 1), nil
		}
		return nil, fmt.Errorf("unknown easing %q", x)
	case eval.FuncVal:
		if x.Name != "cubic-bezier" {
			return nil, fmt.Errorf("unsupported easing %q", x.Name)
		}
		if len(x.Args) != 4 {
			return nil, fmt.Errorf("cubic-bezier wants 4 numbers, got %d", len(x.Args))
		}
		var pts [4]float64
		for i, a := range x.Args {
			n, ok := a.(eval.Num)
			if !ok {
				return nil, fmt.Errorf("cubic-bezier arg %d is %T, want number", i, a)
			}
			pts[i] = float64(n)
		}
		return cubicBezierEasing(pts[0], pts[1], pts[2], pts[3]), nil
	}
	return nil, fmt.Errorf("unsupported easing %T", v)
}

func linear(t float64) float64 { return t }

// cubicBezierEasing solves the cubic bezier by bisection on x(t)=t.
func cubicBezierEasing(x1, y1, x2, y2 float64) Easing {
	return func(t float64) float64 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		lo, hi := 0.0, 1.0
		for i := 0; i < 32; i++ {
			mid := (lo + hi) / 2
			if bezX(mid, x1, x2) < t {
				lo = mid
			} else {
				hi = mid
			}
		}
		s := (lo + hi) / 2
		return bezY(s, y1, y2)
	}
}

func bezX(t, x1, x2 float64) float64 { return bez(t, 0, x1, x2, 1) }
func bezY(t, y1, y2 float64) float64 { return bez(t, 0, y1, y2, 1) }

func bez(t, a, b, c, d float64) float64 {
	u := 1 - t
	return u*u*u*a + 3*u*u*t*b + 3*u*t*t*c + t*t*t*d
}
