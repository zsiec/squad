package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRepos_List(t *testing.T) {
	db := newTestDB(t)
	insertRepo(t, db, "repo-abc", "/Users/me/dev/foo", "git@github.com:me/foo.git")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 1 || out[0]["repo_id"] != "repo-abc" {
		t.Fatalf("got %v", out)
	}
}

func TestWorkspace_Status(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/workspace/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if _, ok := out["repos"]; !ok {
		t.Fatal("expected 'repos' field")
	}
}
