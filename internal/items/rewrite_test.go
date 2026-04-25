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
