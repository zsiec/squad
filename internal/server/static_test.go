package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatic_ServesIndexAtRoot(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{})
	defer s.Close()
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
	defer s.Close()
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
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("POST /api/health should not 200; got %d", rec.Code)
	}
}

func TestInsightsAssetServed(t *testing.T) {
	s := New(newTestDB(t), "repo-1", Config{})
	defer s.Close()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/insights.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("renderInsights")) {
		t.Errorf("renderInsights symbol missing")
	}
}

func minN(a, b int) int {
	if a < b {
		return a
	}
	return b
}
