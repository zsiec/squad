package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEpics_List(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/epics", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 1 {
		t.Fatalf("want 1 epic, got %d: %v", len(out), out)
	}
	if out[0]["name"] != "sample-epic" || out[0]["spec"] != "sample-spec" {
		t.Fatalf("got %v", out[0])
	}
}

func TestEpics_List_FilterBySpec(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/epics?spec=other-spec", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 0 {
		t.Fatalf("want 0 epics for other-spec, got %d", len(out))
	}
}
