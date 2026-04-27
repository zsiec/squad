package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecordEvent_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	clock := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	res, err := RecordEvent(context.Background(), RecordEventArgs{
		DB:         env.DB,
		RepoID:     env.RepoID,
		AgentID:    env.AgentID,
		SessionID:  "sess-abc",
		Kind:       "pre_tool",
		Tool:       "Bash",
		Target:     "ls -la",
		ExitCode:   0,
		DurationMs: 42,
		Now:        func() time.Time { return clock },
	})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	if res == nil || res.ID == 0 {
		t.Fatalf("want non-nil result with ID; got %+v", res)
	}

	var (
		repoID, agentID, sess, kind, tool, target string
		ts, exit, dur                             int64
	)
	row := env.DB.QueryRow(
		`SELECT repo_id, agent_id, session_id, ts, event_kind, tool, target, exit_code, duration_ms
		 FROM agent_events WHERE id = ?`, res.ID,
	)
	if err := row.Scan(&repoID, &agentID, &sess, &ts, &kind, &tool, &target, &exit, &dur); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if repoID != env.RepoID || agentID != env.AgentID {
		t.Errorf("repo/agent mismatch: %s/%s want %s/%s", repoID, agentID, env.RepoID, env.AgentID)
	}
	if sess != "sess-abc" || kind != "pre_tool" || tool != "Bash" || target != "ls -la" {
		t.Errorf("payload mismatch: sess=%q kind=%q tool=%q target=%q", sess, kind, tool, target)
	}
	if ts != clock.Unix() {
		t.Errorf("ts=%d want %d", ts, clock.Unix())
	}
	if exit != 0 || dur != 42 {
		t.Errorf("exit/dur mismatch: exit=%d dur=%d", exit, dur)
	}
}

func TestRecordEvent_RejectsInvalidKind(t *testing.T) {
	env := newTestEnv(t)

	_, err := RecordEvent(context.Background(), RecordEventArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Kind:    "fabricated",
	})
	if !errors.Is(err, ErrInvalidEventKind) {
		t.Fatalf("err=%v want ErrInvalidEventKind", err)
	}

	_, err = RecordEvent(context.Background(), RecordEventArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Kind:    "",
	})
	if !errors.Is(err, ErrInvalidEventKind) {
		t.Fatalf("empty kind err=%v want ErrInvalidEventKind", err)
	}
}

func TestRecordEvent_AcceptsAllValidKinds(t *testing.T) {
	env := newTestEnv(t)
	for _, k := range []string{"pre_tool", "post_tool", "subagent_start", "subagent_stop"} {
		if _, err := RecordEvent(context.Background(), RecordEventArgs{
			DB:      env.DB,
			RepoID:  env.RepoID,
			AgentID: env.AgentID,
			Kind:    k,
		}); err != nil {
			t.Errorf("kind=%s: %v", k, err)
		}
	}
}

func TestEventRecord_CobraSilentOnBootFailure(t *testing.T) {
	bare := t.TempDir()
	t.Chdir(bare)
	t.Setenv("SQUAD_HOME", t.TempDir())

	cmd := newEventCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"record", "--kind", "pre_tool", "--tool", "Bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned non-nil error %v; hooks must never fail", err)
	}
	if stderr.Len() == 0 {
		t.Errorf("want stderr diagnostic when boot fails; got empty")
	}
}

func TestEventRecord_CobraSessionEnvFallback(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("SQUAD_SESSION_ID", "from-env-session")

	cmd := newEventCmd()
	stderr := &bytes.Buffer{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"record", "--kind", "post_tool", "--tool", "Edit", "--target", "x.go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stderr.Len() > 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	// On macOS the test's literal t.TempDir() path differs from the cobra
	// command's resolved os.Getwd() path (/private/var vs /var), so the
	// repo_id derived inside the cobra invocation diverges from env.RepoID.
	// The fact under test is the env-fallback for session id, not the
	// repo-id derivation, so query by session_id directly.
	var sess string
	if err := env.DB.QueryRow(
		`SELECT session_id FROM agent_events ORDER BY id DESC LIMIT 1`,
	).Scan(&sess); err != nil {
		t.Fatalf("query last event: %v", err)
	}
	if sess != "from-env-session" {
		t.Errorf("session=%q want from-env-session (env fallback)", sess)
	}
}

func TestEventRecord_CobraSilentOnInvalidKind(t *testing.T) {
	env := newTestEnv(t)

	cmd := newEventCmd()
	stderr := &bytes.Buffer{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"record", "--kind", "bogus"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned %v; must exit 0 even on bad input", err)
	}
	if !strings.Contains(stderr.String(), "invalid event kind") {
		t.Errorf("stderr=%q want diagnostic mentioning invalid event kind", stderr.String())
	}

	var n int
	_ = env.DB.QueryRow(`SELECT count(*) FROM agent_events`).Scan(&n)
	if n != 0 {
		t.Errorf("want 0 rows on rejected kind; got %d", n)
	}
}
