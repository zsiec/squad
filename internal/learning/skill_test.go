package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeApproved(t *testing.T, root, kindDir, slug, area string, paths []string) {
	t.Helper()
	dir := filepath.Join(root, ".squad", "learnings", kindDir, "approved")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	kind := strings.TrimSuffix(kindDir, "s")
	body := writeYAMLFrontmatter(kind, slug, area, "approved", paths)
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeProposed(t *testing.T, root, kindDir, slug, area string, paths []string) {
	t.Helper()
	dir := filepath.Join(root, ".squad", "learnings", kindDir, "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	kind := strings.TrimSuffix(kindDir, "s")
	body := writeYAMLFrontmatter(kind, slug, area, "proposed", paths)
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeYAMLFrontmatter(kind, slug, area, state string, paths []string) string {
	var sb strings.Builder
	sb.WriteString("---\nid: " + kind + "-" + slug + "\nkind: " + kind +
		"\nslug: " + slug + "\ntitle: t\narea: " + area + "\npaths:\n")
	for _, p := range paths {
		sb.WriteString("  - " + p + "\n")
	}
	sb.WriteString("created: 2026-04-25\ncreated_by: agent-x\nsession: s\nstate: " + state + "\n---\n\n")
	switch kind {
	case "gotcha":
		sb.WriteString("## Looks like\n\nx\n## Is\n\ny\n")
	case "pattern":
		sb.WriteString("## When\n\nx\n## Do\n\ny\n## Why\n\nz\n")
	case "dead-end":
		sb.WriteString("## Tried\n\nx\n## Doesn't work because\n\ny\n")
	}
	return sb.String()
}

func TestRegenerateSkill_UnionsPathsAndIncludesEveryApproved(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeApproved(t, root, "gotchas", "sqlite-busy", "store", []string{"internal/store/**", "internal/claims/**"})
	writeApproved(t, root, "patterns", "boot-context", "boot", []string{"cmd/squad/**"})
	writeProposed(t, root, "patterns", "not-yet", "ux", []string{"cmd/**"})

	if err := RegenerateSkill(root); err != nil {
		t.Fatalf("RegenerateSkill: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "squad-learnings.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"name: squad-learnings",
		"paths:", "internal/store/**", "internal/claims/**", "cmd/squad/**",
		"sqlite-busy", "boot-context",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("skill missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "not-yet") {
		t.Errorf("proposed learning leaked into auto-load skill")
	}
}

func TestRegenerateSkill_NoApproved_RemovesStaleSkill(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, ".claude", "skills", "squad-learnings.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RegenerateSkill(root); err != nil {
		t.Fatalf("RegenerateSkill: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("expected stale skill removed, got %v", err)
	}
}
