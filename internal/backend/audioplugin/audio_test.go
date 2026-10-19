package audioplugin

import (
	"context"
	"os/exec"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

const sinksJSON = `[
  {"name":"alsa_output.pci-0000_01_00.1.hdmi-stereo","description":"HDMI",
   "mute":false,"volume":{"front-left":{"value_percent":"35%"}}},
  {"name":"bluez_output.14_3F_A6_C6_BB_CC.1","description":"Headphones",
   "mute":true,"volume":{"front-left":{"value_percent":"100%"}}}
]`

func stubBackend(script string) *Backend {
	return &Backend{pactl: func(_ ...string) *exec.Cmd {
		return exec.Command("sh", "-c", script)
	}}
}

// TestQueryListsSinks verifies sinks become items with range/toggle controls.
func TestQueryListsSinks(t *testing.T) {
	b := stubBackend(`printf '%s' '` + sinksJSON + `'`)
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	var items []sdk.Item
	for {
		it, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		items = append(items, it)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].ID != "alsa_output.pci-0000_01_00.1.hdmi-stereo" {
		t.Errorf("item0 id = %q", items[0].ID)
	}
	if len(items[0].Controls) != 2 {
		t.Errorf("item0 controls = %d, want 2", len(items[0].Controls))
	}
	if items[0].Controls[0].Value != 0.35 {
		t.Errorf("volume = %v, want 0.35", items[0].Controls[0].Value)
	}
	if !items[1].Controls[1].Bool {
		t.Errorf("item1 mute = false, want true")
	}
}

// TestQueryFiltersByTitle checks filtering.
func TestQueryFiltersByTitle(t *testing.T) {
	b := stubBackend(`printf '%s' '` + sinksJSON + `'`)
	stream, _ := b.Query(context.Background(), sdk.Query{Text: "hdmi"})
	defer stream.Close()

	got := 0
	for {
		_, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		got++
	}
	if got != 1 {
		t.Errorf("filtered = %d, want 1", got)
	}
}

// TestControlRoutesToPactl verifies volume and mute map to pactl commands.
func TestControlRoutesToPactl(t *testing.T) {
	var args []string
	b := &Backend{pactl: func(a ...string) *exec.Cmd {
		args = a
		return exec.Command("true")
	}}

	if err := b.Control(context.Background(), sdk.Change{Item: "sink-1", Control: "volume", Value: 0.35}); err != nil {
		t.Fatal(err)
	}
	if len(args) < 3 || args[0] != "set-sink-volume" || args[2] != "35%" {
		t.Errorf("volume args = %v, want set-sink-volume sink-1 35%%", args)
	}

	if err := b.Control(context.Background(), sdk.Change{Item: "sink-1", Control: "mute", Bool: true}); err != nil {
		t.Fatal(err)
	}
	if args[0] != "set-sink-mute" || args[2] != "1" {
		t.Errorf("mute args = %v, want set-sink-mute sink-1 1", args)
	}
}
