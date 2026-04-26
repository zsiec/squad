package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGetItem_ReturnsParsedItem(t *testing.T) {
	dir := t.TempDir()
	body := "---\nid: FEAT-001\ntitle: example\ntype: feat\npriority: P2\nstatus: ready\ncreated: 2026-04-25\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "FEAT-001.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := GetItem(context.Background(), GetItemArgs{ItemsDir: dir, ItemID: "FEAT-001"})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got == nil || got.ID != "FEAT-001" {
		t.Fatalf("got=%+v want ID=FEAT-001", got)
	}
	if got.Title != "example" {
		t.Fatalf("title=%q", got.Title)
	}
}

func TestGetItem_MissingReturnsErrItemNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := GetItem(context.Background(), GetItemArgs{ItemsDir: dir, ItemID: "FEAT-404"})
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("err=%v want ErrItemNotFound", err)
	}
}
