package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

// TestHandleItemsList_WorkspaceModeAggregatesRepos covers BUG-040: when
// cfg.RepoID is empty (the daemon-spawned-by-launchd case), /api/items
// must enumerate repos via ~/.squad/global.db and aggregate items across
// all of them. Each item in the response carries its repo_id so the SPA
// can disambiguate.
func TestHandleItemsList_WorkspaceModeAggregatesRepos(t *testing.T) {
	db := newTestDB(t)
	tmp := t.TempDir()
	repoARoot := filepath.Join(tmp, "repoA")
	repoBRoot := filepath.Join(tmp, "repoB")
	for _, root := range []string{repoARoot, repoBRoot} {
		if err := os.MkdirAll(filepath.Join(root, ".squad", "items"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := items.NewWithOptions(filepath.Join(root, ".squad"), "BUG", "alpha bravo charlie delta echo foxtrot", items.Options{Area: "test"}); err != nil {
			t.Fatalf("create item: %v", err)
		}
	}
	insertRepo(t, db, "repo-A", repoARoot, "")
	insertRepo(t, db, "repo-B", repoBRoot, "")

	// Workspace mode: cfg.RepoID == "" and SquadDir is empty/sentinel.
	s := New(db, "", Config{RepoID: "", SquadDir: ""})
	t.Cleanup(s.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, rec.Body.String())
	}
	if len(rows) < 2 {
		t.Fatalf("expected items from both repos, got %d: %v", len(rows), rows)
	}
	repoIDs := map[string]int{}
	for _, r := range rows {
		if rid, ok := r["repo_id"].(string); ok && rid != "" {
			repoIDs[rid]++
		}
	}
	if repoIDs["repo-A"] == 0 || repoIDs["repo-B"] == 0 {
		t.Errorf("expected items tagged with both repo-A and repo-B; got %v", repoIDs)
	}
}
