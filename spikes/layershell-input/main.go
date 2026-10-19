//go:build linux

// SPIKE S2 (throwaway): does a wlr-layer-shell surface receive input?
//
// The question this answers is not "can gogpu make a layer surface" - the
// layer-shell branch already proved that - but whether a launcher overlay can
// rely on keyboard and pointer input once it has one:
//
//   - does text input arrive on a layer surface at all (exclusive focus)?
//   - with on-demand focus, is focus granted by clicking the surface and
//     released by clicking away, the way a launcher overlay needs?
//   - do pointer coordinates arrive in logical pixels?
//   - do the keys a launcher binds (Escape, Enter, Backspace) reach the app?
//   - does the surface survive losing focus, or does it disappear?
//
// Everything the spike observes is printed to stdout with a timestamp and
// tallied at exit, so a test run can be asserted from the log. The surface
// itself paints cyan while focused and dark while not, so a screenshot shows
// the focus state too.
//
// Run:
//
//	mise run spike:s2                       # on-demand keyboard (the launcher case)
//	go run ./spikes/layershell-input -keyboard exclusive   # focus without clicking
//	go run ./spikes/layershell-input -duration=20s
//
// Drive it without a human at the keyboard (see spikes/tools/inject):
//
//	go run ./spikes/tools/inject click 100 50
//	go run ./spikes/tools/inject type "hello"
//	go run ./spikes/tools/inject key esc
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gpucontext"
)

const (
	surfaceWidth  = 480
	surfaceHeight = 160
)

func main() {
	keyboard := flag.String("keyboard", "on_demand", "layer surface keyboard interactivity: on_demand|exclusive|none")
	duration := flag.Duration("duration", 0, "quit after this long (0 = until closed)")
	namespace := flag.String("namespace", "keyshell-spike-s2", "layer surface namespace")
	flag.Parse()

	keyboardMode, err := parseKeyboard(*keyboard)
	if err != nil {
		log.Fatal(err)
	}

	s := newSpike()

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("keyshell S2 spike").
		WithSize(surfaceWidth, surfaceHeight).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     *namespace,
			Layer:         gogpu.LayerShellOverlay,
			Anchor:        gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorTop,
			Width:         surfaceWidth,
			Height:        surfaceHeight,
			Keyboard:      keyboardMode,
			ExclusiveZone: -1,
		}))

	// gogpu documents that callbacks may be registered before Run.
	events := app.EventSource()
	events.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		s.logf("key press    %s mods=%s", keyName(key), modName(mods))
		s.mu.Lock()
		s.keyPresses++
		s.mu.Unlock()
		s.handleKey(key)
	})
	events.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		s.mu.Lock()
		s.keyReleases++
		s.mu.Unlock()
	})
	events.OnTextInput(func(text string) {
		s.logf("text input   %q", text)
		s.mu.Lock()
		s.textEvents++
		s.buffer += text
		s.mu.Unlock()
	})
	events.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		s.logf("mouse press  button=%d at (%.0f, %.0f) logical", button, x, y)
		s.mu.Lock()
		s.mousePresses++
		s.lastPointer = fmt.Sprintf("(%.0f, %.0f)", x, y)
		s.mu.Unlock()
	})
	events.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		s.mu.Lock()
		s.mouseReleases++
		s.mu.Unlock()
	})
	events.OnMouseMove(func(x, y float64) {
		s.mu.Lock()
		s.mouseMoves++
		// Pointer motion is continuous; log only the first few and every 50th.
		shouldLog := s.mouseMoves <= 3 || s.mouseMoves%50 == 0
		s.mu.Unlock()
		if shouldLog {
			s.logf("mouse move   (%d total) at (%.0f, %.0f)", s.mouseMoves, x, y)
		}
	})
	events.OnScroll(func(dx, dy float64) { s.logf("scroll       dx=%.1f dy=%.1f", dx, dy) })
	events.OnFocus(func(focused bool) {
		s.logf("FOCUS        focused=%v", focused)
		s.mu.Lock()
		s.focused = focused
		if focused {
			s.focusGained++
		} else {
			s.focusLost++
		}
		s.mu.Unlock()
	})
	events.OnIMECompositionStart(func() { s.logf("ime          composition start") })
	events.OnIMECompositionUpdate(func(state gpucontext.IMEState) {
		s.logf("ime          composition update %q", state.CompositionText)
	})
	events.OnIMECompositionEnd(func(committed string) { s.logf("ime          composition end %q", committed) })

	app.OnDraw(func(dc *gogpu.Context) {
		// Colour encodes focus: cyan when focused, dark ink when not. Plus a
		// white strip whose width tracks typed characters, so a screenshot
		// shows input arrived.
		if s.isFocused() {
			dc.ClearColor(gmath.Cyan)
		} else {
			dc.ClearColor(gmath.Color{R: 0.04, G: 0.04, B: 0.06, A: 1})
		}
	})

	if *duration > 0 {
		time.AfterFunc(*duration, app.Quit)
	}

	// Ctrl+C should print the summary too, not just die.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		s.logf("signal received, quitting")
		app.Quit()
	}()

	s.logf("S2 spike up: %dx%d namespace=%q keyboard=%s", surfaceWidth, surfaceHeight, *namespace, *keyboard)
	s.logf("verify surface: niri msg layers")

	if err := app.Run(); err != nil {
		s.logf("FAILED: %v", err)
		s.summary()
		fmt.Fprintln(os.Stderr, "this spike needs a Wayland compositor with zwlr_layer_shell_v1")
		os.Exit(1)
	}
	s.summary()
}

