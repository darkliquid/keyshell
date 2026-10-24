# Multi-Platform Surface & Portability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple keyshell from Linux/Wayland-specific assumptions to achieve cross-platform portability across Linux, macOS, and Windows.

**Architecture:** Introduce `internal/surface` with a `SurfaceHost` interface isolating layer-shell vs desktop windowing; abstract font resolution with platform candidate lists and embedded `Go Bold` font fallback; split display resolution detection and capability broker actions into OS-tagged implementations.

**Tech Stack:** Go standard library (`syscall`, `os/exec`), `golang.org/x/image/font/gofont/gobold`, `github.com/gogpu/gogpu`, `github.com/gogpu/gg/text`.

---

### Task 1: Theme Font Fallback Cascade & Embedded Font

**Files:**
- Modify: `internal/theme/theme.go`
- Create: `internal/theme/font_linux.go`
- Create: `internal/theme/font_darwin.go`
- Create: `internal/theme/font_windows.go`
- Create: `internal/theme/theme_test.go`

- [x] **Step 1: Write failing tests for font resolution and embedded fallback**

Create `internal/theme/theme_test.go`:
```go
package theme

import (
	"testing"
)

func TestResolveFontSourceFallbackWhenNoSystemFonts(t *testing.T) {
	// Point to a non-existent path to force the candidate search and embedded fallback
	source, err := ResolveFontSource("/nonexistent/font/path.otf")
	if err != nil {
		t.Fatalf("ResolveFontSource failed: %v", err)
	}
	defer source.Close()

	face := source.Face(24)
	if face == nil {
		t.Fatal("expected non-nil Face from font source")
	}
}

func TestThemeLoadSucceeds(t *testing.T) {
	th, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	defer th.Close()

	if th.Face(16) == nil {
		t.Fatal("expected non-nil Face from Theme")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme -run TestResolveFontSourceFallbackWhenNoSystemFonts -v`
Expected: FAIL with `undefined: ResolveFontSource`

- [x] **Step 3: Implement platform font candidate lists and embedded fallback**

Create `internal/theme/font_linux.go`:
```go
//go:build !darwin && !windows

package theme

func platformFontCandidates() []string {
	return []string{
		"/usr/share/fonts/redhat/RedHatDisplay-BlackItalic.otf",
		"/usr/share/fonts/inter/Inter-Bold.otf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	}
}
```

Create `internal/theme/font_darwin.go`:
```go
//go:build darwin

package theme

func platformFontCandidates() []string {
	return []string{
		"/System/Library/Fonts/SFProDisplay-Bold.otf",
		"/System/Library/Fonts/Helvetica.ttc",
		"/Library/Fonts/Arial Bold.ttf",
	}
}
```

Create `internal/theme/font_windows.go`:
```go
//go:build windows

package theme

func platformFontCandidates() []string {
	return []string{
		"C:\\Windows\\Fonts\\segoeuib.ttf",
		"C:\\Windows\\Fonts\\arialbd.ttf",
	}
}
```

