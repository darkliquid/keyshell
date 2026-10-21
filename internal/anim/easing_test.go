package anim

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

func keyword(name string) eval.Value { return eval.Kw(name) }

func cubicBezier(a, b, c, d float64) eval.Value {
	return eval.FuncVal{Name: "cubic-bezier", Args: []eval.Value{
		eval.Num(a), eval.Num(b), eval.Num(c), eval.Num(d),
	}}
}

func must(t *testing.T, e Easing, err error) Easing {
	t.Helper()
	if err != nil {
		t.Fatalf("ParseEasing: %v", err)
	}
	return e
}

func TestEasingEndpoints(t *testing.T) {
	for _, name := range []string{"linear", "ease", "ease-in", "ease-out", "ease-in-out"} {
		e, err := ParseEasing(keyword(name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if e(0) != 0 || e(1) != 1 {
			t.Errorf("%s endpoints = %v, %v", name, e(0), e(1))
		}
	}
	e, _ := ParseEasing(keyword("ease-in"))
	if got := e(0.5); got >= 0.5 {
		t.Errorf("ease-in(0.5) = %v, want < 0.5 (slow start)", got)
	}
	e2, _ := ParseEasing(keyword("ease-out"))
	if got := e2(0.5); got <= 0.5 {
		t.Errorf("ease-out(0.5) = %v, want > 0.5 (fast start)", got)
	}
}

func TestCubicBezier(t *testing.T) {
	e, err := ParseEasing(cubicBezier(0.42, 0, 0.58, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := e(0.5); got < 0.45 || got > 0.55 {
		t.Errorf("cubic-bezier(0.42,0,0.58,1)(0.5) = %v, want ~0.5", got)
	}
	prev := 0.0
	for i := 1; i <= 100; i++ {
		v := e(float64(i) / 100)
		if v < prev-1e-6 {
			t.Fatalf("easing went backwards at %d: %v < %v", i, v, prev)
		}
		prev = v
	}
}

func TestEasingErrors(t *testing.T) {
	if _, err := ParseEasing(keyword("bogus")); err == nil {
		t.Errorf("unknown easing = nil error")
	}
	if _, err := ParseEasing(eval.FuncVal{Name: "cubic-bezier", Args: []eval.Value{eval.Num(0.42), eval.Num(0), eval.Num(0.58)}}); err == nil {
		t.Errorf("wrong argument count = nil error")
	}
}
