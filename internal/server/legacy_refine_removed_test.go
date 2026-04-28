package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLegacyPeerQueueRefineRouteRemoved pins that the peer-queue refine
// HTTP surface no longer exists. The comment-driven auto-refine flow
// (`/api/items/:id/auto-refine`) is the only refinement entry point.
func TestLegacyPeerQueueRefineRouteRemoved(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/items/BUG-100/refine"},
		{http.MethodGet, "/api/refine"},
		{http.MethodPost, "/api/items/BUG-100/recapture"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, bytes.NewReader([]byte(`{}`)))
			req.Header.Set("X-Squad-Agent", "agent-aaaa")
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s: expected 404 (route removed), got %d body=%s",
					c.method, c.path, rec.Code, rec.Body.String())
			}
		})
	}
}
