//go:build linux

// Test aid (throwaway): synthesise real keystrokes and pointer clicks through
// /dev/uinput so the layer-shell spike can be driven without a human at the
// keyboard. Nothing else in this repository talks to uinput.
//
// A virtual keyboard is straightforward. The pointer needs absolute
// positioning (a click has to land on a known pixel), so the device is created
// as an absolute pointer - ABS_X/ABS_Y with an absinfo range, plus BTN_LEFT -
// which is the same shape a VM's "absolute mouse" uses.
//
// Usage:
//
//	inject type "hello world"     type text (US layout)
//	inject key esc|enter|tab|backspace|space
//	inject click X Y              click at logical output coordinates
//	inject move X Y               move the pointer without clicking
//
// Requires write access to /dev/uinput (membership of the input group).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	uinputPath = "/dev/uinput"
	iocWrite   = 1
)

// ioctl request numbers, from linux/uinput.h. 'U' is the uinput type byte.
const (
	// UI_DEV_CREATE and UI_DEV_DESTROY are _IO(), i.e. direction 0, not
	// _IOC_READ. Wrong direction bits make the kernel answer EINVAL as an
	// unknown ioctl.
	uiDevCreate  = 'U'<<8 | 1
	uiDevDestroy = 'U'<<8 | 2

	uiDevSetup = uintptr(iocWrite)<<30 | uintptr(unsafe.Sizeof(uinputSetup{}))<<16 | 'U'<<8 | 3
	uiAbsSetup = uintptr(iocWrite)<<30 | uintptr(unsafe.Sizeof(uinputAbsSetup{}))<<16 | 'U'<<8 | 4

	uiSetEvBit  = uintptr(iocWrite)<<30 | 4<<16 | 'U'<<8 | 100
	uiSetKeyBit = uintptr(iocWrite)<<30 | 4<<16 | 'U'<<8 | 101
	uiSetAbsBit = uintptr(iocWrite)<<30 | 4<<16 | 'U'<<8 | 103
)

// Event types and codes, from linux/input-event-codes.h.
const (
	evSyn = 0
	evKey = 1
	evAbs = 3

	synReport = 0

	keyEsc       = 1
	key1         = 2
	key0         = 11
	keyBackspace = 14
	keyTab       = 15
	keyQ         = 16
	keyA         = 30
	keyLeftShift = 42
	keyZ         = 44
	keySpace     = 57
	keyEnter     = 28

	btnLeft = 0x110

	absX = 0
	absY = 1
)

// Output geometry, in logical pixels. The absolute pointer's 0..65535 range is
// mapped onto this so callers can click on coordinates they can see.
const (
	outputWidth  = 3840
	outputHeight = 1080
	absMaximum   = 65535
)

type inputID struct {
	busType, vendor, product, version uint16
}

type uinputSetup struct {
	id           inputID
	name         [80]byte
	ffEffectsMax uint32
}

type inputAbsInfo struct {
	value, minimum, maximum, fuzz, flat, resolution int32
}

type uinputAbsSetup struct {
	code    uint16
	_       uint16 // padding to align absinfo
	absInfo inputAbsInfo
}

