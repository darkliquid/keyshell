package render

import (
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"sync"

	_ "golang.org/x/image/webp"
)

// ImageCache decodes images off the frame thread and caches them keyed
// by path plus target size. Gets before a decode finishes report
// not-ready, and the painter draws a placeholder.
type ImageCache struct {
	mu   sync.Mutex
	reqs map[string]*pendingImage
}

type pendingImage struct {
	mu    sync.Mutex
	ready bool
	img   image.Image
}

func NewImageCache() *ImageCache {
	return &ImageCache{reqs: map[string]*pendingImage{}}
}

// Get returns the decoded image for a path and target size, or not-ready
// when it is still decoding or failed.
func (c *ImageCache) Get(path string, w, h int) (image.Image, bool) {
	key := path + "@" + itoa(w) + "x" + itoa(h)
	c.mu.Lock()
	p, ok := c.reqs[key]
	if !ok {
		p = &pendingImage{}
		c.reqs[key] = p
		go decodeInto(p, path, w, h)
	}
	c.mu.Unlock()
	// The decode writes p under p.mu; read the result under the same lock.
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.img, p.ready
}

// Placeholder returns a muted 1x1 image painted where an image is not
// yet ready.
func (c *ImageCache) Placeholder() image.Image {
	return placeholderImage
}

var placeholderImage = func() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = 0x12
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xFF
	}
	return img
}()

func decodeInto(p *pendingImage, path string, w, h int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	img, err := decode(f)
	if err != nil {
		return
	}
	if w > 0 && h > 0 {
		img = scaleTo(img, w, h)
	}
	p.mu.Lock()
	p.img = img
	p.ready = true
	p.mu.Unlock()
}

func decode(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// scaleTo scales img to the target box, preserving aspect and centring
// (a cover fit).
func scaleTo(img image.Image, w, h int) image.Image {
	src := img.Bounds()
	srcW, srcH := src.Dx(), src.Dy()
	scale := float64(w) / float64(srcW)
	if float64(h)/float64(srcH) < scale {
		scale = float64(h) / float64(srcH)
	}
	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			sx := src.Min.X + int(float64(x)/scale)
			sy := src.Min.Y + int(float64(y)/scale)
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

// Register codecs so image.Decode knows them.
func init() {
	image.RegisterFormat("jpeg", "\xff\xd8\xff", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "\x89PNG\r\n\x1a\n", png.Decode, png.DecodeConfig)
}
