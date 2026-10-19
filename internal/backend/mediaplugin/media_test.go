package mediaplugin

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// stubPlayerctl dispatches on the first argument, mimicking playerctl enough for
// the tests: --list-all, and -p <player> -f ... status.
func stubPlayerctl(players map[string]string) *Backend {
	return &Backend{playerctl: func(args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "--list-all" {
			list := ""
			for p := range players {
				list += p + "\n"
			}
			return exec.Command("sh", "-c", "printf '%s' '"+strings.TrimSuffix(list, "\n")+"'")
		}
		// -p <player> -f ... status
		if len(args) >= 2 && args[0] == "-p" {
			player := args[1]
			if line, ok := players[player]; ok {
				return exec.Command("sh", "-c", "printf '%s' '"+line+"'")
			}
		}
		return exec.Command("sh", "-c", "printf '%s' ''")
	}}
}

func TestQueryListsPlayers(t *testing.T) {
	b := stubPlayerctl(map[string]string{
		"firefox": "Playing\tCool Song\tArtist X\t30\t240",
		"mpv":     "Paused\t\n\t0\t0",
	})
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	items := []sdk.Item{}
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
	if items[0].Title != "Cool Song" || items[0].Subtitle != "Artist X" {
		t.Errorf("item0 = %q / %q", items[0].Title, items[0].Subtitle)
	}
	// firefox has position range (length 240), mpv does not.
	if len(items[0].Controls) != 2 {
		t.Errorf("item0 controls = %d, want 2 (playback + position)", len(items[0].Controls))
	}
	if len(items[1].Controls) != 1 {
		t.Errorf("item1 controls = %d, want 1 (playback only)", len(items[1].Controls))
	}
}

func TestQueryFilters(t *testing.T) {
	b := stubPlayerctl(map[string]string{
		"firefox": "Playing\tCool Song\tArtist X\t30\t240",
		"mpv":     "Paused\tFilm\tCinema\t0\t0",
	})
	stream, _ := b.Query(context.Background(), sdk.Query{Text: "song"})
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

func TestControlMapsToPlayerctl(t *testing.T) {
	var args []string
	b := &Backend{playerctl: func(a ...string) *exec.Cmd {
		args = a
		return exec.Command("true")
	}}
	if err := b.Control(context.Background(), sdk.Change{Item: "firefox", Control: "playback", Text: "pause"}); err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[2] != "pause" {
		t.Errorf("playback args = %v, want -p firefox pause", args)
	}
	if err := b.Control(context.Background(), sdk.Change{Item: "firefox", Control: "position", Value: 60}); err != nil {
		t.Fatal(err)
	}
	if args[2] != "position" || args[3] != "60" {
		t.Errorf("position args = %v, want position 60", args)
	}
}
