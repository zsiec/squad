package hygiene

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/plugin/hooks"
)

func installAllHooks(t *testing.T, dir string) {
	t.Helper()
	entries, err := fs.ReadDir(hooks.FS, ".")
	if err != nil {
		t.Fatalf("read embed: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sh" {
			continue
		}
		body, err := fs.ReadFile(hooks.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), body, 0o755); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}

func TestDetectHookDrift_NoDriftWhenInstalledMatchesEmbedded(t *testing.T) {
	dir := t.TempDir()
	installAllHooks(t, dir)
	findings, err := DetectHookDrift(dir)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("want no findings, got %d: %+v", len(findings), findings)
	}
}

func TestDetectHookDrift_ReportsModifiedHook(t *testing.T) {
	dir := t.TempDir()
	installAllHooks(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "session_start.sh"), []byte("# tampered\n"), 0o755); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	findings, err := DetectHookDrift(dir)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Filename != "session_start.sh" {
		t.Fatalf("want session_start.sh, got %q", findings[0].Filename)
	}
	if findings[0].Kind != DriftModified {
		t.Fatalf("want DriftModified, got %v", findings[0].Kind)
	}
}

func TestDetectHookDrift_ReportsMissingHook(t *testing.T) {
	dir := t.TempDir()
	installAllHooks(t, dir)
	if err := os.Remove(filepath.Join(dir, "session_start.sh")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	findings, err := DetectHookDrift(dir)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Kind != DriftMissing {
		t.Fatalf("want DriftMissing, got %v", findings[0].Kind)
	}
}

func TestDetectHookDrift_IgnoresUnknownFiles(t *testing.T) {
	dir := t.TempDir()
	installAllHooks(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "user_added_thing.sh"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	findings, err := DetectHookDrift(dir)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected unknown files to be ignored; got %+v", findings)
	}
}

func TestDetectHookDrift_NonexistentDirectoryReturnsNothing(t *testing.T) {
	findings, err := DetectHookDrift("/this/path/does/not/exist")
	if err != nil {
		t.Fatalf("detect should not error on missing dir: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("want empty findings for missing dir, got %+v", findings)
	}
}

func TestDriftKindString(t *testing.T) {
	if got := DriftModified.String(); got != "modified" {
		t.Fatalf("DriftModified.String() = %q, want %q", got, "modified")
	}
	if got := DriftMissing.String(); got != "missing" {
		t.Fatalf("DriftMissing.String() = %q, want %q", got, "missing")
	}
}
