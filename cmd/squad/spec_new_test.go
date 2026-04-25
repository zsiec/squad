package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func enterRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	dir = resolved
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSpecNew_WritesScaffold(t *testing.T) {
	dir := enterRepo(t)
	var out bytes.Buffer
	if code := runSpecNew([]string{"auth-rework", "Auth rework"}, &out); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	want := filepath.Join(dir, ".squad", "specs", "auth-rework.md")
	if strings.TrimSpace(out.String()) != want {
		t.Errorf("printed=%q want %q", out.String(), want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{
		"title: Auth rework", "motivation:", "acceptance:", "non_goals:", "integration:",
	} {
		if !bytes.Contains(body, []byte(frag)) {
			t.Errorf("scaffold missing %q", frag)
		}
	}
}

func TestSpecNew_RejectsExisting(t *testing.T) {
	enterRepo(t)
	var out bytes.Buffer
	if code := runSpecNew([]string{"auth-rework", "first"}, &out); code != 0 {
		t.Fatalf("first call exit=%d", code)
	}
	out.Reset()
	if code := runSpecNew([]string{"auth-rework", "second"}, &out); code == 0 {
		t.Fatal("second call should refuse to overwrite")
	}
}
