package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHome_HonorsEnvOverride(t *testing.T) {
	t.Setenv("SQUAD_HOME", "/tmp/squad-test-home")
	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/squad-test-home" {
		t.Fatalf("got %q", got)
	}
}

func TestHome_DefaultsToHomeDotSquad(t *testing.T) {
	t.Setenv("SQUAD_HOME", "")
	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, ".squad") {
		t.Fatalf("got %q, want suffix .squad", got)
	}
}

func TestEnsureHome_CreatesSubdirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	if err := EnsureHome(); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"", "archive", "backups"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Fatalf("missing %s: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s not dir", sub)
		}
	}
}

func TestDBPath_UsesHome(t *testing.T) {
	t.Setenv("SQUAD_HOME", "/tmp/squad-test-home")
	got, err := DBPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/squad-test-home/global.db" {
		t.Fatalf("got %q", got)
	}
}
