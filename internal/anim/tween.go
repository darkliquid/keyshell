package anim

import "github.com/darkliquid/keyshell/internal/ksl/eval"

// Lerp interpolates two resolved values at t in [0,1]; it reports false
// for pairs that cannot mix.
func Lerp(a, b eval.Value, t float64) (eval.Value, bool) {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	switch av := a.(type) {
	case eval.Num:
		if bv, ok := b.(eval.Num); ok {
			return eval.Num(float64(av) + (float64(bv)-float64(av))*t), true
		}
	case eval.Length:
		if bv, ok := b.(eval.Length); ok && av.Unit == bv.Unit {
			return eval.Length{V: av.V + (bv.V-av.V)*t, Unit: av.Unit}, true
		}
	case eval.Color:
		if bv, ok := b.(eval.Color); ok {
			l := func(a, b uint8) uint8 {
				return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5)
			}
			return eval.Color{R: l(av.R, bv.R), G: l(av.G, bv.G), B: l(av.B, bv.B), A: l(av.A, bv.A)}, true
		}
	case eval.Str:
		if bv, ok := b.(eval.Str); ok {
			if t < 0.5 {
				return av, true
			}
			return bv, true
		}
	case eval.Bool:
		if bv, ok := b.(eval.Bool); ok {
			if t < 0.5 {
				return av, true
			}
			return bv, true
		}
	}
	return nil, false
}
