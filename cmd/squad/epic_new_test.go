package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEpicNew_RequiresExistingSpec(t *testing.T) {
	enterRepo(t)
	var out bytes.Buffer
	if code := runEpicNew([]string{"login-redirect"}, "auth-rework", &out); code == 0 {
		t.Fatal("expected nonzero — spec does not exist")
	}
}

func TestEpicNew_HappyPath(t *testing.T) {
	dir := enterRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".squad", "specs", "auth-rework.md"),
		[]byte("---\ntitle: Auth\nmotivation: x\nacceptance: [x]\n---\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := runEpicNew([]string{"login-redirect"}, "auth-rework", &out); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".squad", "epics", "login-redirect.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{"spec: auth-rework", "status: open"} {
		if !bytes.Contains(body, []byte(frag)) {
			t.Errorf("epic missing %q", frag)
		}
	}
}
