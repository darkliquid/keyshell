package trayplugin

import (
	"context"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// fakeWatcher serves a fixed item list without D-Bus.
type fakeWatcher struct {
	items []string
}

func (f *fakeWatcher) Items() []string { return f.items }

// newFakeBackend builds a tray backend over a nil connection, so titles fall
// back to service names (the query path is what we assert).
func newFakeBackend(items ...string) *Backend {
	return &Backend{watcher: &fakeWatcher{items: items}}
}

func TestQueryListsApplets(t *testing.T) {
	b := newFakeBackend("org.kde.StatusNotifierItem-1-1", "org.kde.StatusNotifierItem-2-1")
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()

	ids := []string{}
	for {
		it, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		ids = append(ids, it.ID)
	}
	if len(ids) != 2 {
		t.Errorf("ids = %v, want 2", ids)
	}
	if ids[0] != "org.kde.StatusNotifierItem-1-1" {
		t.Errorf("first id = %q", ids[0])
	}
}

func TestQueryFilters(t *testing.T) {
	b := newFakeBackend("org.kde.StatusNotifierItem-audio", "org.kde.StatusNotifierItem-other")
	stream, _ := b.Query(context.Background(), sdk.Query{Text: "audio"})
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

func TestQueryDeduplicatesItems(t *testing.T) {
	// Two service entries with the same derived title "Nordvpn"
	b := newFakeBackend("org.kde.StatusNotifierItem-nordvpn-1", "org.ayatana.NotificationItem-nordvpn")
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query err = %v", err)
	}
	defer stream.Close()
	var items []sdk.Item
	for {
		it, more, err := stream.Next()
		if err != nil || !more {
			break
		}
		items = append(items, it)
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1 (deduplicated)", len(items))
	}
	if items[0].Title != "Nordvpn" {
		t.Errorf("item title = %q, want Nordvpn", items[0].Title)
	}
}

func TestContainsFold(t *testing.T) {
	if !containsFold("My Applet", "APPLET") {
		t.Error("containsFold should match case-insensitively")
	}
	if containsFold("My Applet", "nope") {
		t.Error("containsFold matched a non-substring")
	}
}

func TestParseMenuItemID(t *testing.T) {
	svc, id, ok := parseMenuItemID("menu:org.kde.StatusNotifierItem-1-1:42")
	if !ok || svc != "org.kde.StatusNotifierItem-1-1" || id != 42 {
		t.Errorf("parse = (%q,%d,%v)", svc, id, ok)
	}
	if _, _, ok := parseMenuItemID("bogus"); ok {
		t.Error("parse(bogus) should fail")
	}
	if _, _, ok := parseMenuItemID("menu:svc:notanumber"); ok {
		t.Error("parse(bad id) should fail")
	}

	// Service with unique bus name and path
	svc, id, ok = parseMenuItemID("menu::1.42/StatusNotifierItem:12")
	if !ok || svc != ":1.42/StatusNotifierItem" || id != 12 {
		t.Errorf("parse = (%q,%d,%v), want (:1.42/StatusNotifierItem, 12, true)", svc, id, ok)
	}
}

func TestSplitItem(t *testing.T) {
	svc, path := splitItem(":1.42/org/ayatana/NotificationItem/steam")
	if svc != ":1.42" || path != "/org/ayatana/NotificationItem/steam" {
		t.Errorf("splitItem = (%q,%q)", svc, path)
	}
	svc, path = splitItem("org.kde.StatusNotifierItem-1-1")
	if svc != "org.kde.StatusNotifierItem-1-1" || path != itemPath {
		t.Errorf("splitItem = (%q,%q)", svc, path)
	}
}

func TestExtractPixmapsAndConversion(t *testing.T) {
	// Simulate an ARGB 2x2 image
	// Pixels:
	// (0,0): A=255, R=10, G=20, B=30
	// (1,0): A=200, R=40, G=50, B=60
	// (0,1): A=150, R=70, G=80, B=90
	// (1,1): A=100, R=100, G=110, B=120
	data := []byte{
		255, 10, 20, 30,
		200, 40, 50, 60,
		150, 70, 80, 90,
		100, 100, 110, 120,
	}

	type sniPixmap struct {
		Width  int32
		Height int32
		Data   []byte
	}
	input := []sniPixmap{
		{Width: 2, Height: 2, Data: data},
	}

	pms := extractPixmaps(input)
	if len(pms) != 1 {
		t.Fatalf("extractPixmaps = %d pixmaps, want 1", len(pms))
	}
	if pms[0].width != 2 || pms[0].height != 2 {
		t.Errorf("pixmap dimensions = %dx%d, want 2x2", pms[0].width, pms[0].height)
	}

	img := pixmapToImage(pms[0])
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("img bounds = %v, want 2x2", img.Bounds())
	}
	// Verify ARGB -> RGBA conversion
	// Pixel 0:
	px0 := img.RGBAAt(0, 0)
	if px0.R != 10 || px0.G != 20 || px0.B != 30 || px0.A != 255 {
		t.Errorf("px(0,0) = %+v, want R=10, G=20, B=30, A=255", px0)
	}
	// Pixel 3:
	px3 := img.RGBAAt(1, 1)
	if px3.R != 100 || px3.G != 110 || px3.B != 120 || px3.A != 100 {
		t.Errorf("px(1,1) = %+v, want R=100, G=110, B=120, A=100", px3)
	}

	// Also test savePixmapImage
	path := savePixmapImage("org.test.applet", img)
	if path == "" {
		t.Fatal("savePixmapImage returned empty path")
	}
	if !fileExists(path) {
		t.Fatalf("savePixmapImage file %q does not exist", path)
	}
}