Modify `internal/theme/theme.go`:
```go
package theme

import (
	"fmt"
	"os"
	"sync"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
	"golang.org/x/image/font/gofont/gobold"
)

// DefaultFace is the preferred display face when available.
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

// ResolveFont returns the path of a font file on disk if found, or empty if fallback is used.
func ResolveFont(preferred string) (string, error) {
	if preferred != "" && fileExists(preferred) {
		return preferred, nil
	}
	for _, path := range platformFontCandidates() {
		if fileExists(path) {
			return path, nil
		}
	}
	return "", nil
}

// ResolveFontSource loads a font source from preferred path, platform candidates,
// or the embedded Go Bold font fallback.
func ResolveFontSource(preferred string) (*text.FontSource, error) {
	path, _ := ResolveFont(preferred)
	if path != "" {
		source, err := text.NewFontSourceFromFile(path)
		if err == nil {
			return source, nil
		}
	}
	return loadEmbeddedFont()
}

func loadEmbeddedFont() (*text.FontSource, error) {
	return text.NewFontSource(gobold.TTF)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// Load builds the theme from the resolved font cascade.
func Load() (*Theme, error) {
	return LoadWithFont("")
}

// LoadWithFont builds the theme using a preferred font path or fallback cascade.
func LoadWithFont(preferred string) (*Theme, error) {
	source, err := ResolveFontSource(preferred)
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

// Face returns a Face at the given pixel size.
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

func hex(s string) gg.RGBA {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return gg.RGBA{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}
}
```

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/theme -v`
Expected: PASS

- [x] **Step 5: Run tests across packages**

Run: `go test ./internal/render ./internal/card ./internal/theme -v`
Expected: PASS

---

### Task 2: Platform Capability Drivers (Clipboard & Open URL)

**Files:**
- Modify: `internal/capability/capability.go:30-45`
- Create: `internal/capability/capability_linux.go`
- Create: `internal/capability/capability_darwin.go`
- Create: `internal/capability/capability_windows.go`
- Modify: `internal/capability/capability_test.go`

- [x] **Step 1: Write failing tests for platform capability defaults**

In `internal/capability/capability_test.go`:
```go
func TestDefaultCommandsConfigured(t *testing.T) {
	cfg := config.Config{}
	b := New(cfg)
	if b.copyCmd == nil {
		t.Fatal("expected non-nil copyCmd")
	}
	if b.openURLCmd == nil {
		t.Fatal("expected non-nil openURLCmd")
	}
}
```

- [x] **Step 2: Create platform-tagged files in `internal/capability/`**

Create `internal/capability/capability_linux.go`:
```go
//go:build !darwin && !windows

package capability

import "os/exec"

func defaultCopy(s string) error {
	return exec.Command("wl-copy", s).Run()
}

func defaultOpenURL(url string) error {
	return runDetached("xdg-open", url)
}
```

Create `internal/capability/capability_darwin.go`:
```go
//go:build darwin

package capability

import (
	"os/exec"
	"strings"
)

func defaultCopy(s string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

func defaultOpenURL(url string) error {
	return runDetached("open", url)
}
```

Create `internal/capability/capability_windows.go`:
```go
//go:build windows

package capability

import "os/exec"

func defaultCopy(s string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard", "-Value", s)
	return cmd.Run()
}

func defaultOpenURL(url string) error {
	return runDetached("rundll32", "url.dll,FileProtocolHandler", url)
}
```

Modify `internal/capability/capability.go`:
Update `New(cfg config.Config) *Broker` to call `defaultCopy(s)` and `defaultOpenURL(url)` when `cfg.Commands.Copy` / `cfg.Commands.OpenURL` are empty:
```go
		copyCmd: func(s string) error {
			if argv := cfg.Commands.Copy; len(argv) > 0 {
				return runDetached(argv[0], append(argv[1:], s)...)
			}
			return defaultCopy(s)
		},
		openURLCmd: func(url string) error {
			if argv := cfg.Commands.OpenURL; len(argv) > 0 {
				return runDetached(argv[0], append(argv[1:], url)...)
			}
			return defaultOpenURL(url)
		},
```

- [x] **Step 3: Run capability tests**

Run: `go test ./internal/capability -v`
Expected: PASS

---

### Task 3: Display Geometry Platform Provider

**Files:**
- Modify: `internal/display/display.go`
- Create: `internal/display/display_linux.go`
- Create: `internal/display/display_darwin.go`
- Create: `internal/display/display_windows.go`
- Create: `internal/display/display_test.go`

- [x] **Step 1: Write failing test for `display.Primary`**

Create `internal/display/display_test.go`:
```go
package display

import "testing"

func TestPrimaryReturnsNonZeroDimensions(t *testing.T) {
	w, h := Primary()
	if w <= 0 || h <= 0 {
		t.Fatalf("Primary() = (%d, %d), expected positive dimensions", w, h)
	}
}
```

- [x] **Step 2: Refactor `internal/display/display.go` with OS build tags**

In `internal/display/display.go`:
```go
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
```

Create `internal/display/display_linux.go`:
```go
//go:build !darwin && !windows

package display

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func platformPrimary() (int, int) {
	if os.Getenv("NIRI_SOCKET") != "" {
		if w, h, ok := niriSize(); ok {
			return w, h
		}
	}
	if os.Getenv("SWAYSOCK") != "" {
		if w, h, ok := swaySize(); ok {
			return w, h
		}
	}
	if w, h, ok := wlOutputsSize(); ok {
		return w, h
	}
	return 0, 0
}

func niriSize() (int, int, bool) {
	out, err := exec.Command("niri", "msg", "-j", "outputs").Output()
	if err != nil {
		return 0, 0, false
	}
	var outputs map[string]struct {
		Logical *struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"logical"`
		CurrentMode *int `json:"current_mode"`
		Modes       []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"modes"`
	}
	if json.Unmarshal(out, &outputs) != nil {
		return 0, 0, false
	}
	for _, o := range outputs {
		if o.Logical != nil && o.Logical.Width > 0 {
			return o.Logical.Width, o.Logical.Height, true
		}
		if o.CurrentMode != nil && *o.CurrentMode < len(o.Modes) {
			m := o.Modes[*o.CurrentMode]
			if m.Width > 0 {
				return m.Width, m.Height, true
			}
		}
	}
	return 0, 0, false
}

func swaySize() (int, int, bool) {
	out, err := exec.Command("swaymsg", "-j", "-t", "get_outputs").Output()
	if err != nil {
		return 0, 0, false
	}
	var outputs []struct {
		Active  bool                        `json:"active"`
		Current struct{ Width, Height int } `json:"current_mode"`
		Rect    struct{ Width, Height int }
	}
	if json.Unmarshal(out, &outputs) != nil {
		return 0, 0, false
	}
	for _, o := range outputs {
		if !o.Active {
			continue
		}
		if o.Current.Width > 0 {
			return o.Current.Width, o.Current.Height, true
		}
		if o.Rect.Width > 0 {
			return o.Rect.Width, o.Rect.Height, true
		}
	}
	return 0, 0, false
}

func wlOutputsSize() (int, int, bool) {
	for _, cmd := range [][]string{
		{"wlr-randr"},
		{"kscreen-doctor", "-o"},
	} {
		if _, err := exec.LookPath(cmd[0]); err != nil {
			continue
		}
		out, err := exec.Command(cmd[0], cmd[1:]...).Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			for _, word := range strings.Fields(line) {
				if w, h, ok := parseResolution(word); ok {
					return w, h, true
				}
			}
		}
	}
	return 0, 0, false
}

