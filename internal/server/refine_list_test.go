package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func insertItemRow(t *testing.T, s *Server, id, status string, updatedAt int64, capturedBy string) {
	t.Helper()
	path := filepath.Join(s.cfg.SquadDir, "items", id+"-x.md")
	if _, err := s.db.ExecContext(context.Background(), `
		INSERT INTO items (repo_id, item_id, title, status, captured_by, captured_at, updated_at, path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.cfg.RepoID, id, id+" title", status, capturedBy, updatedAt, updatedAt, path); err != nil {
		t.Fatalf("insert item row: %v", err)
	}
}

// TestGetRefine_WorkspaceModeAggregatesRepos covers BUG-048. handleRefineList
// currently passes cfg.RepoID directly into the SQL filter without the
// workspace-mode "" sentinel branch every other read route uses. A daemon
// running in workspace mode (cfg.RepoID == "") returns an empty array even
// when items in the global DB have status='needs-refinement'. Mirror the
// inbox handler shape: widen the query and tag each row with its repo id.
func TestGetRefine_WorkspaceModeAggregatesRepos(t *testing.T) {
	db := newTestDB(t)
	s := New(db, "", Config{RepoID: "", SquadDir: ""})
	t.Cleanup(s.Close)

	for _, r := range []struct {
		repoID, itemID string
		ts             int64
	}{
		{"repo-A", "BUG-100", 100},
		{"repo-B", "BUG-200", 200},
	} {
		if _, err := db.Exec(`
			INSERT INTO items (repo_id, item_id, title, status, captured_by, captured_at, updated_at, path)
			VALUES (?, ?, ?, 'needs-refinement', 'agent-x', ?, ?, '/tmp/'||?||'.md')
		`, r.repoID, r.itemID, r.itemID+" title", r.ts, r.ts, r.itemID); err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/refine", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 rows from both repos, got %d: %v", len(out), out)
	}
	repoIDs := map[string]string{}
	for _, row := range out {
		repoIDs[row["id"].(string)] = row["repo_id"].(string)
	}
	if repoIDs["BUG-100"] != "repo-A" {
		t.Errorf("BUG-100 repo_id=%q want repo-A", repoIDs["BUG-100"])
	}
	if repoIDs["BUG-200"] != "repo-B" {
		t.Errorf("BUG-200 repo_id=%q want repo-B", repoIDs["BUG-200"])
	}
}

func TestGetRefine_ListsAndSorts(t *testing.T) {
	s, _ := newCreateServer(t)
	insertItemRow(t, s, "BUG-920", "needs-refinement", 200, "agent-a")
	insertItemRow(t, s, "BUG-921", "needs-refinement", 100, "agent-b")
	insertItemRow(t, s, "BUG-922", "captured", 50, "agent-c")

	req := httptest.NewRequest(http.MethodGet, "/api/refine", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2: %v", len(out), out)
	}
	if out[0]["id"] != "BUG-921" || out[1]["id"] != "BUG-920" {
		t.Fatalf("order wrong: %v", out)
	}
	if int64(out[0]["refined_at"].(float64)) != 100 {
		t.Fatalf("refined_at=%v want 100", out[0]["refined_at"])
	}
	if out[0]["captured_by"] != "agent-b" {
		t.Fatalf("captured_by=%v want agent-b", out[0]["captured_by"])
	}
}
