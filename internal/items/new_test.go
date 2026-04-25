package items

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_WritesStubFileAndReturnsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := New(dir, "BUG", "Plug a leak in the pump")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "BUG-001-plug-a-leak-in-the-pump.md") {
		t.Fatalf("path=%s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		"id: BUG-001",
		"title: Plug a leak in the pump",
		"type: bug",
		"## Problem",
		"## Acceptance criteria",
		"## Resolution",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

func TestNew_IncrementsAcrossActiveAndDone(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"items", "done"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "items", "BUG-005-x.md"),
		[]byte("---\nid: BUG-005\ntitle: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "done", "BUG-007-y.md"),
		[]byte("---\nid: BUG-007\ntitle: y\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := New(dir, "BUG", "another")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "BUG-008-another.md") {
		t.Fatalf("path=%s want BUG-008-...", path)
	}
}
