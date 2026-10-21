package anim

import (
	"fmt"
	"math"
	"time"

	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

// Sample evaluates a keyframe animation at a point in time. The
// animation's duration, easing, iteration and direction are honoured.
func Sample(kf *ksl.Keyframes, step ksl.AnimStep, at time.Duration) (map[string]eval.Value, error) {
	if len(kf.Frames) == 0 {
		return nil, fmt.Errorf("keyframes %s has no frames", kf.Name)
	}
	dur, err := durationOf(step)
	if err != nil {
		return nil, err
	}
	if dur <= 0 {
		return frameProps(kf.Frames[len(kf.Frames)-1].Props), nil
	}
	cycle := float64(at) / float64(dur)

	// Finite iteration clamps to the last frame.
	if count := iterationCount(step); count >= 0 && cycle >= float64(count) {
		return frameProps(kf.Frames[len(kf.Frames)-1].Props), nil
	}

	pos := math.Mod(cycle, 1)
	reversed := step.Direction == "reverse" || (step.Direction == "alternate" && int(cycle)%2 == 1)
	if reversed {
		pos = 1 - pos
	}
	pct := pos * 100

	var lo, hi *ksl.Frame
	for _, f := range kf.Frames {
		if f.Percent <= pct {
			lo = f
		}
		if f.Percent >= pct && hi == nil {
			hi = f
		}
	}
	if lo == nil {
		lo = kf.Frames[0]
	}
	if hi == nil {
		hi = kf.Frames[len(kf.Frames)-1]
	}

	easing, err := ParseEasing(easingOf(step))
	if err != nil {
		return nil, err
	}
	t := 0.0
	if hi.Percent != lo.Percent {
		t = easing((pct - lo.Percent) / (hi.Percent - lo.Percent))
	}

	loProps := frameProps(lo.Props)
	hiProps := frameProps(hi.Props)
	out := map[string]eval.Value{}
	for k, lv := range loProps {
		if hv, ok := hiProps[k]; ok {
			if v, ok := Lerp(lv, hv, t); ok {
				out[k] = v
			} else {
				out[k] = lv
			}
		} else {
			out[k] = lv
		}
	}
	return out, nil
}

// frameProps evaluates keyframe props to literal values.
func frameProps(props []*ksl.Prop) map[string]eval.Value {
	out := map[string]eval.Value{}
	for _, pr := range props {
		if v, err := literalValue(pr.Val); err == nil {
			out[pr.Key] = v
		}
	}
	return out
}

// literalValue converts a keyframe prop value (in practice a literal)
// using eval's conversion.
func literalValue(v ksl.Value) (eval.Value, error) {
	switch x := v.(type) {
	case *ksl.Scalar:
		if x.Unit == "" {
			return eval.Num(x.N), nil
		}
		return eval.Length{V: x.N, Unit: x.Unit}, nil
	case *ksl.ColorVal:
		n := uint32(0)
		for _, c := range x.Hex {
			n = n<<4 | hexDigit(c)
		}
		switch len(x.Hex) {
		case 3:
			return eval.Color{R: uint8(n>>8&0xF) * 0x11, G: uint8(n>>4&0xF) * 0x11, B: uint8(n&0xF) * 0x11, A: 0xFF}, nil
		case 4:
			return eval.Color{R: uint8(n>>12&0xF) * 0x11, G: uint8(n>>8&0xF) * 0x11, B: uint8(n>>4&0xF) * 0x11, A: uint8(n&0xF) * 0x11}, nil
		case 6:
			return eval.Color{R: uint8(n >> 16), G: uint8(n >> 8 & 0xFF), B: uint8(n & 0xFF), A: 0xFF}, nil
		case 8:
			return eval.Color{R: uint8(n >> 24), G: uint8(n >> 16 & 0xFF), B: uint8(n >> 8 & 0xFF), A: uint8(n & 0xFF)}, nil
		}
	}
	return nil, fmt.Errorf("unsupported literal %T", v)
}

func hexDigit(c rune) uint32 {
	switch {
	case c >= '0' && c <= '9':
		return uint32(c - '0')
	case c >= 'a' && c <= 'f':
		return uint32(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return uint32(c-'A') + 10
	}
	return 0
}

func durationOf(step ksl.AnimStep) (time.Duration, error) {
	if step.Dur == nil {
		return 0, nil
	}
	switch step.Dur.Unit {
	case "ms":
		return time.Duration(step.Dur.N) * time.Millisecond, nil
	case "s":
		return time.Duration(step.Dur.N*1000) * time.Millisecond, nil
	}
	return time.Duration(step.Dur.N) * time.Millisecond, nil
}

// iterationCount returns the finite iteration count, or -1 for
// infinite.
func iterationCount(step ksl.AnimStep) int {
	if it, ok := step.Iteration.(*ksl.Scalar); ok {
		return int(it.N)
	}
	return -1
}

func easingOf(step ksl.AnimStep) eval.Value {
	if step.Easing != nil {
		return evalFromAST(step.Easing)
	}
	return eval.Kw("linear")
}

// evalFromAST converts an AST easing value (Keyword or FuncCall) into
// an eval value via a minimal mapping.
func evalFromAST(v ksl.Value) eval.Value {
	switch x := v.(type) {
	case *ksl.Keyword:
		return eval.Kw(x.Name)
	case *ksl.FuncCall:
		if x.Name == "cubic-bezier" {
			args := make([]eval.Value, len(x.Args))
			for i, a := range x.Args {
				if sc, ok := a.(*ksl.Scalar); ok {
					args[i] = eval.Num(sc.N)
				}
			}
			return eval.FuncVal{Name: x.Name, Args: args}
		}
	}
	return nil
}