func parseResolution(s string) (int, int, bool) {
	p := strings.Index(s, "x")
	if p <= 0 || p >= len(s)-1 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(s[:p])
	hStr := s[p+1:]
	if at := strings.Index(hStr, "@"); at > 0 {
		hStr = hStr[:at]
	}
	h, err2 := strconv.Atoi(hStr)
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}
```

Create `internal/display/display_darwin.go`:
```go
//go:build darwin

package display

func platformPrimary() (int, int) {
	// Fall back cleanly to DefaultWidth, DefaultHeight on macOS without requiring external CLI tools
	return 0, 0
}
```

Create `internal/display/display_windows.go`:
```go
//go:build windows

package display

import "syscall"

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	smCxScreen = 0
	smCyScreen = 1
)

func platformPrimary() (int, int) {
	w, _, _ := getSystemMetrics.Call(uintptr(smCxScreen))
	h, _, _ := getSystemMetrics.Call(uintptr(smCyScreen))
	return int(w), int(h)
}
```

- [x] **Step 3: Run display tests**

Run: `go test ./internal/display -v`
Expected: PASS

---

### Task 4: SurfaceHost Abstraction & CLI Run Decoupling

**Files:**
- Create: `internal/surface/surface.go`
- Create: `internal/surface/surface_linux.go`
- Create: `internal/surface/surface_darwin.go`
- Create: `internal/surface/surface_windows.go`
- Create: `internal/surface/surface_test.go`
- Modify: `internal/cli/run.go`

- [x] **Step 1: Define `SurfaceHost` interface and write tests**

Create `internal/surface/surface.go`:
```go
package surface

