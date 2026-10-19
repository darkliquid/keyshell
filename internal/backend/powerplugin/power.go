// Package powerplugin is the first-party power plugin. It reports battery level
// via upower and exposes the power profile as a choice control via
// powerprofilesctl.
package powerplugin

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// Backend is the power plugin's domain logic.
type Backend struct {
	upower   func(args ...string) *exec.Cmd
	profiles func(args ...string) *exec.Cmd
}

// New builds the power backend.
func New() *Backend {
	return &Backend{
		upower:   func(args ...string) *exec.Cmd { return exec.Command("upower", args...) },
		profiles: func(args ...string) *exec.Cmd { return exec.Command("powerprofilesctl", args...) },
	}
}

// Info implements sdk.Backend.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:        "power",
		Name:      "Power",
		Badge:     "POWER",
		Trigger:   "%",
		Controls:  []string{"choice"},
		Hot:       true,
		StableIDs: true,
	}
}

// Query reports battery level and the power profile choice.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	profile, profiles := b.profileState()
	battery := b.batteryPercent()

	subtitle := ""
	if battery >= 0 {
		subtitle = fmt.Sprintf("%.0f%% battery", battery)
	}
	item := sdk.Item{
		ID: "power", Title: "Power",
		Subtitle: subtitle, Glyph: "power",
		Controls: []sdk.Control{
			{ID: "profile", Kind: "choice", Label: "Profile", Text: profile, Options: profiles},
		},
	}
	return sdk.NewStream(item), nil
}

// Control sets the power profile.
func (b *Backend) Control(_ context.Context, c sdk.Change) error {
	if c.Control == "profile" && c.Text != "" {
		return b.profiles("set", c.Text).Run()
	}
	return fmt.Errorf("power: no control %q", c.Control)
}

func (b *Backend) profileState() (current string, profiles []string) {
	out, err := b.profiles("list").Output()
	if err != nil {
		return "", nil
	}
	re := regexp.MustCompile(`^(\*)?\s{0,2}(\S+):`)
	for _, line := range strings.Split(string(out), "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[2]
		profiles = append(profiles, name)
		if m[1] == "*" {
			current = name
		}
	}
	return current, profiles
}

func (b *Backend) batteryPercent() float64 {
	devs, err := b.upower("-e").Output()
	if err != nil {
		return -1
	}
	for _, dev := range strings.Split(string(devs), "\n") {
		if !strings.Contains(dev, "battery") {
			continue
		}
		info, err := b.upower("-i", strings.TrimSpace(dev)).Output()
		if err != nil {
			continue
		}
		return parsePercentage(string(info))
	}
	return -1
}

var percentRe = regexp.MustCompile(`percentage:\s*([0-9.]+)%`)

func parsePercentage(s string) float64 {
	m := percentRe.FindStringSubmatch(s)
	if m == nil {
		return -1
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}
