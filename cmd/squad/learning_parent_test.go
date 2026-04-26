package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestLearning_ParentListsSubcommands(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"learning", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, want := range []string{
		"propose", "list", "approve", "reject",
		"agents-md-suggest", "agents-md-approve", "agents-md-reject",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--help missing %q:\n%s", want, out.String())
		}
	}
	if !strings.Contains(out.String(), "AGENTS.md follows a stricter gate") {
		t.Errorf("--help missing Long text:\n%s", out.String())
	}
}
