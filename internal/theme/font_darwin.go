//go:build darwin

package theme

func platformFontCandidates() []string {
	return []string{
		"/System/Library/Fonts/SFProDisplay-Bold.otf",
		"/System/Library/Fonts/Helvetica.ttc",
		"/Library/Fonts/Arial Bold.ttf",
	}
}
