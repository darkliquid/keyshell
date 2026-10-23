package trayplugin

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestParseLayout handles a two-level menu tree.
func TestParseLayout(t *testing.T) {
	child := []interface{}{int32(3), map[string]dbus.Variant{
		"label": dbus.MakeVariant("Play"),
	}, []interface{}{}}
	separator := []interface{}{int32(4), map[string]dbus.Variant{
		"type": dbus.MakeVariant("separator"),
	}, []interface{}{}}
	root := []interface{}{int32(1), map[string]dbus.Variant{
		"label": dbus.MakeVariant("Controls"),
	}, []interface{}{child, separator}}

	parsed := parseLayout([]interface{}{root})
	if len(parsed) != 1 {
		t.Fatalf("parsed = %d entries, want 1 root", len(parsed))
	}
	if parsed[0].label != "Controls" {
		t.Errorf("root label = %q", parsed[0].label)
	}
	if len(parsed[0].children) != 2 {
		t.Fatalf("root children = %d, want 2", len(parsed[0].children))
	}
	if parsed[0].children[0].label != "Play" || parsed[0].children[0].kind != "" {
		t.Errorf("child = %+v", parsed[0].children[0])
	}
	if parsed[0].children[1].kind != "separator" {
		t.Errorf("separator kind = %q", parsed[0].children[1].kind)
	}
}

// TestFlattenEntries produces flat items and skips separators.
func TestFlattenEntries(t *testing.T) {
	entries := []menuEntry{
		{id: 1, label: "Root", enabled: true, children: []menuEntry{
			{id: 2, label: "Play", enabled: true},
			{id: 3, label: "Pause", enabled: false},
		}},
		{id: 4, label: "", kind: "separator", enabled: true},
	}
	items := flattenEntries(entries, 0, "svc")
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3 (root + 2 children, separator skipped)", len(items))
	}
	if items[0].Title != "Root" || items[1].Title != "Play" {
		t.Errorf("titles = %q / %q", items[0].Title, items[1].Title)
	}
	if items[2].Subtitle != "disabled" {
		t.Errorf("disabled subtitle = %q", items[2].Subtitle)
	}
}

// TestNestedSubmenuNavigation proves entriesToItems correctly assigns HasChildren
// and findNode finds submenus by target id.
func TestNestedSubmenuNavigation(t *testing.T) {
	entries := []menuEntry{
		{id: 1, label: "_Controls", enabled: true, children: []menuEntry{
			{id: 2, label: "Play", enabled: true},
			{id: 3, label: "&Volume", enabled: true, children: []menuEntry{
				{id: 4, label: "Mute", enabled: true},
			}},
		}},
		{id: 5, label: "Quit", enabled: true},
	}

	// Level 1: top-level entries (accelerators cleaned)
	items := entriesToItems(entries, "applet1")
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2 (Controls, Quit)", len(items))
	}
	if items[0].Title != "Controls" || !items[0].HasChildren {
		t.Errorf("items[0] = %+v, want Controls with HasChildren=true", items[0])
	}
	if items[1].Title != "Quit" || items[1].HasChildren || len(items[1].Actions) != 1 {
		t.Errorf("items[1] = %+v, want Quit with HasChildren=false and invoke action", items[1])
	}

	// Level 2: descend into "Controls" (id 1)
	node, found := findNode(entries, 1)
	if !found {
		t.Fatal("node 1 not found")
	}
	subItems := entriesToItems(node.children, "applet1")
	if len(subItems) != 2 {
		t.Fatalf("subItems = %d, want 2 (Play, Volume)", len(subItems))
	}
	if subItems[0].Title != "Play" || subItems[0].HasChildren {
		t.Errorf("Play hasChildren = %v, want false", subItems[0].HasChildren)
	}
	if subItems[1].Title != "Volume" || !subItems[1].HasChildren {
		t.Errorf("Volume hasChildren = %v, want true", subItems[1].HasChildren)
	}

	// Level 3: descend into "Volume" (id 3)
	node3, found := findNode(entries, 3)
	if !found {
		t.Fatal("node 3 not found")
	}
	subSubItems := entriesToItems(node3.children, "applet1")
	if len(subSubItems) != 1 || subSubItems[0].Title != "Mute" || subSubItems[0].HasChildren {
		t.Errorf("subSubItems = %+v, want 1 item (Mute)", subSubItems)
	}
}

func TestParseLayoutWithDBusVariants(t *testing.T) {
	// Simulate wire format from godbus:
	// Root has id=0, empty props, and children as []dbus.Variant
	child1 := []interface{}{
		int32(1),
		dbus.MakeVariant(map[string]interface{}{
			"label": dbus.MakeVariant("Disconnect"),
			"type":  "standard",
		}),
		[]dbus.Variant{},
	}
	child2 := []interface{}{
		int32(2),
		map[string]interface{}{
			"label":        "Auto-connect",
			"toggle-type":  "checkmark",
			"toggle-state": 1,
		},
		[]interface{}{},
	}
	root := []interface{}{
		int32(0),
		map[string]interface{}{},
		[]dbus.Variant{
			dbus.MakeVariant(child1),
			dbus.MakeVariant(child2),
		},
	}

	entries := parseLayout(dbus.MakeVariant(root))
	if len(entries) != 2 {
		t.Fatalf("entries count = %d, want 2 (root container unwrapped)", len(entries))
	}
	if entries[0].label != "Disconnect" || entries[0].id != 1 {
		t.Errorf("entry 0 = %+v, want Disconnect id=1", entries[0])
	}
	if entries[1].label != "Auto-connect" || entries[1].id != 2 || entries[1].toggleState != 1 {
		t.Errorf("entry 1 = %+v, want Auto-connect toggleState=1", entries[1])
	}

	items := entriesToItems(entries, "applet")
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}
	if items[0].Title != "Disconnect" || items[0].HasChildren {
		t.Errorf("items[0] = %+v, want Disconnect without children", items[0])
	}
	if items[1].Title != "Auto-connect" || items[1].Status != "ON" || items[1].StatusTone != "positive" {
		t.Errorf("items[1] = %+v, want Auto-connect with ON badge", items[1])
	}
}
