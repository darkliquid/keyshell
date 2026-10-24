package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

func fakePluginScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "plugin", "testdata", "fake_plugin.py"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPluginInstanceBackendInfoNonBlockingInFlight(t *testing.T) {
	hp := &hybridProvider{hosts: make(map[string]*hostEntry)}
	ready := make(chan struct{}) // not closed -> spawn in flight
	hp.hosts["in-flight"] = &hostEntry{ready: ready}

	b := &pluginInstanceBackend{
		hp: hp,
		id: "in-flight",
		info: backend.Info{
			ID:         "in-flight",
			Name:       "Pending Plugin",
			Searchable: false,
		},
	}

	start := time.Now()
	info := b.Info()
	if time.Since(start) > 100*time.Millisecond {
		t.Errorf("Info() took %v, want non-blocking immediate return", time.Since(start))
	}
	if info.ID != "in-flight" {
		t.Errorf("Info().ID = %q, want in-flight", info.ID)
	}
	if info.Searchable != false {
		t.Errorf("Info().Searchable = %v, want false", info.Searchable)
	}
	if info.Name != "Pending Plugin" {
		t.Errorf("Info().Name = %q, want 'Pending Plugin'", info.Name)
	}
}

func TestPluginInstanceBackendInfoPreservesIDAndSearchableWhenAlive(t *testing.T) {
	fakeScript := fakePluginScript(t)
	fBool := false
	cfg := config.Config{
		Instances: []config.Instance{
			{
				ID:         "custom-id",
				Command:    []string{"python3", fakeScript},
				Searchable: &fBool,
			},
		},
	}
	hp := buildProvider(cfg)
	b, err := hp.Get("custom-id")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Trigger spawn via Query
	stream, err := b.Query(context.Background(), backend.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	stream.Close()

	hp.mu.Lock()
	entry := hp.hosts["custom-id"]
	hp.mu.Unlock()
	if entry == nil || entry.host == nil || !entry.host.Alive() {
		t.Fatal("expected host to be alive")
	}
	t.Cleanup(func() { _ = entry.host.Close() })

	// The underlying fake_plugin.py reports ID="fake", Searchable=true.
	// But b.Info() MUST report ID="custom-id" and Searchable=false (config override).
	info := b.Info()
	if info.ID != "custom-id" {
		t.Errorf("info.ID = %q, want custom-id", info.ID)
	}
	if info.Searchable != false {
		t.Errorf("info.Searchable = %v, want false", info.Searchable)
	}
	// Name and Badge should come from the alive host ("Fake" and "FAKE")
	if info.Name != "Fake" {
		t.Errorf("info.Name = %q, want Fake", info.Name)
	}
	if info.Badge != "FAKE" {
		t.Errorf("info.Badge = %q, want FAKE", info.Badge)
	}
}

func TestHybridProviderGetDoesNotSpawnSubprocess(t *testing.T) {
	fakeScript := fakePluginScript(t)
	cfg := config.Config{
		Instances: []config.Instance{
			{
				ID:      "lazy-plugin",
				Command: []string{"python3", fakeScript},
			},
		},
	}
	hp := buildProvider(cfg)

	// Calling Get should NOT spawn
	b, err := hp.Get("lazy-plugin")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	hp.mu.Lock()
	entry := hp.hosts["lazy-plugin"]
	hp.mu.Unlock()
	if entry != nil {
		t.Fatalf("expected hp.hosts[lazy-plugin] to be nil after Get, got %v", entry)
	}

	// Calling Info should NOT spawn
	info := b.Info()
	if info.ID != "lazy-plugin" {
		t.Errorf("info.ID = %q, want lazy-plugin", info.ID)
	}

	hp.mu.Lock()
	entry = hp.hosts["lazy-plugin"]
	hp.mu.Unlock()
	if entry != nil {
		t.Fatalf("expected hp.hosts[lazy-plugin] to be nil after Info, got %v", entry)
	}

	// Calling Query MUST spawn
	stream, err := b.Query(context.Background(), backend.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	stream.Close()

	hp.mu.Lock()
	entry = hp.hosts["lazy-plugin"]
	hp.mu.Unlock()
	if entry == nil {
		t.Fatal("expected hp.hosts[lazy-plugin] to be populated after Query")
	}
	t.Cleanup(func() {
		if entry.host != nil {
			_ = entry.host.Close()
		}
	})
	if entry.host == nil || !entry.host.Alive() {
		t.Error("expected plugin subprocess to be alive after Query")
	}
}

func TestHybridProviderHotStartSpawnsHotPlugin(t *testing.T) {
	fakeScript := fakePluginScript(t)
	cfg := config.Config{
		Instances: []config.Instance{
			{
				ID:      "hot-plugin",
				Command: []string{"python3", fakeScript},
				Mode:    "hot",
			},
			{
				ID:      "lazy-plugin",
				Command: []string{"python3", fakeScript},
				Mode:    "lazy",
			},
		},
	}
	hp := buildProvider(cfg)
	hp.HotStart(cfg)

	// Wait up to 2 seconds for hot-plugin to be spawned
	deadline := time.Now().Add(2 * time.Second)
	var spawned bool
	for time.Now().Before(deadline) {
		hp.mu.Lock()
		entry := hp.hosts["hot-plugin"]
		hp.mu.Unlock()
		if entry != nil {
			select {
			case <-entry.ready:
				if entry.err == nil && entry.host != nil && entry.host.Alive() {
					spawned = true
				}
			default:
			}
		}
		if spawned {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !spawned {
		t.Fatal("expected hot-plugin to be spawned by HotStart")
	}

	hp.mu.Lock()
	entry := hp.hosts["hot-plugin"]
	lazyEntry := hp.hosts["lazy-plugin"]
	hp.mu.Unlock()

	if entry != nil && entry.host != nil {
		t.Cleanup(func() { _ = entry.host.Close() })
	}
	if lazyEntry != nil {
		t.Errorf("lazy-plugin should not be spawned by HotStart, got %v", lazyEntry)
	}
}
