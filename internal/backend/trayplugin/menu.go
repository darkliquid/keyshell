// Menu reading for tray applets: the plugin acts as a DBusMenu client
// (com.canonical.dbusmenu) and flattens an applet's layout into scoped child
// items the host's navigation can display.
package trayplugin

import (
	"github.com/darkliquid/keyshell/internal/plugin/sdk"
	"github.com/godbus/dbus/v5"
)

// dbusMenuIface is the menu interface name.
const dbusMenuIface = "com.canonical.dbusmenu"

// menuEntry is one parsed layout node.
type menuEntry struct {
	id       int32
	label    string
	kind     string
	enabled  bool
	children []menuEntry
}

// menuItems calls GetLayout on the applet's menu and flattens it into SDK
// items. depth of 2 yields the top level plus one level of children.
func (b *Backend) menuItems(service string, depth int32) []sdk.Item {
	menuBus := b.menuBus(service)
	if menuBus == "" {
		return nil
	}
	obj := b.conn.Object(menuBus, dbus.ObjectPath("/MenuBar"))
	var revision int32
	var layout dbus.Variant
	if err := obj.Call(dbusMenuIface+".GetLayout", 0, int32(0), depth, []string{"label", "type", "enabled"}).Store(&revision, &layout); err != nil {
		return nil
	}
	root := parseLayout(layout.Value())
	return flattenEntries(root, 0)
}

// menuBus reads the applet's Menu property (the bus name of its menu).
func (b *Backend) menuBus(service string) string {
	if b.conn == nil {
		return ""
	}
	obj := b.conn.Object(service, dbus.ObjectPath(itemPath))
	var menu string
	if err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.kde.StatusNotifierItem", "Menu").Store(&menu); err != nil {
		return ""
	}
	return menu
}

// parseLayout decodes the variable-depth (ia{sv}av) menu layout tree.
func parseLayout(v interface{}) []menuEntry {
	entries, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]menuEntry, 0, len(entries))
	for _, raw := range entries {
		tuple, ok := raw.([]interface{})
		if !ok || len(tuple) < 3 {
			continue
		}
		id, _ := tuple[0].(int32)
		props, _ := tuple[1].(map[string]dbus.Variant)
		var children []menuEntry
		if childRaw, ok := tuple[2].([]interface{}); ok {
			children = parseLayout(childRaw)
		}
		out = append(out, menuEntry{
			id:       id,
			label:    propString(props, "label"),
			kind:     propString(props, "type"),
			enabled:  propBool(props, "enabled", true),
			children: children,
		})
	}
	return out
}

func propString(props map[string]dbus.Variant, key string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

func propBool(props map[string]dbus.Variant, key string, def bool) bool {
	if v, ok := props[key]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return def
}

// flattenEntries turns a parsed layout into items. A separator or a disabled
// entry still appears (subtitle indicates state); children are appended after
// their parent as flats so the host's flat scoped list sees them all.
func flattenEntries(entries []menuEntry, depth int) []sdk.Item {
	var items []sdk.Item
	for _, e := range entries {
		if e.kind == "separator" {
			continue
		}
		title := e.label
		if title == "" {
			title = "(unnamed)"
		}
		item := sdk.Item{
			ID:    sdkItemID(e.id, depth),
			Title: title,
			Glyph: "menu",
		}
		if !e.enabled {
			item.Subtitle = "disabled"
		}
		items = append(items, item)
		items = append(items, flattenEntries(e.children, depth+1)...)
	}
	return items
}

func sdkItemID(id int32, depth int) string {
	return "menu-" + itoa(int(id)) + "-" + itoa(depth)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
