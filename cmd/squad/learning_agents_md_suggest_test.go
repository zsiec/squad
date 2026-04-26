package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const agentsMdDiff = `--- a/AGENTS.md
+++ b/AGENTS.md
@@ -1,3 +1,4 @@
 # squad — Agent Operating Manual
+## §0.5 — Read learnings before editing

 This is the loop.
`

func TestAgentsMdSuggest_WritesProposalNotAgentsMd(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	diffPath := filepath.Join(repo, "patch.diff")
	if err := os.WriteFile(diffPath, []byte(agentsMdDiff), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"learning", "agents-md-suggest",
		"--diff", diffPath,
		"--rationale", "agents skip §0 fast-tier read; pin a forward reference"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	original, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err == nil && strings.Contains(string(original), "§0.5") {
		t.Errorf("AGENTS.md was modified directly; that is forbidden")
	}

	dir := filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected proposal under %s, got %v / %v", dir, entries, err)
	}
	body, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read proposal: %v", err)
	}
	for _, want := range []string{
		"§0.5 — Read learnings before editing",
		"agents skip §0 fast-tier read",
		"--- a/AGENTS.md", "+++ b/AGENTS.md",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("proposal missing %q:\n%s", want, body)
		}
	}
}

func TestAgentsMdSuggest_RequiresRationale(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "agents-md-suggest", "--diff", "/dev/null"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error()+stderr.String(), "rationale") {
		t.Errorf("want rationale error, got %v / %s", err, stderr.String())
	}
}
