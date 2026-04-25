package items

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// QA r6 H #2/#4: Parse tolerates UTF-8 BOM + CRLF, but rewriteFrontmatter
// used to assume LF-only and reject. Result: squad done committed the DB
// release but bailed on the file rewrite, leaving inconsistent state.
func TestRewriteStatus_AcceptsCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FEAT-001-x.md")
	body := "---\r\nid: FEAT-001\r\ntitle: x\r\nstatus: open\r\n---\r\n\r\n## Problem\r\nstuff\r\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RewriteStatus(path, "done", time.Now()); err != nil {
		t.Fatalf("RewriteStatus on CRLF file failed: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "status: done") {
		t.Fatalf("status not rewritten in CRLF file:\n%s", got)
	}
}

// QA r6-H #1: 'path + ".squad.tmp"' overflowed the 255-byte FS limit when
// the basename approached ~245 bytes. atomicWrite uses CreateTemp's
// random-suffix pattern so the staging name is bounded regardless.
func TestRewriteStatus_HandlesLongFilename(t *testing.T) {
	dir := t.TempDir()
	// 240-byte basename — well past the old .squad.tmp overflow point but
	// still inside the 255-byte limit on its own.
	longSlug := strings.Repeat("a", 230)
	path := filepath.Join(dir, "FEAT-001-"+longSlug+".md")
	if err := os.WriteFile(path,
		[]byte("---\nid: FEAT-001\ntitle: long\nstatus: open\n---\n\n## Body\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if err := RewriteStatus(path, "done", time.Now()); err != nil {
		t.Fatalf("RewriteStatus on long filename: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "status: done") {
		t.Fatalf("status not rewritten: %s", got)
	}
}

func TestRewriteStatus_AcceptsBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FEAT-001-x.md")
	body := "\xef\xbb\xbf---\nid: FEAT-001\ntitle: x\nstatus: open\n---\n\n## Problem\nstuff\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RewriteStatus(path, "done", time.Now()); err != nil {
		t.Fatalf("RewriteStatus on BOM file failed: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "status: done") {
		t.Fatalf("status not rewritten in BOM file:\n%s", got)
	}
}
