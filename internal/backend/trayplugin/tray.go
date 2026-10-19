// Package trayplugin provides the tray backend. It lists registered
// StatusNotifierItem applets as cards. Reading and acting on each applet's
// DBusMenu is the recorded follow-on; the first cut surfaces the applets.
package trayplugin

import (
	"context"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
	"github.com/godbus/dbus/v5"
)

const itemPath = "/StatusNotifierItem"

// Backend is the tray plugin.
type Backend struct {
	watcher interface {
		Items() []string
	}
	conn *dbus.Conn
}

// New builds the tray backend over a live session connection and watcher.
func New(conn *dbus.Conn, w *Watcher) *Backend {
	return &Backend{conn: conn, watcher: w}
}

// Info implements sdk.Backend.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:        "tray",
		Name:      "Tray",
		Badge:     "TRAY",
		Trigger:   "^",
		Controls:  nil,
		Hot:       true,
		StableIDs: true, // applet service names are stable for a session
	}
}

// Query lists applets as items. A scoped query (a child level) returns that
// applet's menu items.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	if len(q.Scope) > 0 {
		return sdk.NewStream(b.menuItems(q.Scope[0], 2)...), nil
	}
	services := b.watcher.Items()
	items := make([]sdk.Item, 0, len(services))
	for _, svc := range services {
		title := b.title(svc)
		if title == "" {
			title = svc
		}
		if q.Text != "" && !containsFold(title, q.Text) {
			continue
		}
		items = append(items, sdk.Item{
			ID: svc, Title: title, Subtitle: "Tray applet",
			Glyph: "tray", HasChildren: b.menuBus(svc) != "",
		})
	}
	return sdk.NewStream(items...), nil
}

func (b *Backend) Control(context.Context, sdk.Change) error { return nil }

// Invoke executes a menu-item click on the applet's DBusMenu.
func (b *Backend) Invoke(_ context.Context, i sdk.Invoke) error {
	if i.Action != "invoke-menu" {
		return nil
	}
	service, id, ok := parseMenuItemID(i.Item)
	if !ok || b.conn == nil {
		return nil
	}
	menuBus := b.menuBus(service)
	if menuBus == "" {
		return nil
	}
	obj := b.conn.Object(menuBus, dbus.ObjectPath("/MenuBar"))
	// Event(id, "clicked", variant(0), timestamp=0)
	call := obj.Call(dbusMenuIface+".Event", 0, id, "clicked", dbus.MakeVariant(0), int32(0))
	return call.Err
}

// parseMenuItemID recovers service and menu id from "menu:<service>:<id>".
func parseMenuItemID(item string) (string, int32, bool) {
	rest := item
	if len(rest) < 5 || rest[:5] != "menu:" {
		return "", 0, false
	}
	rest = rest[5:]
	colon := indexByte(rest, ':')
	if colon < 0 {
		return "", 0, false
	}
	service := rest[:colon]
	idPart := rest[colon+1:]
	var id int32
	for _, c := range idPart {
		if c < '0' || c > '9' {
			return "", 0, false
		}
		id = id*10 + (c - '0')
	}
	return service, id, true
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// title reads the applet's Title property, best-effort.
func (b *Backend) title(service string) string {
	if b.conn == nil {
		return ""
	}
	obj := b.conn.Object(service, dbus.ObjectPath(itemPath))
	var title string
	if err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.kde.StatusNotifierItem", "Title").Store(&title); err != nil {
		return ""
	}
	return title
}

func containsFold(hay, needle string) bool {
	return indexFold(hay, needle) >= 0
}

func indexFold(hay, needle string) int {
	lh := toLower(hay)
	ln := toLower(needle)
	for i := 0; i+len(ln) <= len(lh); i++ {
		if lh[i:i+len(ln)] == ln {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
