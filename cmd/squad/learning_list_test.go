package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkLearning is a shared helper used by tasks 5–7. Lives in this file but
// any test in the package can call it.
func mkLearning(t *testing.T, repoRoot, rel, kind, slug, area, state string) {
	t.Helper()
	p := filepath.Join(repoRoot, ".squad", "learnings", rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: " + kind + "-" + slug + "\nkind: " + kind + "\nslug: " + slug +
		"\ntitle: t\narea: " + area + "\ncreated: 2026-04-25\ncreated_by: agent-x\nsession: s\nstate: " + state + "\n---\n\n"
	switch kind {
	case "gotcha":
		body += "## Looks like\n\nx\n## Is\n\ny\n"
	case "pattern":
		body += "## When\n\nx\n## Do\n\ny\n## Why\n\nz\n"
	case "dead-end":
		body += "## Tried\n\nx\n## Doesn't work because\n\ny\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLearningList_FiltersByAreaAndState(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "gotchas/proposed/a.md", "gotcha", "a", "store", "proposed")
	mkLearning(t, repo, "gotchas/approved/b.md", "gotcha", "b", "store", "approved")
	mkLearning(t, repo, "patterns/approved/c.md", "pattern", "c", "chat", "approved")

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"learning", "list", "--area", "store"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "a.md") || !strings.Contains(s, "b.md") || strings.Contains(s, "c.md") {
		t.Errorf("--area=store filter wrong: %q", s)
	}

	out.Reset()
	root = newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"learning", "list", "--state", "approved"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s = out.String()
	if !strings.Contains(s, "b.md") || !strings.Contains(s, "c.md") || strings.Contains(s, "a.md") {
		t.Errorf("--state=approved filter wrong: %q", s)
	}
}
