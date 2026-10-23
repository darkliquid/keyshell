package netplugin

import (
	"context"
	"os/exec"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

const conns = "Disable WiFi:802-11-wireless:wlan0:activated:yes\nlo:loopback:lo:activated:yes\nWired connection 1:802-3-ethernet:::no\n"

const wifi = "HomeNet:79\nLab5G:64\nHomeNet:75\n"

// stubNmcli routes by the cluster of args that identify the request.
func stubNmcli() *Backend {
	return &Backend{nmcli: func(args ...string) *exec.Cmd {
		joined := joinArgs(args)
		switch {
		case contains(joined, "device") && contains(joined, "wifi") && contains(joined, "list"):
			return exec.Command("sh", "-c", "printf '%s' '"+wifi+"'")
		case contains(joined, "connection") && contains(joined, "show"):
			return exec.Command("sh", "-c", "printf '%s' '"+conns+"'")
		}
		return exec.Command("true")
	}}
}

func joinArgs(a []string) string {
	out := ""
	for _, s := range a {
		out += s + " "
	}
	return out
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && index(hay, needle) >= 0
}

func index(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestQueryListsConnectionsAndWifi(t *testing.T) {
	b := stubNmcli()
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
	// 3 connections (Disable WiFi, lo, Wired connection 1) + wifi choice item.
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4 (3 connections + wifi choice)", len(items))
	}
	var wired, active *sdk.Item
	for i := range items {
		switch items[i].ID {
		case "Wired connection 1":
			wired = &items[i]
		case "Disable WiFi":
			active = &items[i]
		}
	}
	if active == nil || active.Status != "CONNECTED" || active.StatusTone != "positive" {
		t.Errorf("active connection = %+v, want CONNECTED/positive", active)
	}
	if wired == nil || wired.Status != "OPEN" || wired.StatusTone != "neutral" {
		t.Errorf("inactive connection = %+v, want OPEN/neutral", wired)
	}
	if wired != nil && wired.Icon != "network-wired" {
		t.Errorf("wired icon = %q, want network-wired", wired.Icon)
	}

	// The wifi choice item exists last.
	last := items[len(items)-1]
	if last.ID != "wifi" || len(last.Controls) != 1 || last.Controls[0].Kind != "choice" || last.Icon != "network-wireless" {
		t.Errorf("wifi item = %+v", last)
	}
	if len(last.Controls[0].Options) != 2 {
		t.Errorf("wifi options = %v, want 2 unique (HomeNet, Lab5G)", last.Controls[0].Options)
	}
}

func TestControlMapsToNmcli(t *testing.T) {
	var args []string
	b := &Backend{nmcli: func(a ...string) *exec.Cmd {
		args = a
		return exec.Command("true")
	}}
	if err := b.Control(context.Background(), sdk.Change{Item: "HomeNet", Control: "connect", Bool: true}); err != nil {
		t.Fatal(err)
	}
	if joinArgs(args) != "connection up HomeNet " {
		t.Errorf("connect args = %v", args)
	}
	if err := b.Control(context.Background(), sdk.Change{Item: "wifi", Control: "wifi", Text: "Lab5G"}); err != nil {
		t.Fatal(err)
	}
	if joinArgs(args) != "device wifi connect Lab5G " {
		t.Errorf("wifi args = %v", args)
	}
}

func TestPendingPromptAppearsAndControlClears(t *testing.T) {
	b := stubNmcli()
	b.NoteSecretRequest()

	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	items := drainItems(stream)
	found := false
	for _, it := range items {
		if it.ID == "passphrase" {
			found = true
			if len(it.Controls) != 1 || it.Controls[0].Kind != "text" {
				t.Errorf("passphrase controls = %+v", it.Controls)
			}
		}
	}
	if !found {
		t.Fatal("no pending passphrase item when a secret is requested")
	}

	// Submitting the passphrase stores it and clears the pending prompt.
	if err := b.Control(context.Background(), sdk.Change{Control: "passphrase", Text: "secret123"}); err != nil {
		t.Fatal(err)
	}
	if b.PSK() != "secret123" {
		t.Errorf("PSK = %q, want secret123", b.PSK())
	}
	if b.wantsPSK() {
		t.Error("pending flag should clear after the passphrase is stored")
	}
}

func drainItems(stream sdk.Stream) []sdk.Item {
	defer stream.Close()
	var items []sdk.Item
	for {
		it, more, err := stream.Next()
		if err != nil || !more {
			return items
		}
		items = append(items, it)
	}
}
