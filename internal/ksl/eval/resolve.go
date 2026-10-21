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
	// Item is the List item this node was materialised for, set on
	// every node in a template instance's subtree.
	Item *Item
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
	defs     map[string]*ksl.Def
}

// Resolve evaluates a scene against a state snapshot and returns the
// paintable frame.
func Resolve(scene *ksl.Scene, st State, opts ...Option) (*Frame, error) {
	if st.Scale == 0 {
		st.Scale = 1
	}
	// Host tokens are the defaults; a scene `theme` block overrides
	// them (spec §4).
	tokens := make(map[string]Color, len(st.Tokens)+8)
	for k, v := range st.Tokens {
		tokens[k] = v
	}
	for k, v := range sceneTokens(scene) {
		tokens[k] = v
	}
	st.Tokens = tokens
	r := &resolver{st: &st, defs: map[string]*ksl.Def{}}
	for _, d := range scene.Defs() {
		r.defs[d.Name] = d
	}
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
	if n.Name == "List" {
		return r.resolveList(n, bounds)
	}
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
	if n.Name == "List" {
		return r.resolveList(n, rect)
	}
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

// children expands a def instance: a node whose name matches a def
// takes the def body's children, with the instance's own children after
// them.
func (r *resolver) children(n *ksl.Node) []*ksl.Node {
	if d := r.defs[n.Name]; d != nil {
		out := make([]*ksl.Node, 0, len(d.Children)+len(n.Children))
		out = append(out, d.Children...)
		out = append(out, n.Children...)
		return out
	}
	return n.Children
}

// resolveList materialises the List viewport: one template instance per
// item (capped), the empty template when nothing matches, and the
// overflow summary when the cap truncates. Items are positioned along
// the orientation axis with the list gap.
func (r *resolver) resolveList(n *ksl.Node, bounds RectF) (*ResolvedNode, error) {
	props, ok, err := resolveProps(n, r.env, r.ctx)
	if err != nil || !ok {
		return nil, err
	}
	rn := &ResolvedNode{
		Name: n.Name, ID: n.ID, Classes: n.Classes,
		Props: props, Rect: r.refineRect(n, props, bounds), Z: int(numProp(props, "z")),
	}

	var template, empty, summary *ksl.Node
	for _, c := range r.children(n) {
		switch c.Name {
		case "template":
			if len(c.Children) > 0 {
				template = c.Children[0]
			}
		case "empty":
			empty = c
		case "overflowSummary":
			summary = c
		}
	}

	items := listItems(props["items"])
	limit := len(items)
	if capN := int(numProp(props, "cap")); capN > 0 && capN < limit {
		limit = capN
	}
	selected := int(numProp(props, "selected"))
	horizontal := kwProp(props, "orientation") == "horizontal"
	gap := numProp(props, "gap")

	if limit == 0 {
		if empty != nil {
			kids, err := r.resolveChildrenOf(rn, empty)
			if err != nil {
				return nil, err
			}
			rn.Children = append(rn.Children, kids...)
		}
		return rn, nil
	}

	savedEnv, savedCtx := r.env, r.ctx
	for i := 0; i < limit; i++ {
		if template == nil {
			break
		}
		itemEnv := r.env.ItemScope(&items[i])
		itemCtx := &CascadeCtx{Scene: r.ctx.Scene, Mode: r.ctx.Mode,
			Pseudo: map[string]bool{"selected": i == selected}}
		r.env, r.ctx = itemEnv, itemCtx
		node, err := r.resolveNode(template, rn.Rect)
		if err != nil {
			r.env, r.ctx = savedEnv, savedCtx
			return nil, err
		}
		if node != nil {
			stampItem(node, &items[i])
			rn.Children = append(rn.Children, node)
		}
	}
	r.env, r.ctx = savedEnv, savedCtx

	// position the materialised items along the orientation axis
	cursor := 0.0
	for _, it := range rn.Children {
		if horizontal {
			dx := rn.Rect.X + cursor - it.Rect.X
			translate(it, dx, 0)
			cursor += it.Rect.W + gap
		} else {
			dy := rn.Rect.Y + cursor - it.Rect.Y
			translate(it, 0, dy)
			cursor += it.Rect.H + gap
		}
	}

	if summary != nil && limit < len(items) {
		kids, err := r.resolveChildrenOf(rn, summary)
		if err != nil {
			return nil, err
		}
		rn.Children = append(rn.Children, kids...)
	}
	return rn, nil
}

func (r *resolver) resolveChildrenOf(parent *ResolvedNode, n *ksl.Node) ([]*ResolvedNode, error) {
	var out []*ResolvedNode
	for _, c := range r.children(n) {
		rn, err := r.resolveNode(c, parent.Rect)
		if err != nil {
			return nil, err
		}
		if rn != nil {
			out = append(out, rn)
		}
	}
	return out, nil
}

func listItems(v Value) []Item {
	if rv, ok := v.(ResultsVal); ok {
		return rv.Items
	}
	return nil
}

// translate shifts a resolved subtree by (dx, dy).
func translate(n *ResolvedNode, dx, dy float64) {
	n.Rect.X += dx
	n.Rect.Y += dy
	for _, c := range n.Children {
		translate(c, dx, dy)
	}
}

// stampItem records the materialised item on a subtree.
func stampItem(n *ResolvedNode, item *Item) {
	n.Item = item
	for _, c := range n.Children {
		stampItem(c, item)
	}
}