import (
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// SurfaceConfig describes the window or layer surface to initialize.
type SurfaceConfig struct {
	Width       int
	Height      int
	Title       string
	Namespace   string
	Transparent bool
}

// SurfaceHost abstracts the presentation surface across operating systems.
type SurfaceHost interface {
	Init(cfg SurfaceConfig) error
	SetVisible(open bool) error
	SetKeyboardFocus(exclusive bool) error
	SetInputRegion(interactive bool) error
	RequestRedraw()
	Run() error
	Quit()
	App() *gogpu.App
	EventSource() gpucontext.EventSource
}
```

Create `internal/surface/surface_test.go`:
```go
package surface

import "testing"

func TestNewHost(t *testing.T) {
	cfg := SurfaceConfig{
		Width:       800,
		Height:      600,
		Title:       "test-keyshell",
		Namespace:   "test",
		Transparent: true,
	}
	host := NewHost()
	if err := host.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if host.App() == nil {
		t.Fatal("expected non-nil App")
	}
	if host.EventSource() == nil {
		t.Fatal("expected non-nil EventSource")
	}
}
```

- [x] **Step 2: Implement platform-specific `SurfaceHost`**

Create `internal/surface/surface_linux.go`:
```go
//go:build !darwin && !windows

package surface

import (
	"os"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

type linuxSurfaceHost struct {
	app *gogpu.App
	cfg SurfaceConfig
}

// NewHost returns the platform-appropriate SurfaceHost.
func NewHost() SurfaceHost {
	return &linuxSurfaceHost{}
}

func (h *linuxSurfaceHost) Init(cfg SurfaceConfig) error {
	h.cfg = cfg
	if os.Getenv("GOGPU_WAYLAND_FRAME_CALLBACK") == "" {
		_ = os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
	}

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(cfg.Title).
		WithSize(cfg.Width, cfg.Height).
		WithTransparent(cfg.Transparent).
		WithContinuousRender(true).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     cfg.Namespace,
			Layer:         gogpu.LayerShellOverlay,
			Anchor:        gogpu.LayerShellAnchorTop | gogpu.LayerShellAnchorBottom | gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorRight,
			Width:         int32(cfg.Width),
			Height:        int32(cfg.Height),
			Keyboard:      gogpu.LayerShellKeyboardOnDemand,
			ExclusiveZone: -1,
		}))

	h.app = app
	return nil
}

func (h *linuxSurfaceHost) SetVisible(open bool) error {
	return nil
}

func (h *linuxSurfaceHost) SetKeyboardFocus(exclusive bool) error {
	if h.app == nil {
		return nil
	}
	mode := gogpu.LayerShellKeyboardNone
	if exclusive {
		mode = gogpu.LayerShellKeyboardExclusive
	}
	return h.app.SetLayerKeyboardInteractivity(mode)
}

func (h *linuxSurfaceHost) SetInputRegion(interactive bool) error {
	if h.app == nil {
		return nil
	}
	return h.app.SetInputRegion(interactive)
}

func (h *linuxSurfaceHost) RequestRedraw() {
	if h.app != nil {
		h.app.RequestRedraw()
	}
}

func (h *linuxSurfaceHost) Run() error {
	if h.app != nil {
		return h.app.Run()
	}
	return nil
}

func (h *linuxSurfaceHost) Quit() {
	if h.app != nil {
		h.app.Quit()
	}
}

func (h *linuxSurfaceHost) App() *gogpu.App {
	return h.app
}

func (h *linuxSurfaceHost) EventSource() gpucontext.EventSource {
	if h.app != nil {
		return h.app.EventSource()
	}
	return nil
}
```

Create `internal/surface/surface_darwin.go`:
```go
//go:build darwin

package surface

import (
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

type darwinSurfaceHost struct {
	app *gogpu.App
	cfg SurfaceConfig
}

func NewHost() SurfaceHost {
	return &darwinSurfaceHost{}
}

func (h *darwinSurfaceHost) Init(cfg SurfaceConfig) error {
	h.cfg = cfg
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(cfg.Title).
		WithSize(cfg.Width, cfg.Height).
		WithTransparent(cfg.Transparent).
		WithFrameless(true).
		WithContinuousRender(true))

	h.app = app
	return nil
}

func (h *darwinSurfaceHost) SetVisible(open bool) error {
	return nil
}

func (h *darwinSurfaceHost) SetKeyboardFocus(exclusive bool) error {
	return nil
}

func (h *darwinSurfaceHost) SetInputRegion(interactive bool) error {
	return nil
}

func (h *darwinSurfaceHost) RequestRedraw() {
	if h.app != nil {
		h.app.RequestRedraw()
	}
}

