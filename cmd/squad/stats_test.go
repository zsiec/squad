package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/stats"
)

// safeBuffer guards bytes.Buffer for the --tail test, where the cobra
// goroutine writes while the polling main goroutine reads. bytes.Buffer is
// not safe for concurrent use; without this go test -race flags it.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out
}

func (b *safeBuffer) String() string {
	return string(b.Bytes())
}

func setupSquadRepoForStats(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-stats-"+t.Name())
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return repoDir
}

func runStatsRoot(ctx context.Context, args []string, w io.Writer) error {
	root := newRootCmd()
	root.SetOut(w)
	root.SetErr(w)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func waitForCondition(t *testing.T, fn func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestStatsJSONOutputShape(t *testing.T) {
	_ = setupSquadRepoForStats(t)
	var out bytes.Buffer
	if err := runStatsRoot(context.Background(),
		[]string{"stats", "--json", "--window", "24h"}, &out); err != nil {
		t.Fatalf("stats --json: %v\nout=%s", err, out.String())
	}
	var snap stats.Snapshot
	if err := json.Unmarshal(out.Bytes(), &snap); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if snap.SchemaVersion != stats.CurrentSchemaVersion {
		t.Errorf("schema_version: %d", snap.SchemaVersion)
	}
	if !strings.HasPrefix(snap.Window.Label, "24h") {
		t.Errorf("window label: %q", snap.Window.Label)
	}
}

func TestStatsTailEmitsNDJSON(t *testing.T) {
	_ = setupSquadRepoForStats(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var buf safeBuffer
	done := make(chan error, 1)
	go func() {
		done <- runStatsRoot(ctx,
			[]string{"stats", "--tail", "--interval", "50ms"}, &buf)
	}()
	waitForCondition(t, func() bool {
		return bytes.Count(buf.Bytes(), []byte{'\n'}) >= 2
	}, "two NDJSON lines")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stats --tail did not exit after cancel")
	}
	for _, line := range bytes.Split(bytes.TrimRight(buf.Bytes(), "\n"), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var s stats.Snapshot
		if err := json.Unmarshal(line, &s); err != nil {
			t.Fatalf("ndjson invalid: %v\n%s", err, line)
		}
	}
}
