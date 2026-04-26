package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func locateReadme(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	root := wd
	for {
		p := filepath.Join(root, "README.md")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not locate README.md")
		}
		root = parent
	}
}

// TestReadme_QuickStartIsMCPFirst pins the README's structure to the
// MCP-first voice. The Quick start section must lead with `claude install`
// (the Claude Code path); the CLI form (`squad go`) must only appear later
// as a power-user fallback. The previous shape — two parallel quickstarts
// contradicting each other on which command to type — was actively
// confusing for Claude Code users.
func TestReadme_QuickStartIsMCPFirst(t *testing.T) {
	body, err := os.ReadFile(locateReadme(t))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)

	quickStartIdx := strings.Index(s, "## Quick start")
	if quickStartIdx < 0 {
		t.Fatal("README missing '## Quick start' section")
	}
	beyondIdx := strings.Index(s, "## Beyond the quick start")
	if beyondIdx < 0 {
		t.Fatal("README missing '## Beyond the quick start' section")
	}
	if quickStartIdx >= beyondIdx {
		t.Fatalf("'Quick start' should appear before 'Beyond the quick start'; got quick=%d beyond=%d",
			quickStartIdx, beyondIdx)
	}

	claudeInstallIdx := strings.Index(s, "claude install github.com/zsiec/squad")
	if claudeInstallIdx < 0 {
		t.Fatal("README quick-start should show 'claude install github.com/zsiec/squad'")
	}
	if claudeInstallIdx < quickStartIdx || claudeInstallIdx > beyondIdx {
		t.Fatal("'claude install' should be inside the Quick start section, not after Beyond")
	}

	squadGoIdx := strings.Index(s, "squad go")
	if squadGoIdx < 0 {
		t.Fatal("README should mention `squad go` as the CLI fallback")
	}
	if squadGoIdx < beyondIdx {
		t.Fatalf("`squad go` should appear in 'Beyond the quick start' as a fallback, not in Quick start; got squad-go=%d beyond=%d",
			squadGoIdx, beyondIdx)
	}
}
