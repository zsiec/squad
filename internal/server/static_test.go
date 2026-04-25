package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatic_ServesIndexAtRoot(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type=%q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>Squad</title>") {
		t.Fatalf("missing scrubbed title; first 200: %s", body[:minN(200, len(body))])
	}
	if strings.Contains(strings.ToLower(body), "switchframe") {
		t.Fatal("body still contains 'switchframe' — scrub incomplete")
	}
}

func TestStatic_ServesAppJS(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{})
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	got := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(got, "application/javascript") && !strings.HasPrefix(got, "text/javascript") {
		t.Fatalf("content-type=%q", got)
	}
}

func TestStatic_APIPathFallthroughIs405NotHTML(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{})
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("POST /api/health should not 200; got %d", rec.Code)
	}
}

func minN(a, b int) int {
	if a < b {
		return a
	}
	return b
}
