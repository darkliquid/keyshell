package eval

import (
	"math"
	"strconv"
	"strings"
)

// Contains reports whether (x, y) falls inside any rendered top-level window,
// box, panel, or polygon in the frame.
func (f *Frame) Contains(x, y float64) bool {
	if f == nil {
		return false
	}
	if f.Root != nil {
		return nodeContains(f.Root, x, y)
	}
	for _, n := range f.Nodes {
		if nodeContains(n, x, y) {
			return true
		}
	}
	return false
}

// nodeContains tests whether (x, y) falls inside node n or its descendants.
func nodeContains(n *ResolvedNode, x, y float64) bool {
	if n == nil {
		return false
	}

	if n.Name == "Path" {
		if n.Props != nil {
			if dVal, ok := n.Props["d"]; ok {
				if s, ok := dVal.(Str); ok && s != "" {
					pts := parsePolygonPoints(string(s))
					if len(pts) >= 3 && pointInPolygon(pts, x, y) {
						return true
					}
				}
			}
		}
		for _, child := range n.Children {
			if nodeContains(child, x, y) {
				return true
			}
		}
		return false
	}

	if n.Name == "Window" {
		if isBandMask(n) || !hasExplicitDims(n) {
			for _, child := range n.Children {
				if nodeContains(child, x, y) {
					return true
				}
			}
			return false
		}
		if inRect(n.Rect, x, y) {
			return true
		}
		for _, child := range n.Children {
			if nodeContains(child, x, y) {
				return true
			}
		}
		return false
	}

	if n.Name == "Group" {
		cx := n.Rect.X + n.Rect.W/2
		cy := n.Rect.Y + n.Rect.H/2
		dx := x - cx
		dy := y - cy
		if rotVal, ok := n.Props["rotate"]; ok {
			var angleRad float64
			switch v := rotVal.(type) {
			case Length:
				angleRad = v.V * math.Pi / 180
			case Num:
				angleRad = float64(v) * math.Pi / 180
			}
			if angleRad != 0 {
				theta := -angleRad
				cosT := math.Cos(theta)
				sinT := math.Sin(theta)
				rx := dx*cosT - dy*sinT
				ry := dx*sinT + dy*cosT
				dx, dy = rx, ry
			}
		}
		lx := dx + n.Rect.W/2
		ly := dy + n.Rect.H/2
		if n.Rect.W > 0 && n.Rect.H > 0 {
			if lx < 0 || lx > n.Rect.W || ly < 0 || ly > n.Rect.H {
				return false
			}
		}
		for _, child := range n.Children {
			if nodeContains(child, lx, ly) {
				return true
			}
		}
		return n.Rect.W > 0 && n.Rect.H > 0
	}

	if inRect(n.Rect, x, y) {
		return true
	}
	for _, child := range n.Children {
		if nodeContains(child, x, y) {
			return true
		}
	}
	return false
}

func isBandMask(n *ResolvedNode) bool {
	if n.Props == nil {
		return false
	}
	m, ok := n.Props["mask"]
	if !ok {
		return false
	}
	switch v := m.(type) {
	case Kw:
		return v == "band"
	case Str:
		return v == "band"
	default:
		return false
	}
}

func hasExplicitDims(n *ResolvedNode) bool {
	if n.Props == nil {
		return false
	}
	_, hasW := n.Props["width"]
	if !hasW {
		_, hasW = n.Props["w"]
	}
	_, hasH := n.Props["height"]
	if !hasH {
		_, hasH = n.Props["h"]
	}
	return hasW || hasH
}

func inRect(r RectF, x, y float64) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H
}

func pointInPolygon(pts [][2]float64, x, y float64) bool {
	n := len(pts)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := pts[i][0], pts[i][1]
		xj, yj := pts[j][0], pts[j][1]

		if ((yi > y) != (yj > y)) && (x < (xj-xi)*(y-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

func parsePolygonPoints(d string) [][2]float64 {
	var pts [][2]float64
	var currX, currY float64

	var sb strings.Builder
	for i := 0; i < len(d); i++ {
		c := d[i]
		if isCmdChar(c) {
			sb.WriteByte(' ')
			sb.WriteByte(c)
			sb.WriteByte(' ')
		} else if c == ',' {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(c)
		}
	}

	tokens := strings.Fields(sb.String())
	var cmd string
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if len(tok) == 1 && isCmdChar(tok[0]) {
			cmd = tok
			i++
		}

		switch cmd {
		case "M":
			if i+1 < len(tokens) {
				x, errX := strconv.ParseFloat(tokens[i], 64)
				y, errY := strconv.ParseFloat(tokens[i+1], 64)
				if errX == nil && errY == nil {
					currX = x
					currY = y
					pts = append(pts, [2]float64{currX, currY})
				}
				i += 2
				cmd = "L"
			} else {
				i++
			}
		case "m":
			if i+1 < len(tokens) {
				x, errX := strconv.ParseFloat(tokens[i], 64)
				y, errY := strconv.ParseFloat(tokens[i+1], 64)
				if errX == nil && errY == nil {
					currX += x
					currY += y
					pts = append(pts, [2]float64{currX, currY})
				}
				i += 2
				cmd = "l"
			} else {
				i++
			}
		case "L":
			if i+1 < len(tokens) {
				x, errX := strconv.ParseFloat(tokens[i], 64)
				y, errY := strconv.ParseFloat(tokens[i+1], 64)
				if errX == nil && errY == nil {
					currX = x
					currY = y
					pts = append(pts, [2]float64{currX, currY})
				}
				i += 2
			} else {
				i++
			}
		case "l":
			if i+1 < len(tokens) {
				x, errX := strconv.ParseFloat(tokens[i], 64)
				y, errY := strconv.ParseFloat(tokens[i+1], 64)
				if errX == nil && errY == nil {
					currX += x
					currY += y
					pts = append(pts, [2]float64{currX, currY})
				}
				i += 2
			} else {
				i++
			}
		case "H":
			if i < len(tokens) {
				x, err := strconv.ParseFloat(tokens[i], 64)
				if err == nil {
					currX = x
					pts = append(pts, [2]float64{currX, currY})
				}
				i++
			}
		case "h":
			if i < len(tokens) {
				x, err := strconv.ParseFloat(tokens[i], 64)
				if err == nil {
					currX += x
					pts = append(pts, [2]float64{currX, currY})
				}
				i++
			}
		case "V":
			if i < len(tokens) {
				y, err := strconv.ParseFloat(tokens[i], 64)
				if err == nil {
					currY = y
					pts = append(pts, [2]float64{currX, currY})
				}
				i++
			}
		case "v":
			if i < len(tokens) {
				y, err := strconv.ParseFloat(tokens[i], 64)
				if err == nil {
					currY += y
					pts = append(pts, [2]float64{currX, currY})
				}
				i++
			}
		case "Z", "z":
			cmd = ""
		default:
			i++
		}
	}

	if len(pts) > 3 && pts[len(pts)-1] == pts[0] {
		pts = pts[:len(pts)-1]
	}
	return pts
}

func isCmdChar(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'Z', 'z', 'H', 'h', 'V', 'v':
		return true
	}
	return false
}
