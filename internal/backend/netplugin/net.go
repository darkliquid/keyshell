// Package netplugin is the first-party network plugin. It owns the connection
// domain via nmcli and exposes connect toggles and a Wi-Fi choice control.
package netplugin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// Backend is the network plugin's domain logic.
type Backend struct {
	nmcli func(args ...string) *exec.Cmd

	pskMu   sync.Mutex
	psk     string
	pending bool
}

// PSK returns the current stored passphrase ("" when none yet).
func (b *Backend) PSK() string {
	b.pskMu.Lock()
	defer b.pskMu.Unlock()
	return b.psk
}

// NoteSecretRequest flags that a wireless secret is being asked for, so the
// query surfaces a passphrase prompt.
func (b *Backend) NoteSecretRequest() {
	b.pskMu.Lock()
	b.pending = true
	b.pskMu.Unlock()
}

// setPSK stores the passphrase and clears the pending prompt.
func (b *Backend) setPSK(psk string) {
	b.pskMu.Lock()
	b.psk = psk
	b.pending = false
	b.pskMu.Unlock()
}

// New builds the network backend.
func New() *Backend {
	return &Backend{nmcli: func(args ...string) *exec.Cmd {
		return exec.Command("nmcli", args...)
	}}
}

// Info implements sdk.Backend.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:        "network",
		Name:      "Network",
		Badge:     "NET",
		Trigger:   "&",
		Controls:  []string{"toggle", "choice"},
		Hot:       true,
		StableIDs: true, // connection names are stable
	}
}

// Query lists connections as items with a connect toggle, plus a Wi-Fi choice.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	conns := b.connections()
	lower := strings.ToLower(q.Text)

	var items []sdk.Item
	for _, c := range conns {
		if lower != "" && !strings.Contains(strings.ToLower(c.name), lower) {
			continue
		}
		item := sdk.Item{
			ID: c.name, Title: c.name, Subtitle: c.subtitle(), Glyph: "network",
			Status: "OPEN", StatusTone: "neutral",
			Controls: []sdk.Control{
				{ID: "connect", Kind: "toggle", Label: "Connected", Bool: c.active},
			},
		}
		if c.active {
			item.Status, item.StatusTone = "CONNECTED", "positive"
		}
		items = append(items, item)
	}

	// One Wi-Fi choice item listing available networks.
	if ssids := b.wifiNetworks(); len(ssids) > 0 {
		items = append(items, sdk.Item{
			ID: "wifi", Title: "Wi-Fi", Subtitle: "Select a network",
			Glyph: "wifi",
			Controls: []sdk.Control{
				{ID: "wifi", Kind: "choice", Label: "Network", Options: ssids},
			},
		})
	}

	// A pending secret request surfaces a passphrase prompt.
	if b.wantsPSK() {
		items = append(items, sdk.Item{
			ID: "passphrase", Title: "Enter Wi-Fi passphrase",
			Subtitle: "Required by NetworkManager", Glyph: "key",
			Controls: []sdk.Control{
				{ID: "passphrase", Kind: "text", Label: "Passphrase"},
			},
		})
	}
	return sdk.NewStream(items...), nil
}

func (b *Backend) wantsPSK() bool {
	b.pskMu.Lock()
	defer b.pskMu.Unlock()
	return b.pending
}

// Control applies a connect toggle or a Wi-Fi choice.
func (b *Backend) Control(_ context.Context, c sdk.Change) error {
	switch c.Control {
	case "connect":
		if c.Bool {
			return b.nmcli("connection", "up", c.Item).Run()
		}
		return b.nmcli("connection", "down", c.Item).Run()
	case "wifi":
		return b.nmcli("device", "wifi", "connect", c.Text).Run()
	case "passphrase":
		b.setPSK(c.Text)
		// Connect to the network the user selected, if any.
		return b.nmcli("device", "wifi", "connect", c.Text).Run()
	}
	return fmt.Errorf("network: no control %q", c.Control)
}

type connection struct {
	name, typ, device, state string
	active                   bool
}

func (c connection) subtitle() string {
	parts := []string{}
	if c.typ != "" {
		parts = append(parts, c.typ)
	}
	if c.device != "" {
		parts = append(parts, c.device)
	}
	return strings.Join(parts, " · ")
}

func (b *Backend) connections() []connection {
	out, err := b.nmcli("-t", "-f", "NAME,TYPE,DEVICE,STATE,ACTIVE", "connection", "show").Output()
	if err != nil {
		return nil
	}
	var conns []connection
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, ":")
		if len(f) < 5 {
			continue
		}
		conns = append(conns, connection{
			name: f[0], typ: f[1], device: f[2], state: f[3],
			active: f[4] == "yes",
		})
	}
	return conns
}

func (b *Backend) wifiNetworks() []string {
	out, err := b.nmcli("-t", "-f", "SSID,SIGNAL", "device", "wifi", "list").Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var ssids []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.SplitN(line, ":", 2)
		if len(f) < 1 {
			continue
		}
		ssid := strings.TrimSpace(f[0])
		if ssid == "" || seen[ssid] {
			continue
		}
		seen[ssid] = true
		ssids = append(ssids, ssid)
	}
	return ssids
}
