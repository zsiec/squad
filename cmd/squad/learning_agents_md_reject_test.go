package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const seedAgentsMdProposal = `---
id: 20260425T120000Z-test
kind: agents-md-suggestion
created: 2026-04-25
created_by: agent-x
state: proposed
---

## Rationale

something

## Diff

` + "```diff\n" + `--- a/AGENTS.md
+++ b/AGENTS.md
@@ -1,1 +1,2 @@
 # agents
+extra
` + "```\n"

func TestAgentsMdReject_ArchivesNotDeletes(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	id := "20260425T120000Z-test"
	dir := filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(seedAgentsMdProposal), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "agents-md-reject", id, "--reason", "out of scope"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, id+".md")); !os.IsNotExist(err) {
		t.Errorf("expected proposal removed from proposed/, got %v", err)
	}
	archived := filepath.Join(repo, ".squad", "learnings", "agents-md", "rejected", id+".md")
	body, err := os.ReadFile(archived)
	if err != nil {
		t.Fatalf("expected archive at %s, got %v", archived, err)
	}
	s := string(body)
	for _, want := range []string{"state: rejected", "out of scope"} {
		if !strings.Contains(s, want) {
			t.Errorf("archive missing %q:\n%s", want, s)
		}
	}
}

func TestAgentsMdReject_RequiresReason(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "agents-md-reject", "anything"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error()+stderr.String(), "reason") {
		t.Errorf("want reason error, got %v / %s", err, stderr.String())
	}
}
