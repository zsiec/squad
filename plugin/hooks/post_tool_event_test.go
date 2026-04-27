package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostToolEvent_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "post_tool_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":{"exit_code":0}}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestPostToolEvent_SilentWhenSquadMissing(t *testing.T) {
	p := writeFixtureScript(t, "post_tool_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH="}
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash"}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestPostToolEvent_FilterReadSkipsRead(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	stub := filepath.Join(dir, "squad")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	p := writeFixtureScript(t, "post_tool_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = append(os.Environ(),
		"SQUAD_EVENTS_FILTER_READ=1",
		"PATH="+dir+":"+os.Getenv("PATH"),
	)
	cmd.Stdin = strings.NewReader(`{"tool_name":"Read","tool_input":{"file_path":"/tmp/x"}}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook should exit 0; got %v\n%s", err, out)
	}
	if data, _ := os.ReadFile(logPath); len(data) > 0 {
		t.Fatalf("expected no squad call when filtering Read; got: %s", data)
	}
}

func TestPostToolEvent_RecordsExitAndDuration(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	stub := filepath.Join(dir, "squad")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	p := writeFixtureScript(t, "post_tool_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":{"exit_code":7,"duration_ms":123}}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	for _, want := range []string{"event record", "--kind post_tool", "--tool Bash", "--exit 7", "--duration-ms 123"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in squad call; got: %s", want, got)
		}
	}
}

func TestPostToolEvent_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "post_tool_event.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}

func TestPostToolEvent_FallsBackWithoutJQ(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	stub := filepath.Join(dir, "squad")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, bin := range []string{"sed", "head", "cut", "tr", "printf", "sh", "cat"} {
		realPath, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		_ = os.Symlink(realPath, filepath.Join(dir, bin))
	}

	p := writeFixtureScript(t, "post_tool_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=" + dir}
	cmd.Stdin = strings.NewReader(`{"tool_name":"Edit","tool_input":{"file_path":"/tmp/x.go"},"tool_response":{"exit_code":3,"duration_ms":99}}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	for _, want := range []string{"--kind post_tool", "--tool Edit", "/tmp/x.go", "--exit 3", "--duration-ms 99"} {
		if !strings.Contains(got, want) {
			t.Errorf("fallback path: expected %q in squad call; got: %s", want, got)
		}
	}
}