type inputEvent struct {
	sec, usec int64
	typ       uint16
	code      uint16
	value     int32
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "type":
		if len(os.Args) != 3 {
			usage()
		}
		err = withKeyboard(func(fd int) error { return typeText(fd, os.Args[2]) })
	case "key":
		if len(os.Args) != 3 {
			usage()
		}
		err = withKeyboard(func(fd int) error { return pressNamedKey(fd, os.Args[2]) })
	case "click":
		x, y, parseErr := parsePoint(os.Args)
		if parseErr != nil {
			usage()
		}
		err = withPointer(func(fd int) error {
			if moveErr := moveTo(fd, x, y); moveErr != nil {
				return moveErr
			}
			if clickErr := click(fd); clickErr != nil {
				return clickErr
			}
			fmt.Printf("clicked at %d,%d\n", x, y)
			return nil
		})
	case "move":
		x, y, parseErr := parsePoint(os.Args)
		if parseErr != nil {
			usage()
		}
		err = withPointer(func(fd int) error {
			if moveErr := moveTo(fd, x, y); moveErr != nil {
				return moveErr
			}
			fmt.Printf("moved to %d,%d\n", x, y)
			return nil
		})
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "inject:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  inject type "text"
  inject key esc|enter|tab|backspace|space
  inject click X Y
  inject move X Y`)
	os.Exit(2)
}

func parsePoint(args []string) (int, int, error) {
	if len(args) != 4 {
		return 0, 0, fmt.Errorf("expected X and Y")
	}
	x, err := strconv.Atoi(args[2])
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(args[3])
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

// withKeyboard creates a virtual keyboard, runs fn, and destroys the device.
// The device is kept alive for a moment after the last event so the compositor
// has time to deliver everything before the node disappears.
func withKeyboard(fn func(fd int) error) error {
	fd, err := openUinput()
	if err != nil {
		return err
	}
	defer closeDevice(fd)

	if err := ioctlInt(fd, uiSetEvBit, uintptr(evKey)); err != nil {
		return fmt.Errorf("enable EV_KEY: %w", err)
	}
	for _, code := range typedKeys() {
		if err := ioctlInt(fd, uiSetKeyBit, uintptr(code)); err != nil {
			return fmt.Errorf("enable key %d: %w", code, err)
		}
	}
	for _, code := range []int{keyEsc, keyEnter, keyTab, keyBackspace, keySpace, keyLeftShift} {
		if err := ioctlInt(fd, uiSetKeyBit, uintptr(code)); err != nil {
			return fmt.Errorf("enable key %d: %w", code, err)
		}
	}
	if err := setupDevice(fd, "keyshell-spike-keyboard"); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond) // let the compositor pick the device up
	return fn(fd)
}

// withPointer creates a virtual absolute pointer, runs fn, and destroys it.
func withPointer(fn func(fd int) error) error {
	fd, err := openUinput()
	if err != nil {
		return err
	}
	defer closeDevice(fd)

	if err := ioctlInt(fd, uiSetEvBit, uintptr(evKey)); err != nil {
		return fmt.Errorf("enable EV_KEY: %w", err)
	}
	if err := ioctlInt(fd, uiSetKeyBit, uintptr(btnLeft)); err != nil {
		return fmt.Errorf("enable BTN_LEFT: %w", err)
	}
	if err := ioctlInt(fd, uiSetEvBit, uintptr(evAbs)); err != nil {
		return fmt.Errorf("enable EV_ABS: %w", err)
	}
	for _, axis := range []uint16{absX, absY} {
		if err := ioctlInt(fd, uiSetAbsBit, uintptr(axis)); err != nil {
			return fmt.Errorf("enable ABS_%d: %w", axis, err)
		}
		if err := ioctlAbsInfo(fd, axis); err != nil {
			return fmt.Errorf("absinfo for ABS_%d: %w", axis, err)
		}
	}
	if err := setupDevice(fd, "keyshell-spike-pointer"); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	return fn(fd)
}

func openUinput() (int, error) {
	fd, err := unix.Open(uinputPath, unix.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w (needs input group membership)", uinputPath, err)
	}
	return fd, nil
}

func closeDevice(fd int) {
	if err := ioctlRaw(fd, uiDevDestroy, 0); err != nil {
		fmt.Fprintln(os.Stderr, "inject: destroy device:", err)
	}
	time.Sleep(50 * time.Millisecond)
	_ = unix.Close(fd)
}

func setupDevice(fd int, name string) error {
	setup := uinputSetup{id: inputID{busType: 0x03 /* BUS_USB */, vendor: 0x1, product: 0x1, version: 1}}
	copy(setup.name[:], name)
	if err := ioctlRaw(fd, uiDevSetup, uintptr(unsafe.Pointer(&setup))); err != nil {
		return fmt.Errorf("UI_DEV_SETUP: %w", err)
	}
	if err := ioctlRaw(fd, uiDevCreate, 0); err != nil {
		return fmt.Errorf("UI_DEV_CREATE: %w", err)
	}
	return nil
}

func ioctlInt(fd int, request, value uintptr) error {
	return ioctlRaw(fd, request, value)
}

func ioctlAbsInfo(fd int, axis uint16) error {
	info := uinputAbsSetup{
		code: axis,
		absInfo: inputAbsInfo{
			minimum:    0,
			maximum:    absMaximum,
			resolution: 1,
		},
	}
	if err := ioctlRaw(fd, uiAbsSetup, uintptr(unsafe.Pointer(&info))); err != nil {
		return err
	}
	return nil
}

func ioctlRaw(fd int, request, arg uintptr) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), request, arg); errno != 0 {
		return errno
	}
	return nil
}

func moveTo(fd, x, y int) error {
	ax := int32(x) * absMaximum / outputWidth
	ay := int32(y) * absMaximum / outputHeight
	return writeEvents(fd,
		inputEvent{typ: evAbs, code: absX, value: ax},
		inputEvent{typ: evAbs, code: absY, value: ay},
		inputEvent{typ: evSyn, code: synReport},
	)
}

func click(fd int) error {
	if err := writeEvents(fd,
		inputEvent{typ: evKey, code: btnLeft, value: 1},
		inputEvent{typ: evSyn, code: synReport},
	); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return writeEvents(fd,
		inputEvent{typ: evKey, code: btnLeft, value: 0},
		inputEvent{typ: evSyn, code: synReport},
	)
}

func pressNamedKey(fd int, name string) error {
	codes := map[string]int{
		"esc":       keyEsc,
		"enter":     keyEnter,
		"tab":       keyTab,
		"backspace": keyBackspace,
		"space":     keySpace,
	}
	code, ok := codes[strings.ToLower(name)]
	if !ok {
		return fmt.Errorf("unknown key %q", name)
	}
	if err := writeEvents(fd,
		inputEvent{typ: evKey, code: uint16(code), value: 1},
		inputEvent{typ: evSyn, code: synReport},
	); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := writeEvents(fd,
		inputEvent{typ: evKey, code: uint16(code), value: 0},
		inputEvent{typ: evSyn, code: synReport},
	); err != nil {
		return err
	}
	fmt.Printf("pressed %s\n", name)
	return nil
}

// typeText presses and releases the keys for a US-layout string.
func typeText(fd int, text string) error {
	for _, r := range text {
		code, shift, ok := keyFor(r)
		if !ok {
			continue
		}
		if shift {
			if err := keyEvent(fd, keyLeftShift, 1); err != nil {
				return err
			}
		}
		if err := keyEvent(fd, code, 1); err != nil {
			return err
		}
		if err := keyEvent(fd, code, 0); err != nil {
			return err
		}
		if shift {
			if err := keyEvent(fd, keyLeftShift, 0); err != nil {
				return err
			}
		}
		if err := writeEvents(fd, inputEvent{typ: evSyn, code: synReport}); err != nil {
			return err
		}
		time.Sleep(12 * time.Millisecond)
	}
	fmt.Printf("typed %q\n", text)
	return nil
}

func keyEvent(fd, code, value int) error {
	return writeEvents(fd, inputEvent{typ: evKey, code: uint16(code), value: int32(value)})
}

func writeEvents(fd int, events ...inputEvent) error {
	buf := make([]byte, 0, len(events)*int(unsafe.Sizeof(inputEvent{})))
	for _, ev := range events {
		buf = append(buf, unsafe.Slice((*byte)(unsafe.Pointer(&ev)), int(unsafe.Sizeof(ev)))...)
	}
	_, err := unix.Write(fd, buf)
	return err
}

// keyFor maps a rune to a keycode, and whether shift must be held.
func keyFor(r rune) (code int, shift bool, ok bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return letters[r-'a'], false, true
	case r >= 'A' && r <= 'Z':
		return letters[r-'A'], true, true
	case r >= '1' && r <= '9':
		return key1 + int(r-'1'), false, true
	case r == '0':
		return key0, false, true
	case r == ' ':
		return keySpace, false, true
	case r == '.':
		return 52, false, true
	case r == ',':
		return 51, false, true
	case r == '-':
		return 12, false, true
	case r == '=':
		return 13, false, true
	case r == '/':
		return 53, false, true
	}
	return 0, false, false
}

// letters holds the keycodes of a..z in order, so the two alphabetic ranges can
// be indexed directly.
var letters = [26]int{
	30, 48, 46, 32, 18, 33, 34, 35, 23, 36, 37, 38, 50, // a..m
	49, 24, 25, 16, 19, 31, 20, 22, 47, 17, 45, 21, 44, // n..z
}

// typedKeys lists every keycode typeText may need, so they can all be enabled
// on the device up front.
func typedKeys() []int {
	codes := make([]int, 0, len(letters)+12)
	codes = append(codes, letters[:]...)
	codes = append(codes, key1+0, key1+1, key1+2, key1+3, key1+4, key1+5, key1+6, key1+7, key1+8, key0)
	codes = append(codes, 12, 13, 51, 52, 53)
	return codes
}
