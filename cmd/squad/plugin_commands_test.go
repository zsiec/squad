package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkCommand_CallsSquadGo(t *testing.T) {
	wd, _ := os.Getwd()
	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "plugin", "commands", "work.md")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not locate plugin/commands/work.md")
		}
		root = parent
	}
	body, err := os.ReadFile(filepath.Join(root, "plugin", "commands", "work.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "squad go") {
		t.Fatal("plugin/commands/work.md should invoke `squad go`")
	}
	for _, oldRecipe := range []string{
		"openssl rand",
		"squad register --as",
		"squad next",
	} {
		if strings.Contains(s, oldRecipe) {
			t.Errorf("plugin/commands/work.md still contains the pre-r2 recipe: %q", oldRecipe)
		}
	}
}
