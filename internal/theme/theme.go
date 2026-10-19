// Package theme holds keyshell's visual identity: the palette and the one
// display face everything is drawn with. Code in internal/ui draws only with
// these values; raw hex never appears in drawing code.
package theme

import (
	"fmt"
	"sync"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
)

// DefaultFace is the display face the design is built on. It is the Black
// Italic instance of Red Hat Display, which is what the QML theme requests.
const DefaultFace = "/usr/share/fonts/redhat/RedHatDisplay-BlackItalic.otf"

// Theme is the loaded palette plus a shared font source.
type Theme struct {
	Slab       gg.RGBA
	SurfaceAlt gg.RGBA
	Ink        gg.RGBA
	Accent     gg.RGBA
	AccentAlt  gg.RGBA
	Muted      gg.RGBA

	source *text.FontSource
	mu     sync.Mutex
}

// Load builds the theme from the default face.
func Load() (*Theme, error) {
	source, err := text.NewFontSourceFromFile(DefaultFace)
	if err != nil {
		return nil, fmt.Errorf("load display face: %w", err)
	}
	return &Theme{
		Slab:       hex("#0B0B0F"),
		SurfaceAlt: hex("#121218"),
		Ink:        hex("#F2F2F7"),
		Accent:     hex("#FF2E63"),
		AccentAlt:  hex("#08F7FE"),
		Muted:      hex("#5A5A66"),
		source:     source,
	}, nil
}

// Face returns a Face at the given pixel size. FontSource is not safe to call
// Face on concurrently in all versions, so the lock keeps Face() concurrency
// safe even though callers draw on one goroutine.
func (t *Theme) Face(size float64) text.Face {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.source.Face(size)
}

// Close releases the font source.
func (t *Theme) Close() error {
	if t.source == nil {
		return nil
	}
	return t.source.Close()
}

// hex parses "#RRGGBB" into a gg colour.
func hex(s string) gg.RGBA {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return gg.RGBA{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}
}
