package main

import (
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	got := versionString
	if got == "" {
		t.Fatal("versionString must not be empty")
	}
	if !strings.Contains(got, "dev") && !strings.HasPrefix(got, "v") {
		t.Fatalf("versionString %q should look like a version (start with v) or contain 'dev'", got)
	}
}
