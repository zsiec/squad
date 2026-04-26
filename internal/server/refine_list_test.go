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
