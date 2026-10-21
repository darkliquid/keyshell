package anim

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

func TestLerp(t *testing.T) {
	tests := []struct {
		a, b eval.Value
		t    float64
		want eval.Value
		ok   bool
	}{
		{eval.Num(0), eval.Num(10), 0.5, eval.Num(5), true},
		{eval.Length{V: 0, Unit: "px"}, eval.Length{V: 100, Unit: "px"}, 0.25, eval.Length{V: 25, Unit: "px"}, true},
		{eval.Color{R: 0, G: 0, B: 0, A: 0xFF}, eval.Color{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, 0.5, eval.Color{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}, true},
		{eval.Str("a"), eval.Str("b"), 0.4, eval.Str("a"), true},
		{eval.Str("a"), eval.Str("b"), 0.6, eval.Str("b"), true},
		{eval.Bool(false), eval.Bool(true), 0.9, eval.Bool(true), true},
		{eval.Num(1), eval.Str("x"), 0.5, nil, false},
		{eval.Length{V: 1, Unit: "px"}, eval.Length{V: 2, Unit: "deg"}, 0.5, nil, false},
	}
	for _, tt := range tests {
		got, ok := Lerp(tt.a, tt.b, tt.t)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("Lerp(%v, %v, %v) = %v, %v; want %v, %v", tt.a, tt.b, tt.t, got, ok, tt.want, tt.ok)
		}
	}
}
