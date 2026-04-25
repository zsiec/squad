package items

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_HappyPath(t *testing.T) {
	it, err := Parse(filepath.Join("testdata", "items", "BUG-001-example.md"))
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != "BUG-001" {
		t.Fatalf("id=%q want BUG-001", it.ID)
	}
	if it.Priority != "P1" {
		t.Fatalf("priority=%q", it.Priority)
	}
	if it.Type != "bug" {
		t.Fatalf("type=%q", it.Type)
	}
	if it.ACTotal != 2 || it.ACChecked != 1 {
		t.Fatalf("ac=%d/%d want 1/2", it.ACChecked, it.ACTotal)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParse_NoFrontmatterIsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "broken.md")
	writeFile(t, p, "no frontmatter here\n")
	if _, err := Parse(p); err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestParse_MalformedFrontmatterIsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "broken.md")
	writeFile(t, p, "---\nid: [oops\n---\n\nbody\n")
	if _, err := Parse(p); err == nil {
		t.Fatal("expected error for malformed yaml frontmatter")
	}
}
