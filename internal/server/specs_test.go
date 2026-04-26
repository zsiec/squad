package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpecs_List(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/specs", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 spec, got %d: %v", len(out), out)
	}
	if out[0]["name"] != "sample-spec" {
		t.Fatalf("name=%v", out[0]["name"])
	}
	if out[0]["title"] != "Sample spec for server tests" {
		t.Fatalf("title=%v", out[0]["title"])
	}
}

func TestSpecs_Detail(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/specs/sample-spec", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["title"] != "Sample spec for server tests" {
		t.Fatalf("title=%v", out["title"])
	}
	if _, ok := out["body_markdown"]; !ok {
		t.Fatal("expected body_markdown")
	}
	if _, ok := out["acceptance"]; !ok {
		t.Fatal("expected acceptance array")
	}
}

func TestSpecs_Detail_404(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/specs/no-such-spec", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}
