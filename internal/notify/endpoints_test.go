package notify

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func TestRegister_PersistsRow(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	r := NewRegistry(db)
	if err := r.Register(context.Background(), Endpoint{
		Instance: "inst-1", RepoID: "repo-a", Kind: KindListen, Port: 5050,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	eps, err := r.LookupRepo(context.Background(), "repo-a")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(eps) != 1 || eps[0].Port != 5050 || eps[0].Instance != "inst-1" {
		t.Fatalf("eps=%+v", eps)
	}
}

func TestRegister_OverwritesSamePair(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	r := NewRegistry(db)
	ctx := context.Background()
	if err := r.Register(ctx, Endpoint{Instance: "i", RepoID: "r", Kind: KindListen, Port: 1}); err != nil {
		t.Fatalf("register 1: %v", err)
	}
	if err := r.Register(ctx, Endpoint{Instance: "i", RepoID: "r", Kind: KindListen, Port: 2}); err != nil {
		t.Fatalf("register 2: %v", err)
	}

	eps, _ := r.LookupRepo(ctx, "r")
	if len(eps) != 1 || eps[0].Port != 2 {
		t.Fatalf("expected single endpoint with port=2, got %+v", eps)
	}
}

func TestUnregister_DropsRow(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	r := NewRegistry(db)
	ctx := context.Background()
	_ = r.Register(ctx, Endpoint{Instance: "i", RepoID: "r", Kind: KindListen, Port: 1})

	if err := r.Unregister(ctx, "i", KindListen); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	eps, _ := r.LookupRepo(ctx, "r")
	if len(eps) != 0 {
		t.Fatalf("expected no endpoints, got %+v", eps)
	}
}

func TestUnregisterInstance_DropsAllKinds(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	r := NewRegistry(db)
	ctx := context.Background()
	_ = r.Register(ctx, Endpoint{Instance: "i", RepoID: "r", Kind: KindListen, Port: 1})
	_ = r.Register(ctx, Endpoint{Instance: "i", RepoID: "r", Kind: KindRewake, Port: 2})

	if err := r.UnregisterInstance(ctx, "i"); err != nil {
		t.Fatalf("unregister instance: %v", err)
	}
	eps, _ := r.LookupRepo(ctx, "r")
	if len(eps) != 0 {
		t.Fatalf("expected zero, got %+v", eps)
	}
}
