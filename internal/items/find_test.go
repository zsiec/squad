package items

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindByID(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "done"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "items", "BUG-001-foo.md"), []byte("---\nid: BUG-001\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "done", "FEAT-002-bar.md"), []byte("---\nid: FEAT-002\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, inDone, err := FindByID(dir, "BUG-001")
	if err != nil || inDone || filepath.Base(p) != "BUG-001-foo.md" {
		t.Fatalf("BUG-001 lookup: p=%q inDone=%v err=%v", p, inDone, err)
	}

	p, inDone, err = FindByID(dir, "FEAT-002")
	if err != nil || !inDone || filepath.Base(p) != "FEAT-002-bar.md" {
		t.Fatalf("FEAT-002 lookup: p=%q inDone=%v err=%v", p, inDone, err)
	}

	if _, _, err := FindByID(dir, "GHOST-999"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}