type spike struct {
	mu sync.Mutex

	start time.Time
	once  sync.Once
	done  chan struct{}

	focused bool
	buffer  string

	keyPresses, keyReleases, textEvents     int
	mousePresses, mouseReleases, mouseMoves int
	focusGained, focusLost                  int
	escapes, enters, backspaces             int
	lastPointer                             string
}

func newSpike() *spike {
	return &spike{start: time.Now(), done: make(chan struct{})}
}

func (s *spike) logf(format string, args ...any) {
	elapsed := time.Since(s.start).Seconds()
	fmt.Printf("[%7.3fs] %s\n", elapsed, fmt.Sprintf(format, args...))
}

func (s *spike) isFocused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.focused
}

// handleKey implements the parts of a launcher's key handling this spike needs
// to assert: dismissal, commit, and edit.
func (s *spike) handleKey(key gpucontext.Key) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch key {
	case gpucontext.KeyEscape:
		s.escapes++
		s.buffer = ""
		s.logf("   -> Escape: buffer cleared (a launcher would dismiss here)")
	case gpucontext.KeyEnter:
		s.enters++
		s.logf("   -> Enter: committing %q (a launcher would execute here)", s.buffer)
		s.buffer = ""
	case gpucontext.KeyBackspace:
		s.backspaces++
		if n := len(s.buffer); n > 0 {
			s.buffer = s.buffer[:n-1]
		}
		s.logf("   -> Backspace: buffer is now %q", s.buffer)
	}
}

func (s *spike) summary() {
	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Println()
	fmt.Println("=== S2 summary ===")
	fmt.Printf("duration        %s\n", time.Since(s.start).Round(time.Millisecond))
	fmt.Printf("focus           gained %d, lost %d (final focused=%v)\n", s.focusGained, s.focusLost, s.focused)
	fmt.Printf("keyboard        %d key presses, %d releases, %d text input events\n", s.keyPresses, s.keyReleases, s.textEvents)
	fmt.Printf("pointer         %d presses, %d releases, %d moves (last press at %s)\n",
		s.mousePresses, s.mouseReleases, s.mouseMoves, s.lastPointer)
	fmt.Printf("launcher keys   escapes %d, enters %d, backspaces %d\n", s.escapes, s.enters, s.backspaces)
	fmt.Printf("text buffer     %q\n", s.buffer)
	fmt.Println("=== end summary ===")
}

func parseKeyboard(mode string) (gogpu.LayerShellKeyboard, error) {
	switch strings.ToLower(mode) {
	case "on_demand", "on-demand":
		return gogpu.LayerShellKeyboardOnDemand, nil
	case "exclusive":
		return gogpu.LayerShellKeyboardExclusive, nil
	case "none":
		return gogpu.LayerShellKeyboardNone, nil
	default:
		return 0, fmt.Errorf("unknown -keyboard %q (want on_demand, exclusive or none)", mode)
	}
}

// keyName renders the gpucontext key the way a user would describe it.
// gpucontext numbers keys by group: letters are 1..26 (KeyA..KeyZ), not ASCII.
func keyName(key gpucontext.Key) string {
	switch key {
	case gpucontext.KeyEscape:
		return "Escape"
	case gpucontext.KeyEnter:
		return "Enter"
	case gpucontext.KeyBackspace:
		return "Backspace"
	case gpucontext.KeyTab:
		return "Tab"
	case gpucontext.KeySpace:
		return "Space"
	}
	if key >= gpucontext.KeyA && key <= gpucontext.KeyZ {
		return string(rune('A' + (key - gpucontext.KeyA)))
	}
	if key >= gpucontext.Key1 && key <= gpucontext.Key0 {
		digit := rune('1' + (key - gpucontext.Key1))
		if key == gpucontext.Key0 {
			digit = '0'
		}
		return string(digit)
	}
	return fmt.Sprintf("key(%d)", key)
}

// modName decodes the modifier bitmask, so a plain keypress printing "none"
// is visibly different from one reporting NumLock.
func modName(mods gpucontext.Modifiers) string {
	if mods == 0 {
		return "none"
	}
	names := make([]string, 0, 4)
	for _, m := range []struct {
		flag gpucontext.Modifiers
		name string
	}{
		{gpucontext.ModShift, "Shift"},
		{gpucontext.ModControl, "Ctrl"},
		{gpucontext.ModAlt, "Alt"},
		{gpucontext.ModSuper, "Super"},
		{gpucontext.ModCapsLock, "CapsLock"},
		{gpucontext.ModNumLock, "NumLock"},
	} {
		if mods&m.flag != 0 {
			names = append(names, m.name)
		}
	}
	if len(names) == 0 {
		return fmt.Sprintf("mods(%d)", mods)
	}
	return strings.Join(names, "+")
}
