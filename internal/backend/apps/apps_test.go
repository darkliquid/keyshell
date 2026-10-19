package apps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/go-freedesktop/desktopentry"
)

// TestNewListsApplications verifies the backend surfaces installed desktop
// entries as queryable items via the host Backend seam.
func TestNewListsApplications(t *testing.T) {
	data := t.TempDir()
	appsDir := filepath.Join(data, "applications")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeDesktopEntry(t, appsDir, "ghostty", "Ghostty", "Terminal", "ghostty")
	writeDesktopEntry(t, appsDir, "firefox", "Firefox", "Web Browser", "firefox")
	t.Setenv("XDG_DATA_HOME", data)

	scan := func() []*desktopentry.Entry {
		appsDir := filepath.Join(data, "applications")
		return desktopentry.ScanDirs([]string{appsDir})
	}
	b, err := newWithScan(scan)
	if err != nil {
		t.Fatalf("newWithScan = err %v, want nil", err)
	}
	stream, err := b.Query(context.Background(), backend.Query{Text: ""})
	if err != nil {
		t.Fatalf("Query = err %v, want nil", err)
	}
	defer stream.Close()

	var titles []string
	for {
		it, more, err := stream.Next()
		if err != nil {
			t.Fatalf("Next = err %v, want nil", err)
		}
		if !more {
			break
		}
		titles = append(titles, it.Title)
	}
	if len(titles) != 2 {
		t.Errorf("items = %v, want 2 (ghostty, firefox)", titles)
	}
}

// TestQueryFiltersByName proves filtering happens against the entry name.
func TestQueryFiltersByName(t *testing.T) {
	data := t.TempDir()
	appsDir := filepath.Join(data, "applications")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDesktopEntry(t, appsDir, "ghostty", "Ghostty", "Terminal", "ghostty")
	writeDesktopEntry(t, appsDir, "firefox", "Firefox", "Web Browser", "firefox")
	t.Setenv("XDG_DATA_HOME", data)

	scan := func() []*desktopentry.Entry {
		appsDir := filepath.Join(data, "applications")
		return desktopentry.ScanDirs([]string{appsDir})
	}
	b, _ := newWithScan(scan)
	stream, _ := b.Query(context.Background(), backend.Query{Text: "gho"})
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
		t.Errorf("filtered items = %d, want 1 (only Ghostty)", got)
	}
}

func writeDesktopEntry(t *testing.T, dir, id, name, generic, exec string) {
	t.Helper()
	content := "[Desktop Entry]\nType=Application\nName=" + name + "\nGenericName=" + generic + "\nExec=" + exec + "\nIcon=app-icon\n"
	path := filepath.Join(dir, id+".desktop")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
