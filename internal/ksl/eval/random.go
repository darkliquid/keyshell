package eval

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// RandCache freezes rand() draws for the duration of one summon
// (Grammar Rule 12). Draws are keyed by call site: the AST position
// identifies the source location. Each summon reseeds from the state
// seed mixed with the epoch, so a re-summon is a fresh draw.
type RandCache struct {
	epoch int
	seed  uint64
	rng   *rand.Rand
	draws map[callSite]float64
}

type callSite struct {
	name   string
	offset int
}

// Draw returns the cached draw for the call site, sampling once per
// summon epoch.
func (c *RandCache) Draw(pos ksl.Position, name string, epoch int, seed uint64, min, max float64) float64 {
	if c.draws == nil || c.epoch != epoch || c.seed != seed {
		c.epoch = epoch
		c.seed = seed
		c.draws = map[callSite]float64{}
		c.rng = rand.New(rand.NewSource(int64(seed + uint64(epoch)*0x9E3779B97F4A7C15)))
	}
	s := callSite{name: name, offset: pos.Offset}
	if v, ok := c.draws[s]; ok {
		return v
	}
	v := min + c.rng.Float64()*(max-min)
	c.draws[s] = v
	return v
}

// evalRand implements rand(min, max).
func (e *Env) evalRand(call *ksl.FuncCall, args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("%s: rand wants (min, max)", call.Pos())
	}
	min, okMin := args[0].(Num)
	max, okMax := args[1].(Num)
	if !okMin || !okMax {
		return nil, fmt.Errorf("%s: rand bounds must be numbers", call.Pos())
	}
	if e.st.RandCache == nil {
		return nil, fmt.Errorf("%s: rand requires a RandCache on State", call.Pos())
	}
	draw := e.st.RandCache.Draw(call.Pos(), e.st.NameOrDefault(), e.st.Summon, e.st.Seed, float64(min), float64(max))
	return Num(draw), nil
}

// evalJitter implements jitter(key, min, max).
func (e *Env) evalJitter(call *ksl.FuncCall, args []Value) (Value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("%s: jitter wants (key, min, max)", call.Pos())
	}
	key := stringify(args[0])
	min, okMin := args[1].(Num)
	max, okMax := args[2].(Num)
	if !okMin || !okMax {
		return nil, fmt.Errorf("%s: jitter bounds must be numbers", call.Pos())
	}
	return Num(jitter(key, float64(min), float64(max))), nil
}

// jitter hashes key into [min, max]: deterministic, frame-safe, varies
// per key (Grammar Rule 12).
func jitter(key string, min, max float64) float64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	u := float64(h.Sum64()%1000000) / 1000000
	return min + u*(max-min)
}

// NameOrDefault is the scene name used in rand call-site keys.
func (st *State) NameOrDefault() string {
	if st.Name == "" {
		return "scene.ksl"
	}
	return st.Name
}
