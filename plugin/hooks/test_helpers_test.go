package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeFixtureScript(t *testing.T, name string) string {
	t.Helper()
	body, err := FS.ReadFile(name)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, body, 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func fixturePathInRepo(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), name)
}

func requireDash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("dash"); err != nil {
		t.Skip("dash not installed")
	}
}

func writeStubSquad(t *testing.T, body string) string {
	t.Helper()
	now := strconv.FormatInt(time.Now().Unix(), 10)
	body = strings.ReplaceAll(body, "__NOW__", now)
	dir := t.TempDir()
	p := filepath.Join(dir, "squad")
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
  whoami)    printf '%%s\n' '%s' ;;
  workspace) printf '%%s\n' '{"current_repo":"my-repo"}' ;;
  next)      printf '%%s\n' '[{"id":"FEAT-1"},{"id":"BUG-2"}]' ;;
  *)         exit 0 ;;
esac
`, body)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return p
}
