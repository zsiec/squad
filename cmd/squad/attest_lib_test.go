package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAttest_PureHappyPath(t *testing.T) {
	env := newTestEnv(t)
	attDir := filepath.Join(env.Root, ".squad", "attestations")

	res, err := Attest(context.Background(), AttestArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		ItemID:  "BUG-500",
		Kind:    "test",
		Command: "echo hello",
		AttDir:  attDir,
	})
	if err != nil {
		t.Fatalf("Attest: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode=%d want 0", res.ExitCode)
	}
	if res.ItemID != "BUG-500" {
		t.Errorf("ItemID=%q want BUG-500", res.ItemID)
	}
	if res.Kind != "test" {
		t.Errorf("Kind=%q want test", res.Kind)
	}
	if res.OutputPath == "" {
		t.Fatal("OutputPath empty")
	}
	if _, err := os.Stat(res.OutputPath); err != nil {
		t.Fatalf("artifact missing: %v", err)
	}
}

func TestAttest_PureRejectsInvalidKind(t *testing.T) {
	env := newTestEnv(t)
	attDir := filepath.Join(env.Root, ".squad", "attestations")

	_, err := Attest(context.Background(), AttestArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		ItemID:  "BUG-501",
		Kind:    "fabricated",
		Command: "true",
		AttDir:  attDir,
	})
	if !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("err=%v want ErrInvalidKind", err)
	}
}
