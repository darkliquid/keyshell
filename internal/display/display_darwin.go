//go:build darwin

package display

// platformPrimary returns (0, 0) on macOS so that display detection intentionally
// falls back to FallbackWidth and FallbackHeight. Native CoreGraphics or AppKit
// display dimension resolution is not implemented to avoid CGo dependencies.
func platformPrimary() (int, int) {
	return 0, 0
}
