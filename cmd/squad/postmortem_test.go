package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/config"
)

// gitInitForPostmortem inits a tiny repo with a single committed
// item file at a backdated time, so the file is OUT of the default
// claim window and the detector doesn't fire on the initial commit.
func gitInitForPostmortem(t *testing.T, dir, itemRel string) string {
	t.Helper()
	for _, cmd := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v: %s", cmd, err, out)
		}
	}
	full := filepath.Join(dir, itemRel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", itemRel)
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	when := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	commit := exec.Command("git", "commit", "-q", "-m", "initial")
	commit.Dir = dir
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_DATE="+when,
	)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	return full
}

// TestRunPostmortem_DispatchPathRendersPrompt covers AC#5: a release
// with no artifacts triggers dispatch; in --print-prompt mode the
// rendered prompt comes back and no claude binary is required.
func TestRunPostmortem_DispatchPathRendersPrompt(t *testing.T) {
	env := newTestEnv(t)
	itemRel := filepath.Join(".squad", "items", "FEAT-X.md")
	itemPath := gitInitForPostmortem(t, env.Root, itemRel)
	now := time.Now().Unix()
	res, err := RunPostmortem(context.Background(), PostmortemArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
		ItemID: "FEAT-X", ItemPath: itemPath, AgentID: "agent-test",
		ClaimedAt: now - 7200, ReleasedAt: now,
		Cfg:       config.PostmortemConfig{},
		PrintOnly: true,
	})
	if err != nil {
		t.Fatalf("RunPostmortem: %v", err)
	}
	if !res.Decision.Dispatch {
		t.Fatalf("expected dispatch=true, got %+v", res.Decision)
	}
	if res.Invoked {
		t.Errorf("PrintOnly should not invoke; Invoked=true")
	}
	if !strings.Contains(res.Prompt, "FEAT-X") {
		t.Errorf("prompt missing item id:\n%s", res.Prompt)
	}
	if !strings.Contains(res.Prompt, "squad learning propose dead-end") {
		t.Errorf("prompt missing learning-propose instruction:\n%s", res.Prompt)
	}
}

// AC#8: enabled:false short-circuits regardless of signal state.
func TestRunPostmortem_DisabledShortCircuits(t *testing.T) {
	env := newTestEnv(t)
	itemRel := filepath.Join(".squad", "items", "FEAT-Y.md")
	itemPath := gitInitForPostmortem(t, env.Root, itemRel)
	now := time.Now().Unix()
	disabled := false
	res, err := RunPostmortem(context.Background(), PostmortemArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
		ItemID: "FEAT-Y", ItemPath: itemPath, AgentID: "agent-test",
		ClaimedAt: now - 7200, ReleasedAt: now,
		Cfg: config.PostmortemConfig{Enabled: &disabled},
	})
	if err != nil {
		t.Fatalf("RunPostmortem: %v", err)
	}
	if res.Decision.Dispatch {
		t.Errorf("disabled config must skip dispatch, got %+v", res.Decision)
	}
	if res.Prompt != "" {
		t.Errorf("disabled run should not render prompt, got: %s", res.Prompt)
	}
}

// TestLoadLastReleaseFor_PrefersNonDone covers H1: an item with both
// a released-without-done row AND a later `done` row must surface the
// FORMER. The postmortem flow only scans windows where the lesson
// might have been lost; a trailing `done` means the lesson got
// resolved by the re-claim and the postmortem is moot.
func TestLoadLastReleaseFor_PrefersNonDone(t *testing.T) {
	env := newTestEnv(t)
	itemID := "FEAT-Q"
	first := time.Now().Add(-3 * time.Hour).Unix()
	if _, err := env.DB.Exec(`INSERT INTO claim_history (repo_id, item_id, agent_id,
		claimed_at, released_at, outcome) VALUES (?, ?, 'agent-x', ?, ?, 'released')`,
		env.RepoID, itemID, first, first+3600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(-1 * time.Hour).Unix()
	if _, err := env.DB.Exec(`INSERT INTO claim_history (repo_id, item_id, agent_id,
		claimed_at, released_at, outcome) VALUES (?, ?, 'agent-y', ?, ?, 'done')`,
		env.RepoID, itemID, later, later+1800); err != nil {
		t.Fatal(err)
	}
	gotAgent, gotClaimed, _, err := loadLastReleaseFor(context.Background(), env.DB, env.RepoID, itemID)
	if err != nil {
		t.Fatalf("loadLastReleaseFor: %v", err)
	}
	if gotAgent != "agent-x" {
		t.Errorf("agent=%q want agent-x (the released-without-done row, NOT the trailing done)", gotAgent)
	}
	if gotClaimed != first {
		t.Errorf("claimed_at=%d want %d (the earlier released-row window)", gotClaimed, first)
	}
}

// TestRunPostmortem_InjectedDispatcher verifies the test-only
// dispatcher hook works and is invoked when Decision.Dispatch is
// true and PrintOnly is false. AC#5's "agent invocation" half is
// represented by the dispatcher being called with a non-empty prompt.
func TestRunPostmortem_InjectedDispatcher(t *testing.T) {
	env := newTestEnv(t)
	itemRel := filepath.Join(".squad", "items", "FEAT-Z.md")
	itemPath := gitInitForPostmortem(t, env.Root, itemRel)
	now := time.Now().Unix()
	called := false
	var capturedPrompt string
	res, err := RunPostmortem(context.Background(), PostmortemArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
		ItemID: "FEAT-Z", ItemPath: itemPath, AgentID: "agent-test",
		ClaimedAt: now - 7200, ReleasedAt: now,
		Cfg: config.PostmortemConfig{},
		Dispatcher: func(_ context.Context, prompt string) error {
			called = true
			capturedPrompt = prompt
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunPostmortem: %v", err)
	}
	if !called {
		t.Errorf("dispatcher not invoked; res=%+v", res)
	}
	if !res.Invoked {
		t.Errorf("res.Invoked=false but dispatcher was called")
	}
	if !strings.Contains(capturedPrompt, "FEAT-Z") {
		t.Errorf("captured prompt missing item id:\n%s", capturedPrompt)
	}
}
