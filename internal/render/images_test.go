package render

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTinyPNG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{0xFF, 0, 0, 0xFF})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImageCacheLoadsAndCaches(t *testing.T) {
	dir := t.TempDir()
	pngPath := writeTinyPNG(t, dir, "a.png")
	c := NewImageCache()
	if _, ok := c.Get(pngPath, 4, 4); ok {
		t.Fatal("first Get must report not-ready")
	}
	// synchronously drain the decode
	if err := c.wait(pngPath, 4, 4); err != nil {
		t.Fatalf("wait: %v", err)
	}
	img, ok := c.Get(pngPath, 4, 4)
	if !ok || img == nil {
		t.Fatal("second Get must hit the cache")
	}
	img2, ok := c.Get(pngPath, 4, 4)
	if !ok || img2 != img {
		t.Errorf("cache returned a different image")
	}
}

func TestImageCachePlaceholder(t *testing.T) {
	c := NewImageCache()
	ph := c.Placeholder()
	if ph == nil || ph.Bounds().Dx() == 0 {
		t.Errorf("no placeholder")
	}
}

func TestImageCacheMissingFile(t *testing.T) {
	c := NewImageCache()
	if _, ok := c.Get("/no/such.png", 4, 4); ok {
		t.Errorf("missing file must not be ready")
	}
	// wait drains but the file never decodes; still not ready
	_ = c.wait("/no/such.png", 4, 4)
	if _, ok := c.Get("/no/such.png", 4, 4); ok {
		t.Errorf("missing file must stay not-ready")
	}
}

// wait blocks until the pending decode for a key finishes or fails.
func (c *ImageCache) wait(path string, w, h int) error {
	key := path + "@" + itoa(w) + "x" + itoa(h)
	for i := 0; i < 1000; i++ {
		c.mu.Lock()
		p, ok := c.reqs[key]
		c.mu.Unlock()
		if ok {
			p.mu.Lock()
			ready := p.ready
			p.mu.Unlock()
			if ready {
				return nil
			}
		}
		time.Sleep(time.Millisecond)
	}
	return errTimeout
}

var errTimeout = errors.New("image decode timeout")
