package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestLearningTrivialityCheck_NonTrivialInput(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetIn(strings.NewReader("12\t0\tinternal/foo/foo.go\n"))
	root.SetArgs([]string{"learning", "triviality-check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "non-trivial" {
		t.Errorf("got %q, want non-trivial", got)
	}
}

func TestLearningTrivialityCheck_TrivialInput(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetIn(strings.NewReader("3\t0\tinternal/foo/foo.go\n"))
	root.SetArgs([]string{"learning", "triviality-check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "trivial" {
		t.Errorf("got %q, want trivial", got)
	}
}
