package eval

import (
	"fmt"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// Frame is one resolved scene, ready to paint.
type Frame struct {
	Root *ResolvedNode
}

// ResolvedNode is a node with concrete props, a rect and ordered
// children. z is carried for the painter's sibling ordering.
type ResolvedNode struct {
	Name     string
	ID       string
	Classes  []string
	Rect     RectF
	Props    map[string]Value
	Children []*ResolvedNode
	Z        int
}

// Option configures resolution.
type Option func(*resolver)

// WithMeasurer injects text measurement (the renderer supplies the
// theme-backed implementation; tests use a fake).
func WithMeasurer(m Measurer) Option {
	return func(r *resolver) { r.measurer = m }
}

// WithEnv overrides the binding environment (used for item scopes).
func WithEnv(env *Env) Option {
	return func(r *resolver) { r.env = env }
}

type resolver struct {
	st       *State
	env      *Env
	ctx      *CascadeCtx
	measurer Measurer
}

// Resolve evaluates a scene against a state snapshot and returns the
// paintable frame.
func Resolve(scene *ksl.Scene, st State, opts ...Option) (*Frame, error) {
	if st.Scale == 0 {
		st.Scale = 1
	}
	if st.Tokens == nil {
		st.Tokens = sceneTokens(scene)
	}
	r := &resolver{st: &st}
	r.env = NewEnv(&st, nil)
	for _, o := range opts {
		o(r)
	}
	if r.ctx == nil {
		r.ctx = &CascadeCtx{Scene: scene, Mode: st.Mode}
	}
	nodes := scene.Nodes()
	if len(nodes) != 1 {
		return nil, fmt.Errorf("scene must have exactly one root node, found %d", len(nodes))
	}
	root, err := r.resolveNode(nodes[0], RectF{X: 0, Y: 0, W: st.Width, H: st.Height})
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("scene root is hidden")
	}
	return &Frame{Root: root}, nil
}

// resolveNode resolves a node in flow position: its rect derives from
// props and the parent bounds.
func (r *resolver) resolveNode(n *ksl.Node, bounds RectF) (*ResolvedNode, error) {
	props, ok, err := resolveProps(n, r.env, r.ctx)
	if err != nil || !ok {
		return nil, err
	}
	rn := &ResolvedNode{
		Name: n.Name, ID: n.ID, Classes: n.Classes,
		Props: props, Rect: r.nodeRect(n, props, bounds), Z: int(numProp(props, "z")),
	}
	if err := r.layoutChildrenOf(rn, n); err != nil {
		return nil, err
	}
	return rn, nil
}

// resolveNodeRect resolves a child whose rect a container has already
// computed (flex, grid, stack).
func (r *resolver) resolveNodeRect(n *ksl.Node, rect RectF) (*ResolvedNode, error) {
	props, ok, err := resolveProps(n, r.env, r.ctx)
	if err != nil || !ok {
		return nil, err
	}
	rn := &ResolvedNode{
		Name: n.Name, ID: n.ID, Classes: n.Classes,
		Props: props, Rect: r.refineRect(n, props, rect), Z: int(numProp(props, "z")),
	}
	if err := r.layoutChildrenOf(rn, n); err != nil {
		return nil, err
	}
	return rn, nil
}

func (r *resolver) layoutChildrenOf(rn *ResolvedNode, n *ksl.Node) error {
	switch n.Name {
	case "Row":
		return r.layoutFlex(rn, n, true)
	case "Column":
		return r.layoutFlex(rn, n, false)
	case "Grid":
		return r.layoutGrid(rn, n)
	case "Stack":
		return r.layoutStack(rn, n)
	default:
		return r.layoutChildren(rn, n)
	}
}

// sceneTokens pulls theme tokens from the scene's theme blocks, in
// declaration order (a later token overrides an earlier one).
func sceneTokens(scene *ksl.Scene) map[string]Color {
	out := map[string]Color{}
	var walk func(props []*ksl.Prop)
	walk = func(props []*ksl.Prop) {
		for _, pr := range props {
			if cv, ok := pr.Val.(*ksl.ColorVal); ok {
				if c, err := hexColor(cv.Hex); err == nil {
					out[pr.Key] = c
				}
			}
		}
	}
	for _, th := range scene.ThemeBlocks() {
		walk(th.Tokens)
	}
	return out
}
