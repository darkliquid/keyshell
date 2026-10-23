// Menu reading for tray applets: the plugin acts as a DBusMenu client
// (com.canonical.dbusmenu) and surfaces an applet's layout into scoped child
// items the host's navigation can display, supporting nested submenus.
package trayplugin

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
	"github.com/godbus/dbus/v5"
)

// dbusMenuIface is the menu interface name.
const dbusMenuIface = "com.canonical.dbusmenu"

// menuEntry is one parsed layout node.
type menuEntry struct {
	id          int32
	label       string
	kind        string
	icon        string
	toggleType  string
	toggleState int
	enabled     bool
	children    []menuEntry
}

// menuItems calls GetLayout on the applet's menu and produces SDK items for the
// current scope depth.
func (b *Backend) menuItems(scope []string) []sdk.Item {
	if len(scope) == 0 {
		return nil
	}
	applet := scope[0]
	menuPath := b.menuPath(applet)
	if menuPath == "" || b.conn == nil {
		return nil
	}
	svc, _ := splitItem(applet)
	obj := b.conn.Object(svc, dbus.ObjectPath(menuPath))

	targetID := int32(0)
	if len(scope) > 1 {
		if _, id, ok := parseMenuItemID(scope[len(scope)-1]); ok {
			targetID = id
		}
	}

	// 1. Notify server menu is about to show so dynamic labels and items are populated.
	var needUpdate bool
	_ = obj.Call(dbusMenuIface+".AboutToShow", 0, targetID).Store(&needUpdate)

	// 2. Fetch layout. Pass empty property list to request all properties per spec.
	var revision uint32
	var layout dbus.Variant
	call := obj.Call(dbusMenuIface+".GetLayout", 0, targetID, int32(-1), []string{})
	if call.Err != nil {
		call = obj.Call(dbusMenuIface+".GetLayout", 0, targetID, int32(-1), []string{
			"label", "type", "enabled", "icon-name", "children-display", "toggle-type", "toggle-state",
		})
	}
	if call.Err != nil {
		return nil
	}
	if err := call.Store(&revision, &layout); err != nil {
		return nil
	}

	entries := parseLayout(layout.Value())
	if len(scope) > 1 {
		// If the server answered for targetID, entries[0] may be targetID whose
		// children are the items we want to show.
		if len(entries) > 0 && entries[0].id == targetID && len(entries[0].children) > 0 {
			entries = entries[0].children
		} else {
			node, found := findNode(entries, targetID)
			if found && len(node.children) > 0 {
				entries = node.children
			}
		}
	}
	return entriesToItems(entries, applet)
}

func entriesToItems(entries []menuEntry, applet string) []sdk.Item {
	var items []sdk.Item
	for _, e := range entries {
		if e.kind == "separator" {
			continue
		}
		title := cleanMenuText(e.label)
		if title == "" {
			title = "(unnamed)"
		}
		hasChildren := len(e.children) > 0
		item := sdk.Item{
			ID:          menuItemID(applet, e.id),
			Title:       title,
			Icon:        e.icon,
			Glyph:       "menu",
			HasChildren: hasChildren,
		}
		if !hasChildren {
			item.Actions = []sdk.Action{{
				Type: "plugin", Value: "invoke-menu",
			}}
		}
		if !e.enabled {
			item.Subtitle = "disabled"
		}
		if e.toggleType != "" {
			if e.toggleState == 1 {
				item.Status = "ON"
				item.StatusTone = "positive"
			} else {
				item.Status = "OFF"
				item.StatusTone = "neutral"
			}
		}
		items = append(items, item)
	}
	return items
}

func findNode(entries []menuEntry, id int32) (menuEntry, bool) {
	for _, e := range entries {
		if e.id == id {
			return e, true
		}
		if child, ok := findNode(e.children, id); ok {
			return child, true
		}
	}
	return menuEntry{}, false
}

func cleanMenuText(s string) string {
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "&", "")
	return strings.TrimSpace(s)
}

// parseLayout decodes the variable-depth (ia{sv}av) menu layout tree.
func parseLayout(v interface{}) []menuEntry {
	for {
		if variant, ok := v.(dbus.Variant); ok {
			v = variant.Value()
		} else {
			break
		}
	}
	if node, ok := parseNode(v); ok {
		if node.id == 0 && len(node.children) > 0 {
			return node.children
		}
		if len(node.children) > 0 && node.label == "" {
			return node.children
		}
		return []menuEntry{node}
	}
	slice := extractSlice(v)
	if len(slice) > 0 {
		var out []menuEntry
		for _, raw := range slice {
			if node, ok := parseNode(raw); ok {
				out = append(out, node)
			}
		}
		return out
	}
	return nil
}

