package learning

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPromote_RollsBackDstIfSrcRemoveFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based read-only-dir trick is POSIX-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores file-mode permissions")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(root, ".squad", "learnings", "patterns", "proposed")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(srcDir, "x.md")
	body := writeYAMLFrontmatter("pattern", "x", "store", "proposed", []string{"internal/store/**"})
	if err := os.WriteFile(srcPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(srcDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(srcDir, 0o755) })

	l, err := Parse(srcPath)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	dst, err := Promote(l, StateApproved, nil)
	if err == nil {
		t.Fatalf("expected Promote to fail; got dst=%s", dst)
	}
	if !strings.Contains(err.Error(), "rollback") &&
		!strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error should mention rollback to signal partial-failure cleanup happened, got: %v", err)
	}
	dstPath := PathFor(root, l.Kind, StateApproved, l.Slug)
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Errorf("expected dst %q removed by rollback, got stat err = %v", dstPath, statErr)
	}
}
