package anim

import (
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

func mustTransition(t *testing.T, src string) *ksl.Transition {
	t.Helper()
	s, err := ksl.Parse("t.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s.Transitions()[0]
}

func TestClockTracksEdges(t *testing.T) {
	c := NewClock(time.Unix(0, 0))
	c.Tick(time.Unix(0, 0), map[string]bool{"open": false})
	t1 := time.Unix(0, 100*int64(time.Millisecond))
	c.Tick(t1, map[string]bool{"open": true})
	start, ok := c.Edge("open")
	if !ok || !start.Equal(t1) {
		t.Errorf("edge = %v, %v", start, ok)
	}
	c.Tick(t1.Add(50*time.Millisecond), map[string]bool{"open": true})
	if _, ok := c.Edge("open"); !ok {
		t.Errorf("steady state must not re-arm the edge")
	}
}

func TestTransitionTweens(t *testing.T) {
	tr := mustTransition(t, `transition open { opacity: 200ms ease-out, rotate: 250ms ease-out }`)
	c := NewClock(time.Unix(0, 0))
	// arm the open edge at t=0
	c.Tick(time.Unix(0, 0), map[string]bool{"open": true})
	from := map[string]eval.Value{"opacity": eval.Num(0), "rotate": eval.Length{V: 2, Unit: "deg"}}
	to := map[string]eval.Value{"opacity": eval.Num(1), "rotate": eval.Length{V: 0, Unit: "deg"}}
	mid := c.Transition(tr, from, to, time.Unix(0, 100*int64(time.Millisecond)))
	op := mid["opacity"].(eval.Num)
	if op <= 0 || op >= 1 {
		t.Errorf("opacity at t=100ms/200ms ease-out = %v, want mid-flight", op)
	}
}

func TestTransitionNoEdgeReturnsTarget(t *testing.T) {
	tr := mustTransition(t, `transition open { opacity: 200ms }`)
	c := NewClock(time.Unix(0, 0))
	from := map[string]eval.Value{"opacity": eval.Num(0)}
	to := map[string]eval.Value{"opacity": eval.Num(1)}
	out := c.Transition(tr, from, to, time.Unix(0, 100*int64(time.Millisecond)))
	if out["opacity"] != eval.Num(1) {
		t.Errorf("transition without an edge = %v, want target", out["opacity"])
	}
}
