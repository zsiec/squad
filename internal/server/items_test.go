package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestItems_List(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 1 || out[0]["id"] != "BUG-100" {
		t.Fatalf("got %v", out)
	}
	if out[0]["progress_pct"].(float64) != 50 {
		t.Fatalf("progress=%v", out[0]["progress_pct"])
	}
}

func TestItems_Detail(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-100", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["title"] != "example bug for server tests" {
		t.Fatalf("title=%v", out["title"])
	}
	if _, ok := out["body_markdown"]; !ok {
		t.Fatal("expected body_markdown")
	}
}

func TestItems_Detail_404OnMissing(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-NOPE", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}
