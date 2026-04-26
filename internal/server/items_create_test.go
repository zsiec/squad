package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newCreateServer(t *testing.T) (*Server, string) {
	t.Helper()
	db := newTestDB(t)
	tmp := t.TempDir()
	squadDir := filepath.Join(tmp, ".squad")
	s := New(db, testRepoID, Config{
		RepoID:        testRepoID,
		SquadDir:      squadDir,
		LearningsRoot: tmp,
	})
	t.Cleanup(s.Close)
	return s, tmp
}

func postJSON(t *testing.T, s *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestItemsCreate_HappyPath(t *testing.T) {
	s, _ := newCreateServer(t)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"type":  "feat",
		"title": "add a brand new dashboard widget",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id, _ := out["id"].(string); !strings.HasPrefix(id, "FEAT-") {
		t.Fatalf("id=%v want FEAT- prefix", out["id"])
	}
	if out["status"] != "captured" {
		t.Fatalf("status=%v want captured", out["status"])
	}
	if p, _ := out["path"].(string); p == "" {
		t.Fatalf("path missing: %v", out["path"])
	}

	// Persisted to db with captured_by defaulting to "web".
	var capturedBy string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(captured_by,'') FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, out["id"],
	).Scan(&capturedBy); err != nil {
		t.Fatalf("query db: %v", err)
	}
	if capturedBy != "web" {
		t.Fatalf("captured_by=%q want web", capturedBy)
	}
}

func TestItemsCreate_ReadyTrueOpensImmediately(t *testing.T) {
	s, _ := newCreateServer(t)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"type":  "bug",
		"title": "scrollbar disappears on resize in chrome",
		"ready": true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["status"] != "open" {
		t.Fatalf("status=%v want open", out["status"])
	}
}

func TestItemsCreate_CapturedByOverride(t *testing.T) {
	s, _ := newCreateServer(t)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"type":        "task",
		"title":       "wire up the new metric collector",
		"captured_by": "thomas@laptop",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	var capturedBy string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(captured_by,'') FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, out["id"],
	).Scan(&capturedBy); err != nil {
		t.Fatalf("query db: %v", err)
	}
	if capturedBy != "thomas@laptop" {
		t.Fatalf("captured_by=%q want thomas@laptop", capturedBy)
	}
}

func TestItemsCreate_InvalidTypeReturns400(t *testing.T) {
	s, _ := newCreateServer(t)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"type":  "epic",
		"title": "this prefix is not in id_prefixes by default",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsCreate_MissingTypeReturns400(t *testing.T) {
	s, _ := newCreateServer(t)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"title": "title with no type field",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsCreate_MissingTitleReturns400(t *testing.T) {
	s, _ := newCreateServer(t)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"type": "feat",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsCreate_BodyParseErrorReturns400(t *testing.T) {
	s, _ := newCreateServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/items",
		bytes.NewReader([]byte(`{not valid json`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
