package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	if versionString == "" {
		t.Fatal("versionString must not be empty")
	}
	if !strings.Contains(versionString, "dev") && !strings.HasPrefix(versionString, "v") {
		t.Fatalf("versionString %q should start with v or contain dev", versionString)
	}
}

func TestVersionCommand_PrintsVersion(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != versionString {
		t.Fatalf("got %q want %q", got, versionString)
	}
}
