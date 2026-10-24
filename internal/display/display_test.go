package display

import "testing"

func TestPrimaryReturnsNonZeroDimensions(t *testing.T) {
	w, h := Primary()
	if w <= 0 || h <= 0 {
		t.Fatalf("Primary() = (%d, %d), expected positive dimensions", w, h)
	}
}

func TestFallbackConstants(t *testing.T) {
	if FallbackWidth != 1920 {
		t.Errorf("expected FallbackWidth == 1920, got %d", FallbackWidth)
	}
	if FallbackHeight != 1080 {
		t.Errorf("expected FallbackHeight == 1080, got %d", FallbackHeight)
	}
}
