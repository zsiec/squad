package installer

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInstall_CopiesAllFiles(t *testing.T) {
	src := fstest.MapFS{
		".claude-plugin/plugin.json": {Data: []byte(`{"name":"squad"}`)},
		"skills/squad-loop.md":       {Data: []byte("# loop")},
		"commands/work.md":           {Data: []byte("# work")},
	}
	dst := t.TempDir()

	if err := Install(src, dst); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, want := range []string{".claude-plugin/plugin.json", "skills/squad-loop.md", "commands/work.md"} {
		got, err := os.ReadFile(filepath.Join(dst, want))
		if err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		if len(got) == 0 {
			t.Fatalf("file %s is empty", want)
		}
	}
}

func TestInstall_AtomicReplace(t *testing.T) {
	src := fstest.MapFS{".claude-plugin/plugin.json": {Data: []byte(`{"name":"squad","version":"new"}`)}}
	dst := t.TempDir()
	pluginDir := filepath.Join(dst, "squad")

	if err := os.MkdirAll(filepath.Join(pluginDir, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, ".claude-plugin", "plugin.json"), []byte(`{"name":"squad","version":"old"}`), 0o644); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	if err := Install(src, pluginDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(pluginDir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != `{"name":"squad","version":"new"}` {
		t.Fatalf("expected new content, got %s", got)
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale.txt should have been removed by atomic replace, err=%v", err)
	}
}

func TestUninstall_RemovesDir(t *testing.T) {
	dst := t.TempDir()
	pluginDir := filepath.Join(dst, "squad")
	if err := os.MkdirAll(filepath.Join(pluginDir, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, ".claude-plugin", "plugin.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Uninstall(pluginDir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatalf("plugin dir should be gone, err=%v", err)
	}
}

func TestUninstall_IdempotentWhenAbsent(t *testing.T) {
	dst := t.TempDir()
	pluginDir := filepath.Join(dst, "never-existed")
	if err := Uninstall(pluginDir); err != nil {
		t.Fatalf("Uninstall on absent dir should be nil, got %v", err)
	}
}

func TestInstall_LandsManifestUnderClaudePluginDir(t *testing.T) {
	src := fstest.MapFS{
		".claude-plugin/plugin.json": {Data: []byte(`{"name":"squad"}`)},
		"skills/squad-loop/SKILL.md": {Data: []byte("# loop")},
	}
	dst := t.TempDir()
	if err := Install(src, dst); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatalf("manifest should be at .claude-plugin/plugin.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy top-level plugin.json should not exist: %v", err)
	}
}
