//go:build !darwin && !windows

package theme

func platformFontCandidates() []string {
	return []string{
		"/usr/share/fonts/redhat/RedHatDisplay-BlackItalic.otf",
		"/usr/share/fonts/inter/Inter-Bold.otf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	}
}
