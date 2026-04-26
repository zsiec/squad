package server

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/learning"
)

func TestSSE_AttestationRecorded(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status=%d", resp.StatusCode)
	}

	time.Sleep(100 * time.Millisecond)

	ledger := attest.New(db, testRepoID, nil).WithBus(s.Bus())
	if _, err := ledger.Insert(ctx, attest.Record{
		ItemID: "BUG-100", Kind: attest.KindTest, Command: "go test",
		ExitCode: 0, OutputHash: "h1", OutputPath: "p1", AgentID: "agent-test",
	}); err != nil {
		t.Fatal(err)
	}

	if !waitSSEEvent(t, resp.Body, "attestation_recorded", 2*time.Second) {
		t.Fatal("did not see attestation_recorded event in SSE stream")
	}
}

func TestSSE_LearningStateChanged(t *testing.T) {
	db := newTestDB(t)
	tmpRoot := t.TempDir()
	srcRoot := "testdata/repo-root"
	if err := copyTree(srcRoot, tmpRoot); err != nil {
		t.Fatal(err)
	}

	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: tmpRoot})
	defer s.Close()

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status=%d", resp.StatusCode)
	}

	time.Sleep(100 * time.Millisecond)

	l, err := learning.ResolveSingle(tmpRoot, "sample-gotcha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := learning.Promote(l, learning.StateRejected, s.Bus()); err != nil {
		t.Fatal(err)
	}

	if !waitSSEEvent(t, resp.Body, "learning_state_changed", 2*time.Second) {
		t.Fatal("did not see learning_state_changed event in SSE stream")
	}
}

func waitSSEEvent(t *testing.T, body io.Reader, kind string, d time.Duration) bool {
	t.Helper()
	scanner := bufio.NewScanner(body)
	deadline := time.Now().Add(d)
	for scanner.Scan() {
		if time.Now().After(deadline) {
			return false
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "event: "+kind) {
			return true
		}
	}
	return false
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
