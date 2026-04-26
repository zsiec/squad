package main

import (
	"context"
	"testing"
)

func TestWhoami_PureReturnsAgentID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-whoami-pure")
	t.Setenv("SQUAD_AGENT", "")

	if _, err := Register(context.Background(), RegisterArgs{
		As:          "agent-zed-pure",
		NoRepoCheck: true,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	res, err := Whoami(context.Background(), WhoamiArgs{})
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if res == nil || res.AgentID != "agent-zed-pure" {
		t.Fatalf("got=%+v", res)
	}
}
