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

func TestReadme_AgentQuickstartBeforeHumanQuickstart(t *testing.T) {
	body, err := os.ReadFile(locateReadme(t))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	agentIdx := strings.Index(s, "Agent quickstart")
	humanIdx := strings.Index(s, "Human quickstart")
	if agentIdx < 0 {
		t.Fatal("README missing 'Agent quickstart' section")
	}
	if humanIdx < 0 {
		t.Fatal("README missing 'Human quickstart' section")
	}
	if agentIdx >= humanIdx {
		t.Fatalf("Agent quickstart should appear before Human quickstart; got agent=%d human=%d",
			agentIdx, humanIdx)
	}
	if !strings.Contains(s, "squad go") {
		t.Fatal("README should mention `squad go`")
	}
}
