package anim

import (
	"math"
	"time"

	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

// Clock records when each state last flipped, so transitions measure
// elapsed time from the edge.
type Clock struct {
	now    time.Time
	edges  map[string]time.Time
	active map[string]bool
}

func NewClock(now time.Time) *Clock {
	return &Clock{now: now, edges: map[string]time.Time{}, active: map[string]bool{}}
}

// Tick records the current frame's time and the truth of each state.
func (c *Clock) Tick(now time.Time, states map[string]bool) {
	c.now = now
	for name, on := range states {
		if on != c.active[name] {
			c.active[name] = on
			c.edges[name] = now
		}
	}
}

// Edge reports when a state last flipped.
func (c *Clock) Edge(state string) (time.Time, bool) {
	t, ok := c.edges[state]
	return t, ok
}

// Transition tweens a property map from one resolved frame to the next
// over the transition's per-property timings.
func (c *Clock) Transition(tr *ksl.Transition, from, to map[string]eval.Value, now time.Time) map[string]eval.Value {
	edge, ok := c.edges[tr.State]
	if !ok {
		return to
	}
	elapsed := now.Sub(edge)
	out := map[string]eval.Value{}
	for k, tv := range to {
		fv, ok := from[k]
		if !ok {
			out[k] = tv
			continue
		}
		tp := timingFor(tr, k)
		if tp == nil {
			out[k] = tv
			continue
		}
		ev := evalFromAST(tp.Easing)
		if ev == nil {
			out[k] = tv
			continue
		}
		easing, err := ParseEasing(ev)
		if err != nil {
			out[k] = tv
			continue
		}
		t := math.Min(1, float64(elapsed)/float64(durScalar(tp.Dur)))
		if t < 0 {
			t = 0
		}
		if v, ok := Lerp(fv, tv, easing(t)); ok {
			out[k] = v
		} else {
			out[k] = tv
		}
	}
	return out
}

func timingFor(tr *ksl.Transition, key string) *ksl.TimingProp {
	for _, tp := range tr.Props {
		if tp.Key == key {
			return tp
		}
	}
	return nil
}

func durScalar(s ksl.Scalar) time.Duration {
	switch s.Unit {
	case "s":
		return time.Duration(s.N*1000) * time.Millisecond
	default: // ms or bare
		return time.Duration(s.N) * time.Millisecond
	}
}

// evalFromAST converts an AST easing to an eval value; a nil or
// unsupported value yields linear.
