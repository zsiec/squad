package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/notify"
	"github.com/zsiec/squad/internal/store"
)

func TestRunNotifyCleanup_DropsAllKindsForInstance(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := notify.NewRegistry(db)
	ctx := context.Background()
	_ = r.Register(ctx, notify.Endpoint{Instance: "i", RepoID: "r", Kind: notify.KindListen, Port: 1})
	_ = r.Register(ctx, notify.Endpoint{Instance: "i", RepoID: "r", Kind: notify.KindRewake, Port: 2})
	_ = r.Register(ctx, notify.Endpoint{Instance: "j", RepoID: "r", Kind: notify.KindListen, Port: 3})

	if code := runNotifyCleanup(ctx, r, "i"); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	eps, _ := r.LookupRepo(ctx, "r")
	if len(eps) != 1 || eps[0].Instance != "j" {
		t.Fatalf("expected only instance j to remain, got %+v", eps)
	}
}