func TestExtractPixmapsFromInterfaces(t *testing.T) {
	data := make([]byte, 16*16*4)
	input := []interface{}{
		[]interface{}{int32(16), int32(16), data},
	}
	pms := extractPixmaps(input)
	if len(pms) != 1 || pms[0].width != 16 {
		t.Fatalf("extractPixmaps(interfaces) = %v, want 1 pixmap 16x16", pms)
	}
}

func TestFallbackTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"org.kde.StatusNotifierItem-discord-1", "Discord"},
		{"org.ayatana.NotificationItem-steam", "Steam"},
		{"org.kde.StatusNotifierItem-1234-1", "1234"},
		{":1.42/StatusNotifierItem", ":1.42"},
	}
	for _, tc := range tests {
		if got := fallbackTitle(tc.input); got != tc.want {
			t.Errorf("fallbackTitle(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsSNIService(t *testing.T) {
	if !isSNIService("org.kde.StatusNotifierItem-123-1") {
		t.Error("expected true for org.kde.StatusNotifierItem")
	}
	if !isSNIService("org.ayatana.NotificationItem-steam") {
		t.Error("expected true for org.ayatana.NotificationItem")
	}
	if isSNIService("org.freedesktop.DBus") {
		t.Error("expected false for org.freedesktop.DBus")
	}
}

func TestWatcherProperties(t *testing.T) {
	w := &Watcher{items: []string{"svc1", "svc2"}}
	props := &watcherProperties{w: w}

	v, err := props.Get("org.kde.StatusNotifierWatcher", "RegisteredStatusNotifierItems")
	if err != nil {
		t.Fatalf("props.Get = err %v", err)
	}
	list := v.Value().([]string)
	if len(list) != 2 || list[0] != "svc1" {
		t.Errorf("registered items = %v, want [svc1, svc2]", list)
	}

	all, err := props.GetAll("org.kde.StatusNotifierWatcher")
	if err != nil {
		t.Fatalf("props.GetAll = err %v", err)
	}
	if len(all) != 3 {
		t.Errorf("props.GetAll len = %d, want 3", len(all))
	}

	if err := props.Set("org.kde.StatusNotifierWatcher", "ProtocolVersion", v); err == nil {
		t.Error("props.Set should return error for read-only property")
	}
}
