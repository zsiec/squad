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

func TestWalk_ReadsActiveAndDone(t *testing.T) {
	got, err := Walk("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Active) != 3 {
		t.Fatalf("active=%d want 3", len(got.Active))
	}
	if len(got.Done) != 1 {
		t.Fatalf("done=%d want 1", len(got.Done))
	}
	if got.Done[0].ID != "BUG-000" {
		t.Fatalf("done[0]=%s want BUG-000", got.Done[0].ID)
	}
}

func TestWalk_MissingDoneDirIsOk(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Walk(filepath.Join(dir, ".squad"))
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(got.Active) != 0 || len(got.Done) != 0 {
		t.Fatalf("got %+v", got)
	}
}

// QA r6 H #5: Walk used to recurse with filepath.WalkDir while every
// downstream lookup (findItemPath, findItemFile, blockerInDoneDir) used
// flat os.ReadDir. Items in subdirs showed up in `next` but `claim` said
// "not found". Lock the now-flat behaviour in.
func TestWalk_DoesNotRecurseIntoSubdirs(t *testing.T) {
	dir := t.TempDir()
	itemsDir := filepath.Join(dir, ".squad", "items")
	subDir := filepath.Join(itemsDir, "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemsDir, "FEAT-001-top.md"),
		[]byte("---\nid: FEAT-001\ntitle: top\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "FEAT-002-buried.md"),
		[]byte("---\nid: FEAT-002\ntitle: buried\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Walk(filepath.Join(dir, ".squad"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Active) != 1 || got.Active[0].ID != "FEAT-001" {
		t.Fatalf("Active=%v want only FEAT-001", got.Active)
	}
	if len(got.Broken) != 0 {
		t.Fatalf("nested item should be ignored, not flagged broken: %v", got.Broken)
	}
}
