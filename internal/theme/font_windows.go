//go:build windows

package theme

import (
	"os"
	"path/filepath"
)

func platformFontCandidates() []string {
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = os.Getenv("SystemRoot")
	}
	if windir == "" {
		windir = `C:\Windows`
	}
	fonts := filepath.Join(windir, "Fonts")
	return []string{
		filepath.Join(fonts, "segoeuib.ttf"),
		filepath.Join(fonts, "arialbd.ttf"),
	}
}
