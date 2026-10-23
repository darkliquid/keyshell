# Specification: Multi-Platform Surface & Portability (Option C)

**Status:** Draft / Specification  
**Date:** 2026-10-23  
**Authors:** Google DeepMind pair programming assistant & Darkliquid  

---

## 1. Problem Statement & Motivation

Keyshell was initially developed as a Wayland layer-shell launcher on Linux. Consequently, several platform assumptions are directly embedded into the application:
1. Hardcoded `zwlr_layer_shell_v1` surface setup and keyboard exclusivity calls in [`internal/cli/run.go`](file:///home/darkliquid/Projects/keyshell/internal/cli/run.go).
2. Display geometry queries tied to `niri msg -j outputs` and `swaymsg -t get_outputs` in [`internal/display/display.go`](file:///home/darkliquid/Projects/keyshell/internal/display/display.go).
3. Capability defaults tied to `wl-copy` and `xdg-open` in [`internal/capability/capability.go`](file:///home/darkliquid/Projects/keyshell/internal/capability/capability.go).
4. Hardcoded Red Hat font file path `/usr/share/fonts/redhat/...` in [`internal/theme/theme.go`](file:///home/darkliquid/Projects/keyshell/internal/theme/theme.go).

However, Keyshell's core engine (the Launcher state machine, KSL interpreter, and 2D canvas rasterizer) is completely OS-agnostic pure Go. By defining clean surface and platform abstraction seams, Keyshell can be compiled and run on macOS and Windows with native look, feel, and performance.

---

## 2. Design Goals

1. **Surface Seam**: Isolate the window surface and overlay presenter behind a `SurfaceHost` interface, removing direct Wayland layer-shell calls from `cli/run.go`.
2. **Font Fallback Cascade**: Remove the hardcoded `/usr/share/fonts/...` constant. Support embedded fallback fonts, user-specified font paths, and platform-native font discovery (SF Pro on macOS, Segoe UI on Windows, Red Hat / Inter / DejaVu on Linux).
3. **Display Geometry Provider**: Abstract primary display resolution detection across platforms via Go build tags (`display_linux.go`, `display_darwin.go`, `display_windows.go`).
4. **Platform Capability Drivers**: Provide native clipboard and URL open implementations without shelling out to command-line tools.

---

## 3. Detailed Specification

### 3.1. Surface Host Interface

In `internal/surface/surface.go`:
```go
type SurfaceHost interface {
    Init(cfg SurfaceConfig) error
    SetVisible(open bool) error
    SetKeyboardFocus(exclusive bool) error
    SetInputRegion(interactive bool) error
    RequestRedraw()
    Run() error
}
```

Implementations:
- `surface_linux.go`: Wraps `gogpu.App` with `WithLayerShell`.
- `surface_darwin.go`: Creates a floating borderless `NSPanel` using `Cocoa` / `Metal` or `gogpu` standard window without layer shell.
- `surface_windows.go`: Creates a borderless `WS_EX_TOPMOST | WS_EX_LAYERED` window.

### 3.2. Theme Font Cascade

In [`internal/theme/theme.go`](file:///home/darkliquid/Projects/keyshell/internal/theme/theme.go):
Replace the hardcoded constant:
```go
func ResolveFont(preferred string) (string, error) {
    if preferred != "" && fileExists(preferred) {
        return preferred, nil
    }
    candidates := platformFontCandidates()
    for _, path := range candidates {
        if fileExists(path) {
            return path, nil
        }
    }
    // Fallback: embedded TTF/OTF font bytes loaded with text.NewFontSource
    return loadEmbeddedFont()
}
```
Platform candidates:
- **Linux**: `/usr/share/fonts/redhat/RedHatDisplay-BlackItalic.otf`, `/usr/share/fonts/inter/Inter-Bold.otf`, `/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf`.
- **macOS**: `/System/Library/Fonts/SFProDisplay-Bold.otf`, `/System/Library/Fonts/Helvetica.ttc`.
- **Windows**: `C:\Windows\Fonts\segoeuib.ttf`, `C:\Windows\Fonts\arialbd.ttf`.

### 3.3. Capability Broker Platform Tagging

In [`internal/capability`](file:///home/darkliquid/Projects/keyshell/internal/capability):
- `capability_linux.go`: defaults to `wl-copy` and `xdg-open`.
- `capability_darwin.go`: defaults to `pbcopy` and `open`.
- `capability_windows.go`: defaults to Win32 clipboard API and `rundll32 url.dll,FileProtocolHandler`.

---

## 4. Verification & Testing

- Compile tests across targets (`GOOS=darwin go build ./...`, `GOOS=windows go build ./...`).
- Verify `theme.Load()` succeeds on systems without `/usr/share/fonts/redhat/...` by loading the embedded font fallback.
- Verify `display.Primary()` returns valid screen dimensions without invoking Linux compositor CLI tools.
