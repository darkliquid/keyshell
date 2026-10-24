package eval

import "testing"

func TestFrameContains_AxisAlignedBox(t *testing.T) {
	frame := &Frame{
		Nodes: []*ResolvedNode{
			{
				Name: "Box",
				Rect: RectF{X: 640, Y: 120, W: 640, H: 460},
			},
		},
	}

	if !frame.Contains(960, 300) {
		t.Error("expected inside box point (960, 300) to return true")
	}
	if frame.Contains(100, 100) {
		t.Error("expected outside box point (100, 100) to return false")
	}
}

func TestFrameContains_PathPolygon(t *testing.T) {
	frame := &Frame{
		Nodes: []*ResolvedNode{
			{
				Name: "Path",
				Props: map[string]Value{
					"d": Str("M 0 100 L 1920 120 L 1920 300 L 0 280 Z"),
				},
			},
		},
	}

	if !frame.Contains(960, 200) {
		t.Error("expected inside path polygon point (960, 200) to return true")
	}
	if frame.Contains(960, 50) {
		t.Error("expected point above polygon (960, 50) to return false")
	}
	if frame.Contains(960, 400) {
		t.Error("expected point below polygon (960, 400) to return false")
	}
}

func TestFrameContains_RotatedGroup(t *testing.T) {
	// Center is (200, 150). Rotated 90 degrees.
	// Point (200, 240) rotated by -90 deg maps to (290, 150), which is within [100..300, 100..200].
	// Point (100, 150) rotated by -90 deg maps to (200, 250), which is outside [100..300, 100..200].
	frame := &Frame{
		Nodes: []*ResolvedNode{
			{
				Name: "Group",
				Rect: RectF{X: 100, Y: 100, W: 200, H: 100},
				Props: map[string]Value{
					"rotate": Length{V: 90, Unit: "deg"},
				},
			},
		},
	}

	if !frame.Contains(200, 150) {
		t.Error("expected center of rotated group (200, 150) to return true")
	}
	if !frame.Contains(200, 240) {
		t.Error("expected rotated point (200, 240) to return true")
	}
	if frame.Contains(100, 150) {
		t.Error("expected unrotated-only point (100, 150) to return false")
	}
}

func TestFrameContains_WindowMaskBand(t *testing.T) {
	// Window has surface size (1920x1080) and mask: band.
	// Hit testing must delegate to its children rather than testing Window's 1920x1080 rect.
	root := &ResolvedNode{
		Name: "Window",
		Rect: RectF{X: 0, Y: 0, W: 1920, H: 1080},
		Props: map[string]Value{
			"mask": Kw("band"),
		},
		Children: []*ResolvedNode{
			{
				Name: "Path",
				Props: map[string]Value{
					"d": Str("M 0 100 L 1920 120 L 1920 300 L 0 280 Z"),
				},
			},
		},
	}
	frame := &Frame{
		Root:  root,
		Nodes: []*ResolvedNode{root, root.Children[0]},
	}

	if !frame.Contains(960, 200) {
		t.Error("expected point inside child band path to return true")
	}
	if frame.Contains(100, 50) {
		t.Error("expected point outside child band (scrim) to return false")
	}
}

func TestFrameContains_WindowExplicitDims(t *testing.T) {
	root := &ResolvedNode{
		Name: "Window",
		Rect: RectF{X: 640, Y: 120, W: 640, H: 460},
		Props: map[string]Value{
			"width":  Num(640),
			"height": Num(460),
		},
	}
	frame := &Frame{
		Root:  root,
		Nodes: []*ResolvedNode{root},
	}

	if !frame.Contains(960, 300) {
		t.Error("expected point inside explicit Window rect to return true")
	}
	if frame.Contains(100, 100) {
		t.Error("expected point outside explicit Window rect to return false")
	}
}

func TestFrameContains_EmptyFrame(t *testing.T) {
	var empty *Frame
	if empty.Contains(100, 100) {
		t.Error("expected nil frame to return false")
	}

	zeroFrame := &Frame{}
	if zeroFrame.Contains(100, 100) {
		t.Error("expected frame with no nodes to return false")
	}
}
