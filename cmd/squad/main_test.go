package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	if versionString == "" {
		t.Fatal("versionString must not be empty")
	}
	semver := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	if !semver.MatchString(versionString) {
		t.Fatalf("versionString %q is not a valid semver", versionString)
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
