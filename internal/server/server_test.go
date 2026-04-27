package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_Health(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}
}
