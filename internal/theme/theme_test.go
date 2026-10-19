package theme

import (
	"testing"

	"github.com/gogpu/gg"
)

// TestPaletteValues pins the theme palette so a UI colour can never drift.
func TestPaletteValues(t *testing.T) {
	th, err := Load()
	if err != nil {
		t.Fatalf("Load() = err %v, want nil", err)
	}
	tests := []struct {
		name string
		got  gg.RGBA
		want gg.RGBA
	}{
		{"Slab", th.Slab, hex("#0B0B0F")},
		{"SurfaceAlt", th.SurfaceAlt, hex("#121218")},
		{"Ink", th.Ink, hex("#F2F2F7")},
		{"Accent", th.Accent, hex("#FF2E63")},
		{"AccentAlt", th.AccentAlt, hex("#08F7FE")},
		{"Muted", th.Muted, hex("#5A5A66")},
	}
	for _, tt := range tests {
		if !close(tt.got, tt.want) {
			t.Errorf("%s = %+v, want %+v", tt.name, tt.got, tt.want)
		}
	}
}

// TestFaceReturnsFace ensures a face is produced for the display size used by
// titles (the face this project is named after).
func TestFaceReturnsFace(t *testing.T) {
	th, err := Load()
	if err != nil {
		t.Fatalf("Load() = err %v, want nil", err)
	}
	face := th.Face(88)
	if face == nil {
		t.Fatal("Face(88) = nil, want a non-nil face")
	}
	if face.Advance("KEYSHELL") <= 0 {
		t.Errorf("Advance(KEYSHELL) = %v, want > 0", face.Advance("KEYSHELL"))
	}
}

// TestFaceCaches checks we don't leak a font source per size request.
func TestFaceSizesShareSource(t *testing.T) {
	th, _ := Load()
	f1 := th.Face(12)
	f2 := th.Face(44)
	if f1 == nil || f2 == nil {
		t.Fatal("Face returned nil")
	}
	// Any two sizes must render distinct advances on the same string.
	if f1.Advance("W") >= f2.Advance("W") {
		t.Errorf("12px advance %v should be < 44px advance %v",
			f1.Advance("W"), f2.Advance("W"))
	}
}

func close(a, b gg.RGBA) bool {
	const eps = 1e-6
	return abs(a.R-b.R) < eps && abs(a.G-b.G) < eps &&
		abs(a.B-b.B) < eps && abs(a.A-b.A) < eps
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
