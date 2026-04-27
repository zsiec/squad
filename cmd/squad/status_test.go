package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func TestRunStatus_PrintsCounts(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, ".squad", "items", name),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BUG-001-ready.md", "---\nid: BUG-001\ntitle: ready\ntype: bug\npriority: P1\nstatus: open\nestimate: 1h\n---\n")
	write("BUG-002-ready.md", "---\nid: BUG-002\ntitle: ready\ntype: bug\npriority: P2\nstatus: open\nestimate: 1h\n---\n")
	write("BUG-003-blocked.md", "---\nid: BUG-003\ntitle: blocked\ntype: bug\npriority: P1\nstatus: blocked\nestimate: 1h\n---\n")

	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)
	var stdout bytes.Buffer
	code := runStatus(nil, &stdout)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"claimed: 0", "ready: 2", "blocked: 1", "done: 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunStatus_SubtractsClaimedFromReady(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "items", "FEAT-001-x.md"),
		[]byte("---\nid: FEAT-001\ntitle: x\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "items", "FEAT-002-y.md"),
		[]byte("---\nid: FEAT-002\ntitle: y\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := filepath.Join(dir, "home")
	t.Setenv("SQUAD_HOME", home)
	t.Setenv("SQUAD_AGENT", "agent-test")
	t.Setenv("SQUAD_SESSION_ID", "test-status")
	t.Chdir(dir)

	// Register, claim FEAT-001, verify status reports it as claimed not ready.
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"register", "--as", "agent-test", "--name", "Tester"})
	if err := root.Execute(); err != nil {
		t.Fatalf("register: %v", err)
	}

	root2 := newRootCmd()
	root2.SetOut(&bytes.Buffer{})
	root2.SetErr(&bytes.Buffer{})
	root2.SetArgs([]string{"claim", "FEAT-001", "--intent", "test"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	var out bytes.Buffer
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("status exit=%d", code)
	}
	body := out.String()
	if !strings.Contains(body, "claimed: 1") {
		t.Errorf("expected claimed: 1, got:\n%s", body)
	}
	if !strings.Contains(body, "ready: 1") {
		t.Errorf("expected ready: 1 (FEAT-001 claimed → only FEAT-002 ready), got:\n%s", body)
	}
}

func setupTwoAgentClaims(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"FEAT-001", "FEAT-002"} {
		body := "---\nid: " + id + "\ntitle: x\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"
		if err := os.WriteFile(filepath.Join(dir, ".squad", "items", id+"-x.md"),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)

	registerAndClaim := func(agent, sessionID, itemID string) {
		t.Setenv("SQUAD_AGENT", agent)
		t.Setenv("SQUAD_SESSION_ID", sessionID)
		root := newRootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"register", "--as", agent, "--name", agent})
		if err := root.Execute(); err != nil {
			t.Fatalf("register %s: %v", agent, err)
		}
		root2 := newRootCmd()
		root2.SetOut(&bytes.Buffer{})
		root2.SetErr(&bytes.Buffer{})
		root2.SetArgs([]string{"claim", itemID, "--intent", "x"})
		if err := root2.Execute(); err != nil {
			t.Fatalf("claim %s by %s: %v", itemID, agent, err)
		}
	}
	registerAndClaim("agent-aaa", "test-status-multi-aaa", "FEAT-001")
	registerAndClaim("agent-bbb", "test-status-multi-bbb", "FEAT-002")
	return dir
}

func TestRunStatus_MultiAgentPrintsBreakdown(t *testing.T) {
	_ = setupTwoAgentClaims(t)

	var out bytes.Buffer
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("status exit=%d stdout=%q", code, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "claimed: 2") {
		t.Errorf("expected claimed: 2, got:\n%s", body)
	}
	for _, want := range []string{"agent-aaa", "agent-bbb", "FEAT-001", "FEAT-002"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if i := strings.Index(body, "agent-aaa"); i < 0 ||
		strings.Index(body, "agent-bbb") < i {
		t.Errorf("agent breakdown not sorted ascending:\n%s", body)
	}
}

func TestRunStatus_JSONFlagEmitsHolders(t *testing.T) {
	_ = setupTwoAgentClaims(t)

	var out bytes.Buffer
	if code := runStatusWithJSON(true, &out); code != 0 {
		t.Fatalf("status --json exit=%d stdout=%q", code, out.String())
	}
	var got StatusResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	if got.Claimed != 2 {
		t.Errorf("claimed=%d want 2", got.Claimed)
	}
	if len(got.ClaimedBy) != 2 {
		t.Fatalf("claimed_by=%v want 2 agents", got.ClaimedBy)
	}
	if items := got.ClaimedBy["agent-aaa"]; len(items) != 1 || items[0] != "FEAT-001" {
		t.Errorf("agent-aaa items=%v want [FEAT-001]", items)
	}
	if items := got.ClaimedBy["agent-bbb"]; len(items) != 1 || items[0] != "FEAT-002" {
		t.Errorf("agent-bbb items=%v want [FEAT-002]", items)
	}
}

func TestRunStatus_SingleClaimOmitsBreakdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "items", "FEAT-001-x.md"),
		[]byte("---\nid: FEAT-001\ntitle: x\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Setenv("SQUAD_AGENT", "agent-only")
	t.Setenv("SQUAD_SESSION_ID", "test-status-single")
	t.Chdir(dir)

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"register", "--as", "agent-only", "--name", "x"})
	if err := root.Execute(); err != nil {
		t.Fatalf("register: %v", err)
	}
	root2 := newRootCmd()
	root2.SetOut(&bytes.Buffer{})
	root2.SetErr(&bytes.Buffer{})
	root2.SetArgs([]string{"claim", "FEAT-001", "--intent", "x"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	var out bytes.Buffer
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("status exit=%d", code)
	}
	body := out.String()
	if !strings.Contains(body, "claimed: 1") {
		t.Errorf("expected claimed: 1:\n%s", body)
	}
	if strings.Contains(body, "agent-only") {
		t.Errorf("single-claim breakdown should not be printed:\n%s", body)
	}
}

func TestRunStatus_SingleAgentMultiClaimOmitsBreakdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"FEAT-001", "FEAT-002"} {
		body := "---\nid: " + id + "\ntitle: x\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"
		if err := os.WriteFile(filepath.Join(dir, ".squad", "items", id+"-x.md"),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("OpenDefault: %v", err)
	}
	defer db.Close()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := repo.Discover(wd)
	if err != nil {
		t.Fatalf("repo.Discover: %v", err)
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo.IDFor: %v", err)
	}
	now := time.Now().Unix()
	for _, id := range []string{"FEAT-001", "FEAT-002"} {
		if _, err := db.Exec(
			`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long, worktree)
			 VALUES (?, ?, 'agent-solo', ?, ?, 'x', 0, '')`,
			repoID, id, now, now,
		); err != nil {
			t.Fatalf("insert claim %s: %v", id, err)
		}
	}

	var out bytes.Buffer
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("status exit=%d", code)
	}
	body := out.String()
	if !strings.Contains(body, "claimed: 2") {
		t.Errorf("expected claimed: 2:\n%s", body)
	}
	if strings.Contains(body, "agent-solo") {
		t.Errorf("single-agent breakdown should not be printed (no contention):\n%s", body)
	}
}
