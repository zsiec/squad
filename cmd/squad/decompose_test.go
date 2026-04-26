package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const decomposeSpecFixture = `---
title: Auth rotation
motivation: |
  We need rotating credentials.
acceptance:
  - tokens rotate every 24h
non_goals:
  - rotating root keys
integration:
  - internal/auth
---

## Background

placeholder
`

func setupDecomposeRepo(t *testing.T, specName string) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	dir = resolved
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "specs"), 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if specName != "" {
		path := filepath.Join(dir, ".squad", "specs", specName+".md")
		if err := os.WriteFile(path, []byte(decomposeSpecFixture), 0o644); err != nil {
			t.Fatalf("write spec: %v", err)
		}
	}
	t.Chdir(dir)
	return dir
}

func TestRunDecompose_PrintPromptForExistingSpec(t *testing.T) {
	dir := setupDecomposeRepo(t, "auth-rotation")

	var stdout, stderr bytes.Buffer
	code := runDecompose(context.Background(), "auth-rotation", true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	got := stdout.String()
	wantSpecPath := filepath.Join(dir, ".squad", "specs", "auth-rotation.md")
	for _, frag := range []string{
		"decomposing a squad spec",
		wantSpecPath,
		"squad_new",
		`parent_spec="auth-rotation"`,
	} {
		if !strings.Contains(got, frag) {
			t.Fatalf("stdout missing %q\nstdout=%s", frag, got)
		}
	}
}

func TestRunDecompose_PromptIsBytewiseStable(t *testing.T) {
	setupDecomposeRepo(t, "auth-rotation")

	var first, second bytes.Buffer
	if code := runDecompose(context.Background(), "auth-rotation", true, &first, &bytes.Buffer{}); code != 0 {
		t.Fatalf("first run exit=%d", code)
	}
	if code := runDecompose(context.Background(), "auth-rotation", true, &second, &bytes.Buffer{}); code != 0 {
		t.Fatalf("second run exit=%d", code)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("prompt is not byte-stable\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}
}

func TestRunDecompose_MissingSpecErrors(t *testing.T) {
	setupDecomposeRepo(t, "")

	var stdout, stderr bytes.Buffer
	code := runDecompose(context.Background(), "nonexistent-spec", true, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("exit=%d want 4\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "not found") {
		t.Fatalf("stderr missing 'not found': %s", errOut)
	}
	if !strings.Contains(errOut, "squad spec-list") {
		t.Fatalf("stderr missing 'squad spec-list' hint: %s", errOut)
	}
}

func TestRunDecompose_NoClaudeInPathFallsBackToPrintPrompt(t *testing.T) {
	setupDecomposeRepo(t, "auth-rotation")

	emptyPath := t.TempDir()
	t.Setenv("PATH", emptyPath)

	var stdout, stderr bytes.Buffer
	code := runDecompose(context.Background(), "auth-rotation", false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "claude binary not found") {
		t.Fatalf("stderr missing 'claude binary not found': %s", errOut)
	}
	if !strings.Contains(stdout.String(), "decomposing a squad spec") {
		t.Fatalf("stdout missing fallback prompt: %s", stdout.String())
	}
}
