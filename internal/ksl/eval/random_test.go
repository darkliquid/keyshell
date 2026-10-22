package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func TestRandIsStableWithinASummon(t *testing.T) {
	c := &RandCache{}
	pos := ksl.Position{Offset: 10}
	a := c.Draw(pos, "scene.ksl", 3, 42, -4, 4)
	b := c.Draw(pos, "scene.ksl", 3, 42, -4, 4)
	if a != b {
		t.Errorf("same summon redrew: %v vs %v", a, b)
	}
	// different call sites are independent draws within one summon
	other := c.Draw(ksl.Position{Offset: 20}, "scene.ksl", 3, 42, -4, 4)
	if other == a {
		t.Errorf("different call sites drew the same value twice: %v", other)
	}
	c2 := c.Draw(pos, "scene.ksl", 4, 42, -4, 4)
	if c2 == a {
		t.Errorf("new summon did not redraw: %v", c2)
	}
	if c2 < -4 || c2 > 4 {
		t.Errorf("rand out of range: %v", c2)
	}
}

func TestRandCacheAcrossResolves(t *testing.T) {
	st := &State{Summon: 1, Seed: 7, RandCache: &RandCache{}}
	v1, err := evalOne(t, st, "rand(0, 100)")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := evalOne(t, st, "rand(0, 100)")
	if err != nil {
		t.Fatal(err)
	}
	if v1 != v2 {
		t.Errorf("rand redrawn within one summon: %v vs %v", v1, v2)
	}
	st.Summon = 2
	v3, err := evalOne(t, st, "rand(0, 100)")
	if err != nil {
		t.Fatal(err)
	}
	if v3 == v1 {
		t.Errorf("rand not redrawn across summons: %v", v3)
	}
}

func TestRandNeedsCache(t *testing.T) {
	if _, err := evalOne(t, &State{}, "rand(0, 1)"); err == nil {
		t.Errorf("rand without a RandCache = nil error, want failure")
	}
}

func TestJitterIsDeterministic(t *testing.T) {
	a := jitter("a-item", -3, 3)
	b := jitter("a-item", -3, 3)
	c := jitter("b-item", -3, 3)
	if a != b {
		t.Errorf("jitter not deterministic: %v vs %v", a, b)
	}
	if a < -3 || a > 3 || c < -3 || c > 3 {
		t.Errorf("jitter out of range: %v, %v", a, c)
	}
	if a == c {
		t.Logf("jitter collided on distinct keys (acceptable but suspicious)")
	}
}
