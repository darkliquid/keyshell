package render

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl"
)

//go:embed default.ksl
var defaultScene string

// loadScene reads the configured scene, falling back to the embedded
// default on any error. The default ships with the binary, so a broken
// user scene never keeps the overlay from opening.
func loadScene(cfg config.Config) (*ksl.Scene, error) {
	name, src := "default.ksl", []byte(defaultScene)
	if cfg.UI.Scene != "" {
		b, err := os.ReadFile(cfg.UI.Scene)
		if err == nil {
			name, src = cfg.UI.Scene, b
		} else {
			fmt.Fprintf(os.Stderr, "ksl: read %s: %v (using default scene)\n", cfg.UI.Scene, err)
		}
	}
	s, err := parseScene(name, src, loaderFor(name, cfg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksl: %v (using default scene)\n", err)
		s, err = parseScene("default.ksl", []byte(defaultScene), nil)
	}
	if err != nil {
		return nil, fmt.Errorf("embedded default scene: %w", err)
	}
	return s, nil
}

func parseScene(name string, src []byte, load ksl.Loader) (*ksl.Scene, error) {
	s, err := ksl.ParseWithLoader(name, src, load)
	if err != nil {
		return nil, err
	}
	if err := ksl.ValidateWith(s, "Card"); err != nil {
		return nil, err
	}
	return s, nil
}

// loaderFor resolves includes relative to the scene file's directory.
func loaderFor(name string, cfg config.Config) ksl.Loader {
	if cfg.UI.Scene == "" || name == "default.ksl" {
		return nil
	}
	dir := filepath.Dir(name)
	return func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, rel))
	}
}
