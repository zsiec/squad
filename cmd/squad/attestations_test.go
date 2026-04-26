package main

import (
	"context"
	"testing"

	"github.com/zsiec/squad/internal/attest"
)

func TestAttestations_PureEmpty(t *testing.T) {
	env := newTestEnv(t)

	res, err := Attestations(context.Background(), AttestationsArgs{
		DB:     env.DB,
		RepoID: env.RepoID,
		ItemID: "BUG-NONE",
	})
	if err != nil {
		t.Fatalf("Attestations: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if res.Count != 0 {
		t.Errorf("Count=%d want 0", res.Count)
	}
	if len(res.Items) != 0 {
		t.Errorf("len(Items)=%d want 0", len(res.Items))
	}
}

func TestAttestations_PurePopulated(t *testing.T) {
	env := newTestEnv(t)
	L := attest.New(env.DB, env.RepoID, nil)
	for i, hash := range []string{"hash-aaa", "hash-bbb"} {
		if _, err := L.Insert(context.Background(), attest.Record{
			ItemID:     "BUG-700",
			Kind:       attest.KindTest,
			Command:    "true",
			ExitCode:   0,
			OutputHash: hash,
			OutputPath: "/tmp/" + hash + ".txt",
			AgentID:    env.AgentID,
			CreatedAt:  int64(1000 + i),
		}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	res, err := Attestations(context.Background(), AttestationsArgs{
		DB:     env.DB,
		RepoID: env.RepoID,
		ItemID: "BUG-700",
	})
	if err != nil {
		t.Fatalf("Attestations: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("Count=%d want 2", res.Count)
	}
	if len(res.Items) != 2 {
		t.Fatalf("len(Items)=%d want 2", len(res.Items))
	}
	if res.Items[0].ItemID != "BUG-700" || res.Items[0].Kind != "test" {
		t.Errorf("row 0 = %+v", res.Items[0])
	}
	if res.Items[0].OutputHash != "hash-aaa" || res.Items[1].OutputHash != "hash-bbb" {
		t.Errorf("hash order: %q %q", res.Items[0].OutputHash, res.Items[1].OutputHash)
	}
}
