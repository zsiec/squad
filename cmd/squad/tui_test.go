package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTUI_HelpRegistered(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"tui", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "k9s-style terminal UI") {
		t.Fatalf("tui help missing description: %s", out.String())
	}
}
