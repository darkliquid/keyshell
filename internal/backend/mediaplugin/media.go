// Package mediaplugin is the first-party media plugin. It owns the media-player
// domain via playerctl and exposes playback/position controls the host renders.
package mediaplugin

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// Backend is the media plugin's domain logic.
type Backend struct {
	playerctl func(args ...string) *exec.Cmd
}

// New builds the media backend.
func New() *Backend {
	return &Backend{playerctl: func(args ...string) *exec.Cmd {
		return exec.Command("playerctl", args...)
	}}
}

// Info implements sdk.Backend.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:         "media",
		Name:       "Media",
		Badge:      "MEDIA",
		Trigger:    ">",
		Controls:   []string{"range", "choice"},
		Hot:        true,
		StableIDs:  true, // player names are stable
		Searchable: true,
	}
}

// Query lists players as items with playback and position controls.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	players, err := b.listPlayers()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(q.Text)

	var items []sdk.Item
	for _, p := range players {
		status, title, artist, pos, length := b.playerState(p)
		display := title
		if display == "" {
			display = p
		}
		if lower != "" && !strings.Contains(strings.ToLower(display), lower) {
			continue
		}
		controls := []sdk.Control{
			{ID: "playback", Kind: "choice", Label: "Playback", Text: status,
				Options: []string{"play", "pause", "next", "previous"}},
		}
		if length > 0 {
			controls = append(controls, sdk.Control{
				ID: "position", Kind: "progress", Label: formatClock(pos), Text: formatClock(length),
				Value: pos, Min: 0, Max: length, Step: 1,
			})
		}
		item := sdk.Item{
			ID: p, Title: display, Subtitle: artist,
			Glyph: "media", Controls: controls,
			Status: strings.ToUpper(status), StatusTone: "info",
		}
		if !strings.EqualFold(status, "Playing") {
			item.StatusTone = "neutral"
		}
		items = append(items, item)
	}
	return sdk.NewStream(items...), nil
}

// Control applies a playback or position change.
func (b *Backend) Control(ctx context.Context, c sdk.Change) error {
	// An empty item targets the default player: omit -p.
	args := []string{}
	if c.Item != "" {
		args = append(args, "-p", c.Item)
	}
	switch c.Control {
	case "playback":
		arg := c.Text
		if arg == "" {
			arg = "play-pause"
		}
		args = append(args, arg)
		return b.playerctl(args...).Run()
	case "position":
		sec := strconv.FormatFloat(c.Value, 'f', 0, 64)
		args = append(args, "position", sec)
		return b.playerctl(args...).Run()
	}
	return fmt.Errorf("media: no control %q", c.Control)
}

// formatClock renders seconds as m:ss or h:mm:ss for the timeline label.
func formatClock(sec float64) string {
	if sec < 0 || math.IsInf(sec, 0) || math.IsNaN(sec) {
		return "0:00"
	}
	total := int(sec)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (b *Backend) listPlayers() ([]string, error) {
	out, err := b.playerctl("--list-all").Output()
	if err != nil {
		return nil, err
	}
	var players []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			players = append(players, line)
		}
	}
	return players, nil
}

func (b *Backend) playerState(player string) (status, title, artist string, pos, length float64) {
	cmd := b.playerctl("-p", player, "-f", "{{status}}\t{{title}}\t{{artist}}\t{{position}}\t{{mpris:length}}", "status")
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", 0, 0
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) >= 3 {
		status, title, artist = parts[0], parts[1], parts[2]
	}
	if len(parts) >= 4 {
		pos, _ = strconv.ParseFloat(parts[3], 64)
	}
	if len(parts) >= 5 {
		length, _ = strconv.ParseFloat(parts[4], 64)
	}
	return
}
