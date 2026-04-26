package learning

import (
	"path/filepath"
	"strings"
	"testing"
)

const validGotcha = `---
id: gotcha-2026-04-25-sqlite-busy-on-fork
kind: gotcha
slug: sqlite-busy-on-fork
title: SQLite returns SQLITE_BUSY across fork
area: store
paths:
  - internal/store/**
created: 2026-04-25
created_by: agent-a3f4
session: 9c1e2b
state: proposed
---

## Looks like

A SQLITE_BUSY error mid-test.

## Is

The lock is on the inode.

## So

Don't fork; use t.TempDir.
`

func TestParse_ValidGotcha(t *testing.T) {
	p := filepath.Join(t.TempDir(), "g.md")
	writeFile(t, p, validGotcha)
	got, err := Parse(p)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind != KindGotcha || got.Slug != "sqlite-busy-on-fork" ||
		got.Area != "store" || got.State != StateProposed ||
		len(got.Paths) != 1 || got.Paths[0] != "internal/store/**" {
		t.Errorf("unexpected parse result: %+v", got)
	}
}

func TestParse_RejectsMalformedKind(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.md")
	writeFile(t, p, strings.Replace(validGotcha, "kind: gotcha", "kind: rumor", 1))
	_, err := Parse(p)
	if err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("want kind error, got %v", err)
	}
}

func TestParse_GotchaRequiresLooksLikeAndIs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "g.md")
	writeFile(t, p, strings.Replace(validGotcha, "## Is\n\nThe lock is on the inode.\n\n", "", 1))
	_, err := Parse(p)
	if err == nil || !strings.Contains(err.Error(), "## Is") {
		t.Fatalf("want '## Is' error, got %v", err)
	}
}

func makeFrontmatter(kind, body string) string {
	return "---\nid: " + kind + "-x\nkind: " + kind +
		"\nslug: x\ntitle: t\narea: a\ncreated: 2026-04-25\ncreated_by: agent-x\nsession: s\nstate: proposed\n---\n\n" + body
}

func TestParse_PatternRequiresWhenDoWhy(t *testing.T) {
	p := filepath.Join(t.TempDir(), "p.md")
	writeFile(t, p, makeFrontmatter("pattern", "## When\n\nx\n\n## Do\n\ny\n"))
	_, err := Parse(p)
	if err == nil || !strings.Contains(err.Error(), "## Why") {
		t.Fatalf("want '## Why' error, got %v", err)
	}
}

func TestParse_DeadEndRequiresTriedAndDoesntWork(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.md")
	writeFile(t, p, makeFrontmatter("dead-end", "## Tried\n\nx\n"))
	_, err := Parse(p)
	if err == nil || !strings.Contains(err.Error(), "Doesn't work because") {
		t.Fatalf("want 'Doesn't work because' error, got %v", err)
	}
}
