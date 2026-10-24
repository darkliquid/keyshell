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
		if o.CurrentMode != nil && *o.CurrentMode >= 0 && *o.CurrentMode < len(o.Modes) {
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
		if o.Rect.Width > 0 {
			return o.Rect.Width, o.Rect.Height, true
		}
		if o.Current.Width > 0 {
			return o.Current.Width, o.Current.Height, true
		}
	}
	return 0, 0, false
}

// wlOutputsSize is a best-effort fallback for other compositors using wlr-randr
// or kscreen-doctor, which print "<width>x<height>" in their output.
func wlOutputsSize() (int, int, bool) {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		return 0, 0, false
	}
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
			lower := strings.ToLower(line)
			if strings.Contains(lower, "physical") || strings.HasSuffix(strings.TrimSpace(lower), "mm") {
				continue
			}
			if w, h, ok := parseSize(line); ok {
				return w, h, true
			}
		}
	}
	return 0, 0, false
}

// isPhysicalLine checks whether a line describes physical display dimensions.
func isPhysicalLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "physical") || strings.HasSuffix(strings.TrimSpace(lower), "mm")
}

// parseOutputLine parses a resolution from an output line, skipping physical dimensions.
func parseOutputLine(line string) (int, int, bool) {
	if isPhysicalLine(line) {
		return 0, 0, false
	}
	return parseSize(line)
}

// parseSize finds the first WxH token in a line.
func parseSize(line string) (int, int, bool) {
	fields := strings.Fields(line)
	for _, f := range fields {
		parts := strings.SplitN(strings.Trim(f, ",;()"), "x", 2)
		if len(parts) != 2 {
			continue
		}
		hStr := strings.Split(parts[1], "@")[0]
		hStr = strings.TrimSuffix(strings.ToLower(hStr), "px")
		w, err1 := strconv.Atoi(parts[0])
		h, err2 := strconv.Atoi(hStr)
		if err1 == nil && err2 == nil && w > 0 && h > 0 {
			return w, h, true
		}
	}
	return 0, 0, false
}
