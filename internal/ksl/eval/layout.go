package eval

import (
	"math"
	"strings"

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

func valueText(v Value, uppercase bool) string {
	var s string
	switch x := v.(type) {
	case Str:
		s = string(x)
	case Num:
		s = stringify(x)
	case Kw:
		s = string(x)
	default:
		return ""
	}
	if uppercase {
		s = strings.ToUpper(s)
	}
	return s
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
	upper, _ := props["uppercase"].(Bool)
	text := valueText(props["text"], bool(upper))
	if text == "" {
		return 0, h
	}
	weight := ""
	if kw, ok := props["weight"].(Kw); ok {
		weight = string(kw)
	}
	w, _ = m.TextSize(text, size, weight)
	return w, h
}

// measurePill sizes a Pill badge (uppercase bold label + 16px horizontal padding, 28px height).
func measurePill(m Measurer, props map[string]Value, st *State) (w, h float64) {
	scale := st.Scale
	if scale == 0 {
		scale = 1
	}
	size := 12.0 * scale
	if s, ok := props["size"]; ok {
		size = resolveLength(s, st, false)
	}
	h = 28.0 * scale
	if m == nil {
		return 0, h
	}
	text := valueText(props["text"], true)
	if text == "" {
		return 0, h
	}
	tw, _ := m.TextSize(text, size, "bold")
	return tw + 16*scale, h
}

// nodeRect resolves one node's rect from its props and the parent
// bounds.
func (r *resolver) nodeRect(n *ksl.Node, props map[string]Value, bounds RectF) RectF {
	return r.refineRect(n, props, bounds)
}

// refineRect applies a node's own geometry props to a rect a container
// has already assigned: explicit x/y/width/height opt out of flow,
// leaf primitives size to their content when unconstrained, and constraints clamp.
func (r *resolver) refineRect(n *ksl.Node, props map[string]Value, rect RectF) RectF {
	if v, ok := props["x"]; ok {
		rect.X += resolveLength(v, r.st, true)
	}
	if v, ok := props["y"]; ok {
		rect.Y += resolveLength(v, r.st, false)
	}
	_, hasW := props["width"]
	_, hasH := props["height"]
	if v, ok := props["width"]; ok {
		rect.W = resolveLength(v, r.st, true)
	}
	if v, ok := props["height"]; ok {
		rect.H = resolveLength(v, r.st, false)
	}
	iw, ih := r.leafIntrinsicSize(n, props)
	if !hasW && iw > 0 {
		if rect.W > 0 {
			rect.W = math.Min(rect.W, iw)
		} else {
			rect.W = iw
		}
	} else if !hasW && n.Name == "Text" {
		if rect.W > 0 {
			rect.W = math.Min(rect.W, iw)
		} else {
			rect.W = iw
		}
	}
	if !hasH && ih > 0 {
		rect.H = ih
	}
	return applyConstraints(rect, props, r.st)
}

func (r *resolver) leafIntrinsicSize(n *ksl.Node, props map[string]Value) (w, h float64) {
	scale := r.st.Scale
	if scale == 0 {
		scale = 1
	}
	switch n.Name {
	case "Text":
		return measureText(r.measurer, props, r.st)
	case "Pill":
		return measurePill(r.measurer, props, r.st)
	case "Cursor":
		return 3 * scale, 24 * scale
	}
	return 0, 0
}

func (r *resolver) flexChildSize(n *ksl.Node, props map[string]Value, parentHorizontal bool) (main, cross float64) {
	w := explicitLength(props, "width", r.st, true)
	h := explicitLength(props, "height", r.st, false)
	_, hasW := props["width"]
	_, hasH := props["height"]
	iw, ih := r.leafIntrinsicSize(n, props)
	if !hasW && (iw > 0 || n.Name == "Text") {
		w = iw
	}
	if !hasH && ih > 0 {
		h = ih
	}
	// Nested containers take the intrinsic sizes of their children when not explicitly sized.
	if numProp(props, "grow") == 0 {
		if n.Name == "Row" && !hasW {
			w = r.containerIntrinsicMain(n, props, true)
		} else if n.Name == "Column" && !hasW {
			w = r.containerIntrinsicCross(n, props, false)
		}
		if n.Name == "Column" && !hasH {
			h = r.containerIntrinsicMain(n, props, false)
		} else if n.Name == "Row" && !hasH {
			h = r.containerIntrinsicCross(n, props, true)
		}
	}
	constrained := applyConstraints(RectF{W: w, H: h}, props, r.st)
	if w > 0 {
		w = constrained.W
	}
	if h > 0 {
		h = constrained.H
	}
	if parentHorizontal {
		return w, h
	}
	return h, w
}

func (r *resolver) containerIntrinsicMain(n *ksl.Node, props map[string]Value, horizontal bool) float64 {
	gap := numProp(props, "gap")
	total := 0.0
	count := 0
	for _, c := range r.children(n) {
		cp, ok, err := resolveProps(c, r.env, r.ctx)
		if err != nil || !ok {
			continue
		}
		m, _ := r.flexChildSize(c, cp, horizontal)
		total += m
		count++
	}
	if count > 1 {
		total += gap * float64(count-1)
	}
	return total
}

func (r *resolver) containerIntrinsicCross(n *ksl.Node, props map[string]Value, horizontal bool) float64 {
	maxCross := 0.0
	for _, c := range r.children(n) {
		cp, ok, err := resolveProps(c, r.env, r.ctx)
		if err != nil || !ok {
			continue
		}
		_, cross := r.flexChildSize(c, cp, horizontal)
		if cross > maxCross {
			maxCross = cross
		}
	}
	return maxCross
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
	for _, c := range r.children(n) {
		cp, ok, err := resolveProps(c, r.env, r.ctx)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		main, cross := r.flexChildSize(c, cp, horizontal)
		grow := numProp(cp, "grow")
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
			if crossSize > 0 && crossOut > crossSize {
				crossOut = crossSize
			}
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
	for i, c := range r.children(n) {
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
	for _, c := range r.children(n) {
		cp, ok, err := resolveProps(c, r.env, r.ctx)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		w := explicitLength(cp, "width", r.st, true)
		h := explicitLength(cp, "height", r.st, false)
		iw, ih := r.leafIntrinsicSize(c, cp)
		if w == 0 && iw > 0 {
			w = iw
		}
		if h == 0 && ih > 0 {
			h = ih
		}
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
	for _, c := range r.children(n) {
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
