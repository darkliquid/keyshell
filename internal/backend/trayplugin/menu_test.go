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
