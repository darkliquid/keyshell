package ksl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loaderFromDir(dir string) Loader {
	return func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, name))
	}
}

func TestIncludeSplicesNodes(t *testing.T) {
	s, err := ParseWithLoader("main.ksl",
		[]byte("Window { include \"part.ksl\" }"),
		loaderFromDir(filepath.Join("testdata", "include")))
	if err != nil {
		t.Fatalf("ParseWithLoader() = %v, want nil", err)
	}
	root := s.Nodes()[0]
	if len(root.Children) != 2 || root.Children[0].Name != "Box" || root.Children[1].Name != "Text" {
		t.Errorf("children = %+v, want spliced Box and Text", root.Children)
	}
}

func TestIncludeTopLevel(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "include", "main.ksl"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseWithLoader("main.ksl", src, loaderFromDir(filepath.Join("testdata", "include")))
	if err != nil {
		t.Fatalf("ParseWithLoader() = %v, want nil", err)
	}
	root := s.Nodes()[0]
	if root.Name != "Window" || len(root.Children) != 2 {
		t.Errorf("root = %+v with %d children, want Window with 2", root.Name, len(root.Children))
	}
}

func TestIncludeCycle(t *testing.T) {
	load := func(name string) ([]byte, error) {
		return []byte("Window { include \"self.ksl\" }"), nil
	}
	_, err := ParseWithLoader("self.ksl", []byte("Window { include \"self.ksl\" }"), load)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("ParseWithLoader() err = %v, want cycle error", err)
	}
}

func TestIncludeNeedsLoader(t *testing.T) {
	_, err := Parse("t.ksl", []byte(`include "part.ksl"`))
	if err == nil || !strings.Contains(err.Error(), "loader") {
		t.Errorf("Parse() err = %v, want loader error", err)
	}
}

func TestIncludeOnlyNodesInsideNodes(t *testing.T) {
	load := func(name string) ([]byte, error) { return []byte("theme { ink: #FFFFFF }"), nil }
	_, err := ParseWithLoader("t.ksl", []byte(`Window { Box { include "x.ksl" } }`), load)
	if err == nil || !strings.Contains(err.Error(), "only contain nodes") {
		t.Errorf("ParseWithLoader() err = %v, want nodes-only error", err)
	}
}
