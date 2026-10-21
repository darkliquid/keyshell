package anim

import (
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

func mustKeyframes(t *testing.T, src string) *ksl.Keyframes {
	t.Helper()
	s, err := ksl.Parse("t.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	kf := s.Keyframes()[0]
	return kf
}

func keyKeyword(s string) *ksl.Keyword { return &ksl.Keyword{Name: s} }

func sampleOpacity(t *testing.T, kf *ksl.Keyframes, step ksl.AnimStep, at time.Duration) float64 {
	t.Helper()
	m, err := Sample(kf, step, at)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	return float64(m["opacity"].(eval.Num))
}

func sampleWidth(t *testing.T, kf *ksl.Keyframes, step ksl.AnimStep, at time.Duration) float64 {
	t.Helper()
	m, err := Sample(kf, step, at)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if v, ok := m["width"].(eval.Length); ok {
		return v.V
	}
	return float64(m["width"].(eval.Num))
}

func TestSampleBlink(t *testing.T) {
	kf := mustKeyframes(t, `@keyframes blink { 0% { opacity: 1 } 50% { opacity: 0 } 100% { opacity: 1 } }`)
	step := ksl.AnimStep{Name: "blink", Dur: &ksl.Scalar{N: 800, Unit: "ms"},
		Easing: keyKeyword("linear"), Iteration: keyKeyword("infinite")}
	if got := sampleOpacity(t, kf, step, 0); got != 1 {
		t.Errorf("t=0 opacity = %v", got)
	}
	if got := sampleOpacity(t, kf, step, 400*time.Millisecond); got != 0 {
		t.Errorf("t=400ms opacity = %v", got)
	}
	if got := sampleOpacity(t, kf, step, 1200*time.Millisecond); got != 0 {
		t.Errorf("t=1200ms opacity = %v, want wrapped half cycle 0", got)
	}
}

func TestSampleDirection(t *testing.T) {
	kf := mustKeyframes(t, `@keyframes grow { 0% { width: 0 } 100% { width: 100 } }`)
	rev := ksl.AnimStep{Name: "grow", Dur: &ksl.Scalar{N: 1000, Unit: "ms"}, Direction: "reverse"}
	if got := sampleWidth(t, kf, rev, 0); got != 100 {
		t.Errorf("reverse t=0 width = %v, want 100", got)
	}
	alt := ksl.AnimStep{Name: "grow", Dur: &ksl.Scalar{N: 1000, Unit: "ms"},
		Iteration: keyKeyword("infinite"), Direction: "alternate"}
	if got := sampleWidth(t, kf, alt, 1500*time.Millisecond); got != 50 {
		t.Errorf("alternate t=1.5 cycles width = %v, want 50", got)
	}
}

func TestSampleStopsAtFiniteIteration(t *testing.T) {
	kf := mustKeyframes(t, `@keyframes grow { 0% { width: 0 } 100% { width: 100 } }`)
	finite := ksl.AnimStep{Name: "grow", Dur: &ksl.Scalar{N: 1000, Unit: "ms"},
		Iteration: &ksl.Scalar{N: 2, Unit: ""}}
	if got := sampleWidth(t, kf, finite, 2500*time.Millisecond); got != 100 {
		t.Errorf("after 2 cycles width = %v, want clamped 100", got)
	}
}
