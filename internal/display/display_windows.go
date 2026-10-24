//go:build windows

package display

import "syscall"

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	// smCxScreen is the SM_CXSCREEN system metric index for GetSystemMetrics (width of the primary display monitor in pixels).
	smCxScreen = 0
	// smCyScreen is the SM_CYSCREEN system metric index for GetSystemMetrics (height of the primary display monitor in pixels).
	smCyScreen = 1
)

// platformPrimary retrieves the width and height of the primary display monitor
// in pixels using Windows user32.dll GetSystemMetrics. Returns (0, 0) if user32
// is unavailable or the dimensions are non-positive.
func platformPrimary() (int, int) {
	if err := getSystemMetrics.Find(); err != nil {
		return 0, 0
	}
	w, _, _ := getSystemMetrics.Call(uintptr(smCxScreen))
	h, _, _ := getSystemMetrics.Call(uintptr(smCyScreen))
	if int32(w) <= 0 || int32(h) <= 0 {
		return 0, 0
	}
	return int(w), int(h)
}
