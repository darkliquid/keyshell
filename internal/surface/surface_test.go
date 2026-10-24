package surface

import "testing"

func TestNewHost(t *testing.T) {
	cfg := SurfaceConfig{
		Width:       800,
		Height:      600,
		Title:       "test-keyshell",
		Namespace:   "test",
		Transparent: true,
	}
	host := NewHost()
	if err := host.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if host.App() == nil {
		t.Fatal("expected non-nil App")
	}
	if host.EventSource() == nil {
		t.Fatal("expected non-nil EventSource")
	}

	// Verify calls do not panic
	_ = host.SetVisible(true)
	_ = host.SetVisible(false)
	_ = host.SetKeyboardFocus(true)
	_ = host.SetKeyboardFocus(false)
	_ = host.SetInputRegion(true)
	_ = host.SetInputRegion(false)
	host.RequestRedraw()
	host.Quit()
}

func TestUninitializedHost(t *testing.T) {
	host := NewHost()

	if host.App() != nil {
		t.Fatal("expected nil App for uninitialized host")
	}
	if host.EventSource() != nil {
		t.Fatal("expected nil EventSource for uninitialized host")
	}

	if err := host.SetVisible(false); err != nil {
		t.Errorf("SetVisible(false) failed: %v", err)
	}
	if err := host.SetKeyboardFocus(false); err != nil {
		t.Errorf("SetKeyboardFocus(false) failed: %v", err)
	}
	if err := host.SetInputRegion(false); err != nil {
		t.Errorf("SetInputRegion(false) failed: %v", err)
	}

	host.RequestRedraw()
	host.Quit()

	if err := host.Run(); err == nil {
		t.Error("expected error from Run() on uninitialized host, got nil")
	}
}
