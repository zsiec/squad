package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubagentEvent_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestSubagentEvent_InvokesSquadVerbWithStdin(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	trace := filepath.Join(dir, "trace.txt")
	body := fmt.Sprintf(`#!/bin/sh
{ printf 'argv:'; for a in "$@"; do printf ' %%s' "$a"; done; printf '\n'; cat; } >> %q
exit 0
`, trace)
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"SubagentStart","agent_id":"sub-1"}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(trace)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "argv: subagent-event") {
		t.Fatalf("did not invoke squad subagent-event:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, `"hook_event_name":"SubagentStart"`) {
		t.Fatalf("stdin not propagated to verb:\n%s", gotStr)
	}
}

func TestSubagentEvent_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "subagent_event.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}

// TestSubagentEvent_AlsoInvokesEventRecord verifies the hook calls both the
// existing chat-post verb (subagent-event) AND the new agent_events recorder
// (event record) for SubagentStart / SubagentStop. AC mandates additive
// recording: chat preserved, event row added.
func TestSubagentEvent_AlsoInvokesEventRecord(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	trace := filepath.Join(dir, "trace.txt")
	body := fmt.Sprintf(`#!/bin/sh
{ printf 'argv:'; for a in "$@"; do printf ' %%s' "$a"; done; printf '\n'; } >> %q
exit 0
`, trace)
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"SubagentStart","agent_id":"sub-1","agent_type":"general-purpose","description":"explore the codebase"}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(trace)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "argv: subagent-event") {
		t.Errorf("subagent-event chat-post missing: %s", gotStr)
	}
	if !strings.Contains(gotStr, "argv: event record --kind subagent_start") {
		t.Errorf("event record call missing or wrong kind: %s", gotStr)
	}
	if !strings.Contains(gotStr, "--tool general-purpose") {
		t.Errorf("event record --tool not set to agent_type: %s", gotStr)
	}
}

// TestSubagentEvent_StopMapsExitCode verifies SubagentStop's exit field flows
// through to --exit on the recorder call so the SPA can render success/fail.
func TestSubagentEvent_StopMapsExitCode(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	trace := filepath.Join(dir, "trace.txt")
	body := fmt.Sprintf(`#!/bin/sh
{ printf 'argv:'; for a in "$@"; do printf ' %%s' "$a"; done; printf '\n'; } >> %q
exit 0
`, trace)
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"SubagentStop","agent_id":"sub-1","agent_type":"general-purpose","exit_code":7}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(trace)
	gotStr := string(got)
	if !strings.Contains(gotStr, "--kind subagent_stop") {
		t.Errorf("kind not subagent_stop: %s", gotStr)
	}
	if !strings.Contains(gotStr, "--exit 7") {
		t.Errorf("exit code not propagated: %s", gotStr)
	}
}

// TestSubagentEvent_TaskEventsSkipRecorder ensures TaskCreated/TaskCompleted
// hook fires the chat-post (subagent-event) but does NOT call event record —
// only subagent_start/stop are in the agent_events kind enum for this ship.
func TestSubagentEvent_TaskEventsSkipRecorder(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	trace := filepath.Join(dir, "trace.txt")
	body := fmt.Sprintf(`#!/bin/sh
{ printf 'argv:'; for a in "$@"; do printf ' %%s' "$a"; done; printf '\n'; } >> %q
exit 0
`, trace)
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"TaskCreated","agent_id":"sub-1"}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(trace)
	gotStr := string(got)
	if !strings.Contains(gotStr, "argv: subagent-event") {
		t.Errorf("subagent-event chat-post missing for TaskCreated: %s", gotStr)
	}
	if strings.Contains(gotStr, "argv: event record") {
		t.Errorf("event record should be skipped for TaskCreated: %s", gotStr)
	}
}
