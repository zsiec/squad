package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustContain(t *testing.T, body []byte, sub string) {
	t.Helper()
	if !strings.Contains(string(body), sub) {
		t.Fatalf("expected %q in:\n%s", sub, body)
	}
}

func mustNotContain(t *testing.T, body []byte, sub string) {
	t.Helper()
	if strings.Contains(string(body), sub) {
		t.Fatalf("did not expect %q in:\n%s", sub, body)
	}
}

func TestInstallHooksCmd_YesDefaultsOnlySessionStart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := runInstallHooks([]string{"--yes"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, body, "session-start@v1")
	mustNotContain(t, body, "pre-commit-tick@v1")
}

func TestInstallHooksCmd_PerHookFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	args := []string{"--yes",
		"--session-start=on",
		"--pre-commit-tick=on",
		"--pre-commit-pm-traces=off",
		"--pre-edit-touch-check=on",
		"--stop-handoff=off",
	}
	if err := runInstallHooks(args, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	mustContain(t, body, "session-start@v1")
	mustContain(t, body, "pre-commit-tick@v1")
	mustContain(t, body, "pre-edit-touch-check@v1")
	mustNotContain(t, body, "pre-commit-pm-traces@v1")
	mustNotContain(t, body, "stop-handoff@v1")
}

func TestInstallHooksCmd_Status(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := runInstallHooks([]string{"--yes"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	if err := runInstallHooks([]string{"--status"}, stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	mustContain(t, stdout.Bytes(), "session-start")
	mustContain(t, stdout.Bytes(), "ON")
	mustContain(t, stdout.Bytes(), "pre-commit-tick")
	mustContain(t, stdout.Bytes(), "OFF")
}

func TestInstallHooksCmd_Uninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := runInstallHooks([]string{"--yes"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := runInstallHooks([]string{"--uninstall"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	mustNotContain(t, body, "session-start@v1")
}
