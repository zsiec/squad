package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error for missing config: %v", err)
	}
	want := []string{"BUG", "FEAT", "TASK", "CHORE"}
	if !reflect.DeepEqual(cfg.IDPrefixes, want) {
		t.Fatalf("default prefixes = %v, want %v", cfg.IDPrefixes, want)
	}
}

func TestLoad_CustomPrefixes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "id_prefixes: [STORY, SPIKE, INC]\n"
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.IDPrefixes, []string{"STORY", "SPIKE", "INC"}) {
		t.Fatalf("got %v", cfg.IDPrefixes)
	}
}

func TestLoad_MalformedYAMLIsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("id_prefixes: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for malformed yaml")
	}
}