func (h *darwinSurfaceHost) Run() error {
	if h.app != nil {
		return h.app.Run()
	}
	return nil
}

func (h *darwinSurfaceHost) Quit() {
	if h.app != nil {
		h.app.Quit()
	}
}

func (h *darwinSurfaceHost) App() *gogpu.App {
	return h.app
}

func (h *darwinSurfaceHost) EventSource() gpucontext.EventSource {
	if h.app != nil {
		return h.app.EventSource()
	}
	return nil
}
```

Create `internal/surface/surface_windows.go`:
```go
//go:build windows

package surface

import (
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

type windowsSurfaceHost struct {
	app *gogpu.App
	cfg SurfaceConfig
}

func NewHost() SurfaceHost {
	return &windowsSurfaceHost{}
}

func (h *windowsSurfaceHost) Init(cfg SurfaceConfig) error {
	h.cfg = cfg
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(cfg.Title).
		WithSize(cfg.Width, cfg.Height).
		WithTransparent(cfg.Transparent).
		WithFrameless(true).
		WithContinuousRender(true))

	h.app = app
	return nil
}

func (h *windowsSurfaceHost) SetVisible(open bool) error {
	return nil
}

func (h *windowsSurfaceHost) SetKeyboardFocus(exclusive bool) error {
	return nil
}

func (h *windowsSurfaceHost) SetInputRegion(interactive bool) error {
	return nil
}

func (h *windowsSurfaceHost) RequestRedraw() {
	if h.app != nil {
		h.app.RequestRedraw()
	}
}

func (h *windowsSurfaceHost) Run() error {
	if h.app != nil {
		return h.app.Run()
	}
	return nil
}

func (h *windowsSurfaceHost) Quit() {
	if h.app != nil {
		h.app.Quit()
	}
}

func (h *windowsSurfaceHost) App() *gogpu.App {
	return h.app
}

func (h *windowsSurfaceHost) EventSource() gpucontext.EventSource {
	if h.app != nil {
		return h.app.EventSource()
	}
	return nil
}
```

- [x] **Step 3: Refactor `internal/cli/run.go` to use `SurfaceHost`**

In `internal/cli/run.go`:
Replace direct `gogpu.NewApp` with `surface.NewHost()`:
```go
	screenW, screenH := display.Primary()
	surfHost := surface.NewHost()
	if err := surfHost.Init(surface.SurfaceConfig{
		Width:       screenW,
		Height:      screenH,
		Title:       "keyshell",
		Namespace:   "keyshell",
		Transparent: true,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "surface init:", err)
		return 1
	}

	app := surfHost.App()
	rdr := render.NewConfigured(cfg)
	watchScene(ctx, hf, cfg, rdr, app)

	// Wire input.
	in := input.New(l, surfHost.EventSource())
```
And replace layer interactivity calls with `surfHost`:
```go
	var keyboardActive atomic.Bool
	l.OnChange(func() {
		open := l.IsOpen()
		if keyboardActive.Load() == open {
			return
		}
		keyboardActive.Store(open)
		_ = surfHost.SetKeyboardFocus(open)
		_ = surfHost.SetInputRegion(open)
	})

	in.OnPress(func(x, y float64) {
		if !l.IsOpen() {
			return
		}
		if !rdr.BandContains(x, y) {
			l.Close()
			return
		}
		_ = surfHost.SetKeyboardFocus(false)
		_ = surfHost.SetKeyboardFocus(true)
	})
```
And replace `app.Quit()` on context cancellation:
```go
	go func() {
		<-ctx.Done()
		surfHost.Quit()
	}()
```

- [x] **Step 4: Run tests**

Run: `go test ./internal/surface ./internal/cli -v`
Expected: PASS

---

### Task 5: Multi-Platform Cross-Compilation & Full Verification

**Files:**
- Test: All packages

- [x] **Step 1: Test Linux build**

Run: `GOOS=linux go build ./...`
Expected: exit code 0

- [x] **Step 2: Test macOS build**

Run: `GOOS=darwin go build ./...`
Expected: exit code 0

- [x] **Step 3: Test Windows build**

Run: `GOOS=windows go build ./...`
Expected: exit code 0

- [x] **Step 4: Run full test suite with race detector**

Run: `go test -count=1 ./...`
Expected: PASS across all packages
