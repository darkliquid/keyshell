package eval

import (
	"math"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// RectF is a resolved rectangle in window pixels.
type RectF struct{ X, Y, W, H float64 }

// Measurer measures text without the graphics stack; the renderer
// injects the real one (theme.Face-based).
type Measurer interface {
	TextSize(text string, size float64, weight string) (w, h float64)
}

// resolveLength turns a Length/Num into absolute pixels: % of the
// Window (spec §2), px and unitless scaled by the scene scale.
func resolveLength(v Value, st *State, horizontal bool) float64 {
	scale := st.Scale
	if scale == 0 {
		scale = 1
	}
	switch x := v.(type) {
	case Length:
		switch x.Unit {
		case "%":
			if horizontal {
				return x.V / 100 * st.Width
			}
			return x.V / 100 * st.Height
		case "px", "":
			return x.V * scale
		}
	case Num:
		return float64(x) * scale
	}
	return 0
}

// measureText sizes Text through the injected measurer.
func measureText(m Measurer, props map[string]Value, st *State) (w, h float64) {
	size := 12.0
	if s, ok := props["size"]; ok {
		size = resolveLength(s, st, false)
	}
	h = size
	if m == nil {
		return 0, h
	}
	text, _ := props["text"].(Str)
	weight := ""
	if kw, ok := props["weight"].(Kw); ok {
		weight = string(kw)
	}
	w, _ = m.TextSize(string(text), size, weight)
	return w, h
}

// nodeRect resolves one node's rect from its props and the parent
// bounds.
func (r *resolver) nodeRect(n *ksl.Node, props map[string]Value, bounds RectF) RectF {
	return r.refineRect(n, props, bounds)
}

// refineRect applies a node's own geometry props to a rect a container
// has already assigned: explicit x/y/width/height opt out of flow,
// Text sizes to its content when unconstrained, and constraints clamp.
func (r *resolver) refineRect(n *ksl.Node, props map[string]Value, rect RectF) RectF {
	if v, ok := props["x"]; ok {
		rect.X = resolveLength(v, r.st, true)
	}
	if v, ok := props["y"]; ok {
		rect.Y = resolveLength(v, r.st, false)
	}
	_, hasW := props["width"]
	_, hasH := props["height"]
	if v, ok := props["width"]; ok {
		rect.W = resolveLength(v, r.st, true)
	}
	if v, ok := props["height"]; ok {
		rect.H = resolveLength(v, r.st, false)
	}
	if n.Name == "Text" {
		tw, th := measureText(r.measurer, props, r.st)
		if !hasW {
			rect.W = tw
		}
		if !hasH {
			rect.H = th
		}
	}
	return applyConstraints(rect, props, r.st)
}

func applyConstraints(rect RectF, props map[string]Value, st *State) RectF {
	if v, ok := props["minW"]; ok {
		rect.W = math.Max(rect.W, resolveLength(v, st, true))
	}
	if v, ok := props["maxW"]; ok {
		rect.W = math.Min(rect.W, resolveLength(v, st, true))
	}
	if v, ok := props["minH"]; ok {
		rect.H = math.Max(rect.H, resolveLength(v, st, false))
	}
	if v, ok := props["maxH"]; ok {
		rect.H = math.Min(rect.H, resolveLength(v, st, false))
	}
	if v, ok := props["aspect"]; ok && rect.W > 0 {
		if a, ok := v.(Num); ok && float64(a) > 0 {
			rect.H = rect.W / float64(a)
		}
	}
	return rect
}

// numProp reads a numeric prop with a zero default.
func numProp(props map[string]Value, key string) float64 {
	if v, ok := props[key]; ok {
		if n, ok := v.(Num); ok {
			return float64(n)
		}
	}
	return 0
}

func kwProp(props map[string]Value, key string) string {
	if v, ok := props[key].(Kw); ok {
		return string(v)
	}
	if v, ok := props[key].(Str); ok {
		return string(v)
	}
	return ""
}

// layoutFlex lays Row (horizontal) or Column children out: fixed main
// sizes first, grow children share the leftover, gap spaces them, and
// align/justify position cross/main.
func (r *resolver) layoutFlex(parent *ResolvedNode, n *ksl.Node, horizontal bool) error {
	props := parent.Props
	gap := numProp(props, "gap")
	crossAlign := kwProp(props, "align")
	mainJustify := kwProp(props, "justify")

	mainSize := parent.Rect.H
	crossSize := parent.Rect.W
	if horizontal {
		mainSize, crossSize = parent.Rect.W, parent.Rect.H
	}

	type childSpec struct {
		n      *ksl.Node
		props  map[string]Value
		main   float64
		cross  float64
		grow   float64
		exists bool
	}

	specs := make([]childSpec, 0, len(n.Children))
	fixed := 0.0
	growTotal := 0.0
	count := 0
	for _, c := range n.Children {
		cp, ok, err := resolveProps(c, r.env, r.ctx)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		var main, cross, grow float64
		if horizontal {
			main = explicitLength(cp, "width", r.st, true)
			cross = explicitLength(cp, "height", r.st, false)
		} else {
			main = explicitLength(cp, "height", r.st, false)
			cross = explicitLength(cp, "width", r.st, true)
		}
		grow = numProp(cp, "grow")
		if grow > 0 {
			growTotal += grow
		} else {
			fixed += main
		}
		specs = append(specs, childSpec{n: c, props: cp, main: main, cross: cross, grow: grow, exists: true})
		count++
	}
	if count == 0 {
		return nil
	}
	content := fixed + gap*float64(count-1)
	leftover := math.Max(0, mainSize-content)
	offset := 0.0
	switch mainJustify {
	case "center":
		offset = leftover / 2
	case "end":
		offset = leftover
	}
	start := offset
	if horizontal {
		start += parent.Rect.X
	} else {
		start += parent.Rect.Y
	}

	for i := range specs {
		s := &specs[i]
		mainOut := s.main
		if s.grow > 0 && growTotal > 0 {
			mainOut = leftover * s.grow / growTotal
		}
		crossOut := s.cross
		crossOffset := 0.0
		if crossOut == 0 {
			crossOut = crossSize // stretch
		} else {
			switch crossAlign {
			case "center":
				crossOffset = (crossSize - crossOut) / 2
			case "end":
				crossOffset = crossSize - crossOut
			}
		}
		var rect RectF
		if horizontal {
			rect = RectF{X: start, Y: parent.Rect.Y + crossOffset, W: mainOut, H: crossOut}
			start += mainOut + gap
		} else {
			rect = RectF{X: parent.Rect.X + crossOffset, Y: start, W: crossOut, H: mainOut}
			start += mainOut + gap
		}
		rn, err := r.resolveNodeRect(s.n, rect)
		if err != nil {
			return err
		}
		if rn != nil {
			parent.Children = append(parent.Children, rn)
		}
	}
	return nil
}

// explicitLength returns a node's explicit px length for a dimension,
// or 0 when it is unset.
func explicitLength(props map[string]Value, key string, st *State, horizontal bool) float64 {
	if v, ok := props[key]; ok {
		return resolveLength(v, st, horizontal)
	}
	return 0
}

// layoutGrid tiles children into cols columns, row-major, uniform gap.
func (r *resolver) layoutGrid(parent *ResolvedNode, n *ksl.Node) error {
	cols := int(numProp(parent.Props, "cols"))
	if cols <= 0 {
		return nil
	}
	gap := numProp(parent.Props, "gap")
	rows := (len(n.Children) + cols - 1) / cols
	cellW := (parent.Rect.W - gap*float64(cols-1)) / float64(cols)
	cellH := (parent.Rect.H - gap*float64(rows-1)) / float64(rows)
	for i, c := range n.Children {
		col := i % cols
		row := i / cols
		rect := RectF{
			X: parent.Rect.X + float64(col)*(cellW+gap),
			Y: parent.Rect.Y + float64(row)*(cellH+gap),
			W: cellW,
			H: cellH,
		}
		rn, err := r.resolveNodeRect(c, rect)
		if err != nil {
			return err
		}
		if rn != nil {
			parent.Children = append(parent.Children, rn)
		}
	}
	return nil
}

// layoutStack overlays children in declaration order: align pins, x/y
// offsets, explicit sizes, else the parent's rect. z stays on the node
// for the painter to order.
func (r *resolver) layoutStack(parent *ResolvedNode, n *ksl.Node) error {
	align := kwProp(parent.Props, "align")
	for _, c := range n.Children {
		cp, ok, err := resolveProps(c, r.env, r.ctx)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		w := explicitLength(cp, "width", r.st, true)
		h := explicitLength(cp, "height", r.st, false)
		if w == 0 {
			w = parent.Rect.W
		}
		if h == 0 {
			h = parent.Rect.H
		}
		x, y := parent.Rect.X, parent.Rect.Y
		switch align {
		case "center":
			x += (parent.Rect.W - w) / 2
			y += (parent.Rect.H - h) / 2
		case "end":
			x += parent.Rect.W - w
			y += parent.Rect.H - h
		}
		if v, ok := cp["x"]; ok {
			x = resolveLength(v, r.st, true)
		}
		if v, ok := cp["y"]; ok {
			y = resolveLength(v, r.st, false)
		}
		rn, err := r.resolveNodeRect(c, RectF{X: x, Y: y, W: w, H: h})
		if err != nil {
			return err
		}
		if rn != nil {
			parent.Children = append(parent.Children, rn)
		}
	}
	return nil
}

// layoutChildren is the fallback: children layered in the parent rect.
func (r *resolver) layoutChildren(parent *ResolvedNode, n *ksl.Node) error {
	for _, c := range n.Children {
		rn, err := r.resolveNodeRect(c, parent.Rect)
		if err != nil {
			return err
		}
		if rn != nil {
			parent.Children = append(parent.Children, rn)
		}
	}
	return nil
}
