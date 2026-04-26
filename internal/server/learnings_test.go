package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLearnings_List(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	req := httptest.NewRequest(http.MethodGet, "/api/learnings", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 1 {
		t.Fatalf("want 1 learning, got %d: %v", len(out), out)
	}
	if out[0]["slug"] != "sample-gotcha" {
		t.Fatalf("slug=%v", out[0]["slug"])
	}
}

func TestLearnings_List_FilterByState(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	req := httptest.NewRequest(http.MethodGet, "/api/learnings?state=proposed", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 0 {
		t.Fatalf("want 0 proposed, got %d", len(out))
	}
}
