package specs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse_FullFrontmatter(t *testing.T) {
	s, err := Parse("testdata/specs/auth-rework.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Name != "auth-rework" || s.Title != "Auth rework" || s.Motivation == "" {
		t.Errorf("name=%q title=%q motivation empty=%v",
			s.Name, s.Title, s.Motivation == "")
	}
	want := []string{"login redirects to /home", "sessions expire after 30 days"}
	if !reflect.DeepEqual(s.Acceptance, want) {
		t.Errorf("Acceptance=%v want %v", s.Acceptance, want)
	}
}

func TestParse_RejectsPRDFraming(t *testing.T) {
	if _, err := Parse("testdata/specs/with-prd-key.md"); err == nil {
		t.Fatal("expected error when frontmatter contains 'prd:' key")
	}
}

// Walk silently skips malformed specs (by design — doctor surfaces them),
// which means a YAML typo in any spec under .squad/specs/ disappears from
// the generated AGENTS.md / docs/specs.md without warning. This test runs
// against the real repo specs and fails the suite if any spec fails to
// parse — so a future colon-space or unquoted scalar gets caught at CI
// time rather than at the next operator scan.
func TestParse_AllProjectSpecsParseCleanly(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	specsDir := filepath.Join(repoRoot, ".squad", "specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("read specs dir: %v", err)
	}
	var checked int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(specsDir, e.Name())
		if _, err := Parse(path); err != nil {
			t.Errorf("Parse(%s) failed: %v", e.Name(), err)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("no .md specs found under %s", specsDir)
	}
}

func TestWalk_FindsAllSpecs(t *testing.T) {
	got, err := Walk("testdata")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range got {
		if s.Name == "auth-rework" {
			found = true
		}
	}
	if !found {
		t.Errorf("auth-rework not in %v", got)
	}
}
