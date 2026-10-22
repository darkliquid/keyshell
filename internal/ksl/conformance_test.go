package ksl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpecExamplesParseAndValidate(t *testing.T) {
	for _, name := range []string{"minimal.ksl", "reference.ksl", "dropdown.ksl"} {
		src, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		s, perr := Parse(name, src)
		if perr != nil {
			t.Fatalf("%s: %v", name, perr)
		}
		if verr := Validate(s); verr != nil {
			t.Fatalf("%s: %v", name, verr)
		}
	}
}

func TestReferenceSceneStructure(t *testing.T) {
	src, _ := os.ReadFile(filepath.Join("testdata", "reference.ksl"))
	s, err := Parse("reference.ksl", src)
	if err != nil {
		t.Fatal(err)
	}
	root := s.Nodes()[0]
	band := root.Children[0]
	if band.Name != "Group" || band.ID != "band" {
		t.Fatalf("first child = %+v, want Group#band", band)
	}
	if band.Props[1].Val.String() != "$reveal.y" {
		t.Errorf("band y = %v", band.Props[1].Val)
	}
	var list *Node
	for _, c := range band.Children {
		if c.Name == "Stack" {
			list = c.Children[0].Children[1]
		}
	}
	if list == nil || list.Name != "List" || len(list.Children) != 1 || list.Children[0].Name != "template" {
		t.Errorf("band must contain a List with a template block")
	}
}
