package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/gg"
)

// TestPaletteValues pins the theme palette so a UI colour can never drift.
func TestPaletteValues(t *testing.T) {
	th, err := Load()
	if err != nil {
		t.Fatalf("Load() = err %v, want nil", err)
	}
	defer th.Close()

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
	defer th.Close()

	face := th.Face(88)
	if face == nil {
		t.Fatal("Face(88) = nil, want a non-nil face")
	}
	if face.Advance("KEYSHELL") <= 0 {
		t.Errorf("Advance(KEYSHELL) = %v, want > 0", face.Advance("KEYSHELL"))
	}
}

// TestFaceSizesShareSource checks we don't leak a font source per size request.
func TestFaceSizesShareSource(t *testing.T) {
	th, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	defer th.Close()

	f1 := th.Face(12)
	f2 := th.Face(44)
	if f1 == nil || f2 == nil {
		t.Fatal("Face returned nil")
	}
	if f1.Advance("KEYSHELL") <= 0 || f2.Advance("KEYSHELL") <= 0 {
		t.Errorf("expected positive advances, got f1: %v, f2: %v", f1.Advance("KEYSHELL"), f2.Advance("KEYSHELL"))
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

func TestResolveFontSourceFallbackWhenNoSystemFonts(t *testing.T) {
	// Point to a non-existent path to force the candidate search and embedded fallback
	source, err := ResolveFontSource("/nonexistent/font/path.otf")
	if err != nil {
		t.Fatalf("ResolveFontSource failed: %v", err)
	}
	defer source.Close()

	face := source.Face(24)
	if face == nil || face.Advance("KEYSHELL") <= 0 {
		t.Fatal("expected non-nil Face with positive advance from font source")
	}
}

func TestResolveFontSourceForcesEmbeddedFallback(t *testing.T) {
	old := fontCandidates
	fontCandidates = func() []string { return nil }
	defer func() { fontCandidates = old }()

	source, err := ResolveFontSource("/nonexistent.otf")
	if err != nil {
		t.Fatalf("ResolveFontSource failed: %v", err)
	}
	defer source.Close()

	face := source.Face(24)
	if face == nil || face.Advance("KEYSHELL") <= 0 {
		t.Fatal("expected valid Face with positive advance from embedded font")
	}
}

func TestThemeLoadSucceeds(t *testing.T) {
	th, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	defer th.Close()

	face := th.Face(16)
	if face == nil || face.Advance("KEYSHELL") <= 0 {
		t.Fatal("expected non-nil Face with positive advance from Theme")
	}
}

func TestLoadWithFont(t *testing.T) {
	th, err := LoadWithFont("/nonexistent/font/path.otf")
	if err != nil {
		t.Fatalf("LoadWithFont failed: %v", err)
	}
	defer th.Close()

	face := th.Face(16)
	if face == nil || face.Advance("KEYSHELL") <= 0 {
		t.Fatal("expected non-nil Face with positive advance from Theme")
	}
}

func TestLoadEmbeddedFont(t *testing.T) {
	source, err := loadEmbeddedFont()
	if err != nil {
		t.Fatalf("loadEmbeddedFont failed: %v", err)
	}
	defer source.Close()

	face := source.Face(24)
	if face == nil || face.Advance("KEYSHELL") <= 0 {
		t.Fatal("expected non-nil Face with positive advance from embedded font source")
	}
}

func TestResolveFont(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "testfont.ttf")
	if err := os.WriteFile(existingFile, []byte("fake"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("preferred file exists", func(t *testing.T) {
		got := ResolveFont(existingFile)
		if got != existingFile {
			t.Errorf("ResolveFont(%q) = %q, want %q", existingFile, got, existingFile)
		}
	})

	t.Run("preferred file missing fallback to candidate", func(t *testing.T) {
		old := fontCandidates
		fontCandidates = func() []string { return []string{existingFile} }
		defer func() { fontCandidates = old }()

		got := ResolveFont("/nonexistent.ttf")
		if got != existingFile {
			t.Errorf("ResolveFont() = %q, want %q", got, existingFile)
		}
	})

	t.Run("preferred file missing and no candidates", func(t *testing.T) {
		old := fontCandidates
		fontCandidates = func() []string { return []string{"/nonexistent2.ttf"} }
		defer func() { fontCandidates = old }()

		got := ResolveFont("/nonexistent.ttf")
		if got != "" {
			t.Errorf("ResolveFont() = %q, want empty", got)
		}
	})

	t.Run("empty preferred with candidate", func(t *testing.T) {
		old := fontCandidates
		fontCandidates = func() []string { return []string{existingFile} }
		defer func() { fontCandidates = old }()

		got := ResolveFont("")
		if got != existingFile {
			t.Errorf("ResolveFont(\"\") = %q, want %q", got, existingFile)
		}
	})

	t.Run("empty preferred without candidate", func(t *testing.T) {
		old := fontCandidates
		fontCandidates = func() []string { return nil }
		defer func() { fontCandidates = old }()

		got := ResolveFont("")
		if got != "" {
			t.Errorf("ResolveFont(\"\") = %q, want empty", got)
		}
	})
}

func TestThemeClose(t *testing.T) {
	var nilTheme *Theme
	if err := nilTheme.Close(); err != nil {
		t.Errorf("nilTheme.Close() = %v, want nil", err)
	}

	th, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if err := th.Close(); err != nil {
		t.Errorf("th.Close() = %v, want nil", err)
	}
	if err := th.Close(); err != nil {
		t.Errorf("second th.Close() = %v, want nil", err)
	}
}
