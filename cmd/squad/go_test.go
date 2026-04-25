package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestGoCmd_Exists(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Use == "go" {
			return
		}
	}
	t.Fatal("squad go command not registered on root")
}

func TestGoCmd_HelpMentionsOrchestration(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	body := out.String()
	for _, want := range []string{"register", "claim", "mailbox"} {
		if !strings.Contains(strings.ToLower(body), want) {
			t.Errorf("help should mention %q, got: %s", want, body)
		}
	}
}
