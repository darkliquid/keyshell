package eval

import (
	"fmt"
	"math"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// Frame is one resolved scene, ready to paint.
type Frame struct {
	Root  *ResolvedNode
	// Nodes is a flat list of all resolved nodes in paint order.
	Nodes []*ResolvedNode
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
	// Selected is true when this node belongs to the active List item.
	Selected bool
	// Text is the populated text string from props, if present.
	Text     string
	// Size is the populated font or element size from props, if present.
	Size     float64
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
	if scene == nil {
		return nil, fmt.Errorf("scene is nil")
	}
	if st.Scale == 0 {
		st.Scale = 1
	}
	// Host tokens are the defaults; a scene `theme` block overrides
	// them (spec §4). User-provided st.Vars overrides scene vars.
	tokens := make(map[string]Color, len(st.Tokens)+len(st.Vars)+8)
	for k, v := range st.Tokens {
		tokens[k] = v
	}
	scVars := sceneVars(scene)
	for k, v := range scVars {
		if c, ok := toColor(v); ok {
			tokens[k] = c
		}
	}
	for k, v := range st.Vars {
		if c, ok := toColor(v); ok {
			tokens[k] = c
		}
	}
	st.Tokens = tokens
	r := &resolver{st: &st, defs: map[string]*ksl.Def{}}
	for _, d := range scene.Defs() {
		r.defs[d.Name] = d
	}
	r.env = NewEnv(&st, nil)
	for k, v := range scVars {
		r.env.Set(k, v)
	}
	for k, v := range st.Vars {
		r.env.Set(k, v)
	}
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
	return &Frame{Root: root, Nodes: []*ResolvedNode{root}}, nil
}

func populateNodeProps(rn *ResolvedNode, props map[string]Value) {
	if s, ok := props["text"].(Str); ok {
		rn.Text = string(s)
	}
	if n, ok := props["size"].(Num); ok {
		rn.Size = float64(n)
	} else if l, ok := props["size"].(Length); ok {
		rn.Size = l.V
	}
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
	populateNodeProps(rn, props)
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
	populateNodeProps(rn, props)
	if err := r.layoutChildrenOf(rn, n); err != nil {
		return nil, err
	}
	return rn, nil
}

func (r *resolver) layoutChildrenOf(rn *ResolvedNode, n *ksl.Node) error {
	// A Group lays its children out in a local frame (0,0,W,H); the
	// painter maps that frame to the surface via the group's transform.
	// All other containers lay out in surface coordinates.
	if n.Name == "Group" {
		local := &ResolvedNode{Name: rn.Name, Rect: RectF{W: rn.Rect.W, H: rn.Rect.H}}
		var err error
		switch n.Name {
		case "Row":
			err = r.layoutFlex(local, n, true)
		case "Column":
			err = r.layoutFlex(local, n, false)
		case "Grid":
			err = r.layoutGrid(local, n)
		case "Stack":
			err = r.layoutStack(local, n)
		default:
			err = r.layoutChildren(local, n)
		}
		if err != nil {
			return err
		}
		rn.Children = append(rn.Children, local.Children...)
		return nil
	}
	return r.layoutChildrenOfSurface(rn, n)
}

func (r *resolver) layoutChildrenOfSurface(rn *ResolvedNode, n *ksl.Node) error {
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

// sceneVars extracts theme tokens and static variables from the scene's theme
// blocks, in declaration order (a later variable overrides an earlier one).
func sceneVars(scene *ksl.Scene) map[string]any {
	out := map[string]any{}
	if scene == nil {
		return out
	}
	for _, th := range scene.ThemeBlocks() {
		for _, pr := range th.Tokens {
			if lit, err := literal(pr.Val); err == nil {
				out[pr.Key] = lit
			}
		}
	}
	return out
}

// sceneTokens pulls theme tokens from the scene's theme blocks, in
// declaration order (a later token overrides an earlier one).
func sceneTokens(scene *ksl.Scene) map[string]Color {
	out := map[string]Color{}
	for k, v := range sceneVars(scene) {
		if c, ok := toColor(v); ok {
			out[k] = c
		}
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
	populateNodeProps(rn, props)

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

	if kwProp(props, "overflow") == "cycle" && kwProp(props, "scroll") == "center" && template != nil && limit > 0 {
		sel := ((selected % limit) + limit) % limit
		savedEnv, savedCtx := r.env, r.ctx

		// First resolve the center template (k=0) to measure item span.
		itemEnv := r.env.ItemScope(&items[sel])
		itemCtx := &CascadeCtx{Scene: r.ctx.Scene, Mode: r.ctx.Mode,
			Pseudo: map[string]bool{"selected": true}}
		r.env, r.ctx = itemEnv, itemCtx
		centerNode, err := r.resolveNode(template, rn.Rect)
		r.env, r.ctx = savedEnv, savedCtx
		if err != nil {
			return nil, err
		}
		if centerNode != nil {
			stampItem(centerNode, &items[sel], true)

			itemSpan := centerNode.Rect.W
			viewportLen := rn.Rect.W
			if !horizontal {
				itemSpan = centerNode.Rect.H
				viewportLen = rn.Rect.H
			}
			if itemSpan <= 0 {
				itemSpan = 100
			}
			stride := itemSpan + gap
			halfSlots := int(math.Ceil((viewportLen/2-itemSpan/2)/stride)) + 1
			if halfSlots < 1 {
				halfSlots = 1
			}

			viewCenter := rn.Rect.X + rn.Rect.W/2
			if !horizontal {
				viewCenter = rn.Rect.Y + rn.Rect.H/2
			}

			slots := make([]*ResolvedNode, 0, halfSlots*2+1)
			for k := -halfSlots; k <= halfSlots; k++ {
				idx := ((sel+k)%limit + limit) % limit
				isSel := (k == 0)
				var node *ResolvedNode
				if isSel {
					node = centerNode
				} else {
					itEnv := r.env.ItemScope(&items[idx])
					itCtx := &CascadeCtx{Scene: r.ctx.Scene, Mode: r.ctx.Mode,
						Pseudo: map[string]bool{"selected": false}}
					r.env, r.ctx = itEnv, itCtx
					n, err := r.resolveNode(template, rn.Rect)
					r.env, r.ctx = savedEnv, savedCtx
					if err != nil {
						return nil, err
					}
					node = n
					if node != nil {
						stampItem(node, &items[idx], false)
					}
				}
				if node != nil {
					slotCenter := viewCenter + float64(k)*stride
					if horizontal {
						targetX := slotCenter - node.Rect.W/2
						translate(node, targetX-node.Rect.X, rn.Rect.Y-node.Rect.Y)
					} else {
						targetY := slotCenter - node.Rect.H/2
						translate(node, rn.Rect.X-node.Rect.X, targetY-node.Rect.Y)
					}
					slots = append(slots, node)
				}
			}
			rn.Children = append(rn.Children, slots...)
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

	// When there are many items, virtualize the list so only the visible items
	// are resolved and materialized, avoiding thousands of unnecessary AST allocations.
	if limit > 20 && template != nil && (kwProp(props, "scroll") == "reveal" || kwProp(props, "scroll") == "center" || kwProp(props, "overflow") == "clamp") {
		sel := selected
		if sel < 0 {
			sel = 0
		}
		if sel >= limit {
			sel = limit - 1
		}
		savedEnv, savedCtx := r.env, r.ctx
		itemEnv := r.env.ItemScope(&items[sel])
		itemCtx := &CascadeCtx{Scene: r.ctx.Scene, Mode: r.ctx.Mode,
			Pseudo: map[string]bool{"selected": true}}
		r.env, r.ctx = itemEnv, itemCtx
		sampleNode, err := r.resolveNode(template, rn.Rect)
		r.env, r.ctx = savedEnv, savedCtx
		if err != nil {
			return nil, err
		}
		itemSpan := sampleNode.Rect.H
		viewportLen := rn.Rect.H
		if horizontal {
			itemSpan = sampleNode.Rect.W
			viewportLen = rn.Rect.W
		}
		if itemSpan <= 0 {
			itemSpan = 44
		}
		stride := itemSpan + gap
		contentLen := float64(limit)*stride - gap

		shift := 0.0
		scrollMode := kwProp(props, "scroll")
		switch scrollMode {
		case "center":
			viewCenter := viewportLen / 2
			selCenter := float64(sel)*stride + itemSpan/2
			shift = viewCenter - selCenter
		case "reveal":
			selStart := float64(sel) * stride
			selEnd := selStart + itemSpan
			if selEnd > viewportLen {
				shift = viewportLen - selEnd
			}
			if selStart+shift < 0 {
				shift = -selStart
			}
		}

		if contentLen > viewportLen && shift != 0 {
			if shift > 0 {
				shift = 0
			}
			minShift := -(contentLen - viewportLen)
			if shift < minShift {
				shift = minShift
			}
		}

		first := int(math.Floor(-shift / stride))
		if first < 0 {
			first = 0
		}
		last := int(math.Ceil((viewportLen - shift) / stride))
		if last >= limit {
			last = limit - 1
		}

		for i := first; i <= last; i++ {
			isSel := (i == sel)
			var node *ResolvedNode
			if isSel && sampleNode != nil {
				node = sampleNode
				stampItem(node, &items[i], true)
			} else {
				itEnv := r.env.ItemScope(&items[i])
				itCtx := &CascadeCtx{Scene: r.ctx.Scene, Mode: r.ctx.Mode,
					Pseudo: map[string]bool{"selected": false}}
				r.env, r.ctx = itEnv, itCtx
				n, err := r.resolveNode(template, rn.Rect)
				r.env, r.ctx = savedEnv, savedCtx
				if err != nil {
					return nil, err
				}
				node = n
				if node != nil {
					stampItem(node, &items[i], false)
				}
			}
			if node != nil {
				pos := float64(i)*stride + shift
				if horizontal {
					translate(node, rn.Rect.X+pos-node.Rect.X, rn.Rect.Y-node.Rect.Y)
				} else {
					translate(node, rn.Rect.X-node.Rect.X, rn.Rect.Y+pos-node.Rect.Y)
				}
				rn.Children = append(rn.Children, node)
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
			stampItem(node, &items[i], i == selected)
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

	// scroll: center keeps the selected item centred in the viewport;
	// scroll: reveal ensures the selected item is completely visible in the viewport.
	scrollMode := kwProp(props, "scroll")
	if (scrollMode == "center" || scrollMode == "reveal") && len(rn.Children) > 0 {
		sel := selected
		if sel < 0 {
			sel = 0
		}
		if sel >= len(rn.Children) {
			sel = len(rn.Children) - 1
		}
		var contentStart, contentEnd float64
		if horizontal {
			contentStart = rn.Children[0].Rect.X
			last := rn.Children[len(rn.Children)-1]
			contentEnd = last.Rect.X + last.Rect.W
		} else {
			contentStart = rn.Children[0].Rect.Y
			last := rn.Children[len(rn.Children)-1]
			contentEnd = last.Rect.Y + last.Rect.H
		}
		selNode := rn.Children[sel]
		var selCenter, viewCenter, contentLen float64
		var viewportLen, viewStart, viewEnd, itemStart, itemEnd float64
		if horizontal {
			selCenter = selNode.Rect.X + selNode.Rect.W/2
			viewCenter = rn.Rect.X + rn.Rect.W/2
			viewStart = rn.Rect.X
			viewEnd = rn.Rect.X + rn.Rect.W
			itemStart = selNode.Rect.X
			itemEnd = selNode.Rect.X + selNode.Rect.W
			contentLen = contentEnd - contentStart
			viewportLen = rn.Rect.W
		} else {
			selCenter = selNode.Rect.Y + selNode.Rect.H/2
			viewCenter = rn.Rect.Y + rn.Rect.H/2
			viewStart = rn.Rect.Y
			viewEnd = rn.Rect.Y + rn.Rect.H
			itemStart = selNode.Rect.Y
			itemEnd = selNode.Rect.Y + selNode.Rect.H
			contentLen = contentEnd - contentStart
			viewportLen = rn.Rect.H
		}

		shift := 0.0
		switch scrollMode {
		case "center":
			shift = viewCenter - selCenter
		case "reveal":
			if itemStart < viewStart {
				shift = viewStart - itemStart
			} else if itemEnd > viewEnd {
				shift = viewEnd - itemEnd
			}
		}

		// clamp so the selected item stays in view and content doesn't overscroll
		if contentLen > viewportLen && shift != 0 {
			if shift > 0 {
				shift = 0
			}
			minShift := -(contentLen - viewportLen)
			if shift < minShift {
				shift = minShift
			}
		}
		for _, it := range rn.Children {
			if horizontal {
				translate(it, shift, 0)
			} else {
				translate(it, 0, shift)
			}
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
// Group children live in the group's local (0,0,W,H) coordinate frame
// (applied at paint time via paintThrough), so only the Group's own
// surface rect moves.
func translate(n *ResolvedNode, dx, dy float64) {
	n.Rect.X += dx
	n.Rect.Y += dy
	if n.Name == "Group" {
		return
	}
	for _, c := range n.Children {
		translate(c, dx, dy)
	}
}

// stampItem records the materialised item and selection status on a subtree.
func stampItem(n *ResolvedNode, item *Item, selected bool) {
	n.Item = item
	n.Selected = selected
	for _, c := range n.Children {
		stampItem(c, item, selected)
	}
}
