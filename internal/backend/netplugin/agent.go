// SecretAgent implements the NetworkManager secret-agent interface so keyshell
// can answer a passphrase prompt for a Wi-Fi connection. It is exported on the
// session bus by the network plugin; the host renders the capture control.
package netplugin

import (
	"github.com/godbus/dbus/v5"
)

const (
	nmService  = "org.freedesktop.NetworkManager"
	agentPath  = "/org/freedesktop/NetworkManager/SecretAgent"
	agentIface = "org.freedesktop.NetworkManager.SecretAgent"
)

// Agent is the exported secret-agent object. It talks to the network backend so
// an unanswered wireless request can surface a password prompt.
type Agent struct {
	backend *Backend
}

// NewAgent builds an agent over the network backend.
func NewAgent(b *Backend) *Agent {
	return &Agent{backend: b}
}

// Register exports the agent and registers it with NetworkManager.
func (a *Agent) Register(conn *dbus.Conn) error {
	if err := conn.Export(a, dbus.ObjectPath(agentPath), agentIface); err != nil {
		return err
	}
	nm := conn.Object(nmService, "/org/freedesktop/NetworkManager")
	call := nm.Call("org.freedesktop.NetworkManager.RegisterAgent", 0, agentIface)
	return call.Err
}

// GetSecrets returns the psk for 802-11-wireless-security settings, and an
// empty map for everything else so NetworkManager re-asks.
func (a *Agent) GetSecrets(connection map[string]map[string]dbus.Variant, _ string, setting string, _ []string, _ uint32) (map[string]map[string]dbus.Variant, *dbus.Error) {
	if setting != "802-11-wireless-security" {
		return map[string]map[string]dbus.Variant{}, nil
	}
	if a.backend == nil {
		return map[string]map[string]dbus.Variant{}, nil
	}
	psk := a.backend.PSK()
	if psk == "" {
		// We don't know the secret yet; flag a pending prompt so the query
		// surfaces a passphrase card. NetworkManager will re-ask.
		a.backend.NoteSecretRequest()
		return map[string]map[string]dbus.Variant{}, nil
	}
	return map[string]map[string]dbus.Variant{
		"802-11-wireless-security": {"psk": dbus.MakeVariant(psk)},
	}, nil
}

// SaveSecrets is a no-op (we don't persist secrets).
func (a *Agent) SaveSecrets(_ map[string]map[string]dbus.Variant, _ string) *dbus.Error {
	return nil
}

// DeleteSecrets is a no-op.
func (a *Agent) DeleteSecrets(_ map[string]map[string]dbus.Variant, _ string) *dbus.Error {
	return nil
}
