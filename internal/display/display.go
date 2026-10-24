// Package display resolves the primary output's logical size so the overlay
// can cover it. It is compositor-aware and degrades to a sane default: an
// overlay sized wrong is worse than one sized by convention.
package display

// FallbackWidth and FallbackHeight are default dimensions when resolution detection fails.
const (
	FallbackWidth  = 1920
	FallbackHeight = 1080
)

// Primary returns the primary output's logical size, falling back to 1920x1080 if undetermined.
func Primary() (int, int) {
	if w, h := platformPrimary(); w > 0 && h > 0 {
		return w, h
	}
	return FallbackWidth, FallbackHeight
}