func parseNode(v interface{}) (menuEntry, bool) {
	for {
		if variant, ok := v.(dbus.Variant); ok {
			v = variant.Value()
		} else {
			break
		}
	}
	if v == nil {
		return menuEntry{}, false
	}

	var rawID interface{}
	var rawProps interface{}
	var rawChildren interface{}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.Len() < 3 {
			return menuEntry{}, false
		}
		rawID = rv.Index(0).Interface()
		rawProps = rv.Index(1).Interface()
		rawChildren = rv.Index(2).Interface()
	case reflect.Struct:
		if rv.NumField() < 3 {
			return menuEntry{}, false
		}
		rawID = rv.Field(0).Interface()
		rawProps = rv.Field(1).Interface()
		rawChildren = rv.Field(2).Interface()
	default:
		return menuEntry{}, false
	}

	id := toInt32(rawID)
	props := extractProps(rawProps)

	childSlice := extractSlice(rawChildren)
	var children []menuEntry
	for _, c := range childSlice {
		if child, ok := parseNode(c); ok {
			children = append(children, child)
		}
	}

	label := getStringProp(props, "label", "Label", "text", "title", "accessible-desc", "tooltip")
	kind := getStringProp(props, "type", "Type")
	icon := getStringProp(props, "icon-name", "icon", "Icon")
	toggleType := getStringProp(props, "toggle-type")
	toggleState := getIntProp(props, 0, "toggle-state")
	enabled := getBoolProp(props, true, "enabled", "Enabled")

	return menuEntry{
		id:          id,
		label:       cleanMenuText(label),
		kind:        kind,
		icon:        icon,
		toggleType:  toggleType,
		toggleState: toggleState,
		enabled:     enabled,
		children:    children,
	}, true
}

func toInt32(v interface{}) int32 {
	for {
		if variant, ok := v.(dbus.Variant); ok {
			v = variant.Value()
		} else {
			break
		}
	}
	switch n := v.(type) {
	case int32:
		return n
	case int:
		return int32(n)
	case uint32:
		return int32(n)
	case int64:
		return int32(n)
	case uint64:
		return int32(n)
	case float64:
		return int32(n)
	}
	return 0
}

func extractSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	for {
		if variant, ok := v.(dbus.Variant); ok {
			v = variant.Value()
		} else {
			break
		}
	}
	if s, ok := v.([]interface{}); ok {
		return s
	}
	if s, ok := v.([]dbus.Variant); ok {
		res := make([]interface{}, len(s))
		for i, x := range s {
			res[i] = x
		}
		return res
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		res := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			res[i] = rv.Index(i).Interface()
		}
		return res
	}
	return nil
}

func extractProps(raw interface{}) map[string]interface{} {
	if raw == nil {
		return nil
	}
	for {
		if v, ok := raw.(dbus.Variant); ok {
			raw = v.Value()
		} else {
			break
		}
	}
	out := make(map[string]interface{})
	switch m := raw.(type) {
	case map[string]dbus.Variant:
		for k, val := range m {
			out[k] = val.Value()
		}
	case map[string]interface{}:
		for k, val := range m {
			for {
				if v, ok := val.(dbus.Variant); ok {
					val = v.Value()
				} else {
					break
				}
			}
			out[k] = val
		}
	case map[interface{}]interface{}:
		for k, val := range m {
			for {
				if v, ok := val.(dbus.Variant); ok {
					val = v.Value()
				} else {
					break
				}
			}
			out[fmt.Sprint(k)] = val
		}
	default:
		rv := reflect.ValueOf(raw)
		if rv.Kind() == reflect.Map {
			for _, key := range rv.MapKeys() {
				val := rv.MapIndex(key).Interface()
				for {
					if v, ok := val.(dbus.Variant); ok {
						val = v.Value()
					} else {
						break
					}
				}
				out[fmt.Sprint(key.Interface())] = val
			}
		}
	}
	return out
}

func getStringProp(props map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		val, ok := props[key]
		if !ok || val == nil {
			continue
		}
		for {
			if v, ok := val.(dbus.Variant); ok {
				val = v.Value()
			} else {
				break
			}
		}
		switch s := val.(type) {
		case string:
			if s != "" {
				return s
			}
		case []byte:
			if len(s) > 0 {
				return string(s)
			}
		case fmt.Stringer:
			str := s.String()
			if str != "" {
				return str
			}
		}
	}
	return ""
}

func getBoolProp(props map[string]interface{}, def bool, keys ...string) bool {
	for _, key := range keys {
		val, ok := props[key]
		if !ok || val == nil {
			continue
		}
		for {
			if v, ok := val.(dbus.Variant); ok {
				val = v.Value()
			} else {
				break
			}
		}
		switch b := val.(type) {
		case bool:
			return b
		case int:
			return b != 0
		case int32:
			return b != 0
		}
	}
	return def
}

func getIntProp(props map[string]interface{}, def int, keys ...string) int {
	for _, key := range keys {
		val, ok := props[key]
		if !ok || val == nil {
			continue
		}
		for {
			if v, ok := val.(dbus.Variant); ok {
				val = v.Value()
			} else {
				break
			}
		}
		switch n := val.(type) {
		case int:
			return n
		case int32:
			return int(n)
		case uint32:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return def
}

func menuItemID(service string, id int32) string {
	return fmt.Sprintf("menu:%s:%d", service, id)
}

// flattenEntries turns a parsed layout into items. A separator or a disabled
// entry still appears (subtitle indicates state); children are appended after
// their parent as flats so the host's flat scoped list sees them all.
func flattenEntries(entries []menuEntry, depth int, service string) []sdk.Item {
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
			ID:    menuItemID(service, e.id),
			Title: title,
			Glyph: "menu",
			Actions: []sdk.Action{{
				Type: "plugin", Value: "invoke-menu",
			}},
		}
		if !e.enabled {
			item.Subtitle = "disabled"
		}
		items = append(items, item)
		items = append(items, flattenEntries(e.children, depth+1, service)...)
	}
	return items
}
