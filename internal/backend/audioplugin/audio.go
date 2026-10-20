// Package audioplugin is the first-party audio plugin. It runs as a subprocess
// via the SDK and controls sinks through pactl. It is the reference for what a
// real plugin looks like: domain logic owned by the plugin, with range/toggle
// controls the host renders; and its wpctl/pactl choice is an implementation
// detail, not part of the host.
package audioplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// Backend is the audio plugin's domain logic.
type Backend struct {
	// pactl is injectable for tests.
	pactl func(args ...string) *exec.Cmd
}

// New builds the audio backend.
func New() *Backend {
	return &Backend{pactl: func(args ...string) *exec.Cmd {
		return exec.Command("pactl", args...)
	}}
}

// Info implements sdk.Backend.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:        "audio",
		Name:      "Audio",
		Badge:     "AUDIO",
		Trigger:   "!",
		Controls:  []string{"range", "toggle"},
		Hot:       true,
		StableIDs: true, // sink ids are stable
	}
}

// Query lists sinks as items with volume and mute controls.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	cmd := b.pactl("-f", "json", "list", "sinks")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var sinks []sink
	if err := json.Unmarshal(out, &sinks); err != nil {
		return nil, err
	}

	lower := strings.ToLower(q.Text)
	defaultSink := b.defaultSink()
	var items []sdk.Item
	for _, s := range sinks {
		title := s.Description
		if title == "" {
			title = s.Name
		}
		if lower != "" && !strings.Contains(strings.ToLower(title), lower) && !strings.Contains(strings.ToLower(s.Name), lower) {
			continue
		}
		volume := s.VolumePercent()
		item := sdk.Item{
			ID: s.Name, Title: title, Subtitle: s.Name, Glyph: "audio",
			Badge: "OUTPUT", StatusTone: "info",
			Controls: []sdk.Control{
				{ID: "volume", Kind: "range", Label: "Volume", Value: volume, Min: 0, Max: 1, Step: 0.01,
					Display: "card", Primary: true},
				{ID: "mute", Kind: "toggle", Label: "Mute", Bool: s.Mute},
			},
		}
		if s.Name == defaultSink {
			item.Status = "DEFAULT"
		}
		items = append(items, item)
	}
	return sdk.NewStream(items...), nil
}

// Control applies a value change to a sink. An empty item resolves to the
// default sink, which is how the global IPC verbs target audio.
func (b *Backend) Control(_ context.Context, c sdk.Change) error {
	target := c.Item
	if target == "" {
		target = b.defaultSink()
		if target == "" {
			return fmt.Errorf("audio: no sink to control")
		}
	}
	switch c.Control {
	case "volume":
		pct := strconv.FormatFloat(c.Value*100, 'f', 0, 64)
		return b.pactl("set-sink-volume", target, pct+"%").Run()
	case "volume-delta":
		cur := b.sinkVolume(target)
		next := cur + c.Value
		if next < 0 {
			next = 0
		}
		if next > 1 {
			next = 1
		}
		pct := strconv.FormatFloat(next*100, 'f', 0, 64)
		return b.pactl("set-sink-volume", target, pct+"%").Run()
	case "mute":
		state := "0"
		if c.Bool {
			state = "1"
		}
		return b.pactl("set-sink-mute", target, state).Run()
	}
	return fmt.Errorf("audio: no control %q", c.Control)
}

func (b *Backend) sinkVolume(name string) float64 {
	out, err := b.pactl("-f", "json", "list", "sinks").Output()
	if err != nil {
		return 0
	}
	var sinks []sink
	if json.Unmarshal(out, &sinks) != nil {
		return 0
	}
	for _, s := range sinks {
		if s.Name == name {
			return s.VolumePercent()
		}
	}
	return 0
}

func (b *Backend) defaultSink() string {
	out, err := b.pactl("get-default-sink").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// sink is the subset of pactl's JSON sink object we use.
type sink struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Mute        bool   `json:"mute"`
	Volume      struct {
		FrontLeft struct {
			ValuePercent string `json:"value_percent"`
		} `json:"front-left"`
	} `json:"volume"`
}

// VolumePercent parses e.g. "35%" into 0.35.
func (s sink) VolumePercent() float64 {
	raw := s.Volume.FrontLeft.ValuePercent
	raw = strings.TrimSuffix(raw, "%")
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return v / 100
}
