package powerplugin

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

const profilesOut = "* performance:\n    CpuDriver: amd_pstate\n\n  balanced:\n    CpuDriver: amd_pstate\n\n  power-saver:\n    CpuDriver: amd_pstate\n"

func stubBackend() *Backend {
	return &Backend{
		upower: func(a ...string) *exec.Cmd {
			if len(a) > 0 && a[0] == "-e" {
				return exec.Command("sh", "-c", "printf '%s' '/org/freedesktop/UPower/devices/battery_hidpp_battery_0'")
			}
			return exec.Command("sh", "-c", "printf '%s' 'state: discharging\npercentage: 65%'")
		},
		profiles: func(a ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "printf '%s' '"+profilesOut+"'")
		},
	}
}

func TestQueryReportsBatteryAndProfiles(t *testing.T) {
	b := stubBackend()
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	it, more, err := stream.Next()
	if err != nil || !more {
		t.Fatalf("Next = more=%v err=%v", more, err)
	}
	if !strings.Contains(it.Subtitle, "65%") {
		t.Errorf("subtitle = %q, want battery 65%%", it.Subtitle)
	}
	if len(it.Controls) != 1 || it.Controls[0].Kind != "choice" {
		t.Fatalf("controls = %+v, want one profile choice", it.Controls)
	}
	c := it.Controls[0]
	if c.Text != "performance" {
		t.Errorf("current profile = %q, want performance", c.Text)
	}
	if len(c.Options) != 3 {
		t.Errorf("options = %v, want 3 profiles", c.Options)
	}
}

func TestControlSetsProfile(t *testing.T) {
	var args []string
	b := &Backend{
		profiles: func(a ...string) *exec.Cmd { args = a; return exec.Command("true") },
		upower:   func(a ...string) *exec.Cmd { return exec.Command("true") },
	}
	if err := b.Control(context.Background(), sdk.Change{Control: "profile", Text: "balanced"}); err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "set" || args[1] != "balanced" {
		t.Errorf("args = %v, want set balanced", args)
	}
}

func TestParsePercentage(t *testing.T) {
	if got := parsePercentage("percentage: 65%"); got != 65 {
		t.Errorf("parsePercentage = %v, want 65", got)
	}
	if got := parsePercentage("nope"); got != -1 {
		t.Errorf("parsePercentage(nope) = %v, want -1", got)
	}
}
