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
	rel := filepath.Join("plugin", "skills", "squad-work", "SKILL.md")
	for {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("could not locate %s", rel)
		}
		root = parent
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "squad go") {
		t.Fatalf("%s should invoke `squad go`", rel)
	}
	for _, oldRecipe := range []string{
		"openssl rand",
		"squad register --as",
		"squad next",
	} {
		if strings.Contains(s, oldRecipe) {
			t.Errorf("%s still contains the pre-r2 recipe: %q", rel, oldRecipe)
		}
	}
}

func TestSquadLoopSkill_DocumentsPaths(t *testing.T) {
	wd, _ := os.Getwd()
	root := wd
	rel := filepath.Join("plugin", "skills", "squad-loop", "SKILL.md")
	for {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("could not locate %s", rel)
		}
		root = parent
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "paths:") {
		t.Fatalf("%s should document the `paths:` skill field", rel)
	}
	if !strings.Contains(s, "agents-deep.md") {
		t.Fatalf("%s should reference docs/agents-deep.md as the depth-tier read", rel)
	}
}
