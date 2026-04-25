package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR3_LegacyItemsStillWork(t *testing.T) {
	dir := t.TempDir()
	w := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w(filepath.Join(dir, ".git", "HEAD"), "")
	w(filepath.Join(dir, ".squad", "config.yaml"), "")
	w(filepath.Join(dir, ".squad", "items", "FEAT-001.md"),
		"---\nid: FEAT-001\ntitle: Legacy\ntype: feature\npriority: P1\n"+
			"area: core\nstatus: open\nestimate: 1h\nrisk: low\n"+
			"created: 2026-04-25\nupdated: 2026-04-25\n"+
			"references: []\nrelates-to: []\nblocked-by: []\n"+
			"---\n\n## Problem\nx\n\n## Acceptance criteria\n- [ ] thing\n")

	t.Setenv("SQUAD_HOME", filepath.Join(dir, "squad-home"))
	t.Chdir(dir)

	var out bytes.Buffer
	if code := runNext(nil, &out, false, 0, false); code != 0 {
		t.Fatalf("next exit=%d, out=%s", code, out.String())
	}
	if !strings.Contains(out.String(), "FEAT-001") {
		t.Fatalf("legacy item not surfaced:\n%s", out.String())
	}
	out.Reset()
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("status exit=%d", code)
	}
}
