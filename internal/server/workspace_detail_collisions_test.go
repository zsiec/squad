package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// seedItemRowOnly inserts just the items DB row (no markdown file). Used for
// the cross-repo collision fixtures where a second repo claims the same
// item id — squad's monotonic numbering doesn't allow real collisions, so
// the second row is a synthetic test of the disambiguation path.
func seedItemRowOnly(t *testing.T, db *sql.DB, repoID, itemID, title string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO items (repo_id, item_id, title, type, priority, area, status,
		                   estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES (?, ?, ?, 'bug', 'P2', '', 'open', '', '', 0, 0, 0, '', 0)`,
		repoID, itemID, title); err != nil {
		t.Fatal(err)
	}
}

// detailRoutes is the closed list of read-detail routes BUG-050 wires for
// cross-repo disambiguation. Each route under test must:
//   - return 409 AmbiguousRepoError when the id collides and no ?repo_id= is given
//   - resolve unambiguously when ?repo_id=<one> is given
func detailRoutes(itemID string) []struct {
	name string
	path string
} {
	return []struct{ name, path string }{
		{"detail", "/api/items/" + itemID},
		{"links", "/api/items/" + itemID + "/links"},
		{"activity", "/api/items/" + itemID + "/activity"},
		{"attestations", "/api/items/" + itemID + "/attestations"},
	}
}

func TestDetailRoutes_WorkspaceMode_AmbiguousReturns409(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	idA := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "title in repo A here today")
	seedItemRowOnly(t, db, "repo-B", idA, "collision in repo B")

	s := wsServer(t, db)
	for _, route := range detailRoutes(idA) {
		t.Run(route.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route.path, nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("ambiguous lookup should be 409; got %d body=%s", rec.Code, rec.Body.String())
			}
			var body struct {
				Error      string   `json:"error"`
				ItemID     string   `json:"item_id"`
				Candidates []string `json:"candidates"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("409 body should parse as structured JSON: %v\n%s", err, rec.Body.String())
			}
			if body.Error != "ambiguous" {
				t.Errorf("error field=%q want \"ambiguous\"", body.Error)
			}
			if body.ItemID != idA {
				t.Errorf("item_id=%q want %q", body.ItemID, idA)
			}
			got := map[string]bool{}
			for _, c := range body.Candidates {
				got[c] = true
			}
			if !got["repo-A"] || !got["repo-B"] {
				t.Errorf("candidates=%v, want both repo-A and repo-B", body.Candidates)
			}
		})
	}
}

func TestDetailRoutes_WorkspaceMode_RepoIDDisambiguates(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	idA := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "title in repo A here today")
	seedItemRowOnly(t, db, "repo-B", idA, "collision in repo B")

	s := wsServer(t, db)
	for _, route := range detailRoutes(idA) {
		t.Run(route.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route.path+"?repo_id=repo-A", nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			if rec.Code == http.StatusConflict || rec.Code == http.StatusNotFound {
				t.Fatalf("?repo_id=repo-A should resolve unambiguously; got %d body=%s",
					rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDetailRoutes_SingleRepoMode_NoQueryParamRequired(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	idA := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "title in repo A here today")
	// Single-repo mode: cfg.RepoID is non-empty.
	s := New(db, "repo-A", Config{RepoID: "repo-A", SquadDir: repoA + "/.squad", LearningsRoot: repoA})
	t.Cleanup(s.Close)

	for _, route := range detailRoutes(idA) {
		t.Run(route.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route.path, nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("single-repo mode should resolve without ?repo_id=; got %d body=%s",
					rec.Code, rec.Body.String())
			}
		})
	}
}

// TestHandleItemDetail_WorkspaceMode_DisambiguatedDetailReflectsRepo pins
// that the response body's repo_id matches the requested repo, not just
// "any" repo's row. Without this, a future bug where the handler resolves
// repo correctly but renders a different repo's body would silently pass.
func TestHandleItemDetail_WorkspaceMode_DisambiguatedDetailReflectsRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, repoB := seedWorkspaceRepos(t, db)
	idA := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "title in repo A here today")
	// Real second-repo item with a different title so we can tell them apart.
	idB := seedItemInRepo(t, db, repoB, "repo-B", "BUG", "title in repo B different here")
	if idA != idB {
		// Squad's monotonic numbering starts fresh per repo, so both should
		// be BUG-001 — but defend against future schema drift.
		t.Fatalf("expected colliding ids across repos for the test premise; got %s vs %s", idA, idB)
	}

	s := wsServer(t, db)
	req := httptest.NewRequest(http.MethodGet, "/api/items/"+idA+"?repo_id=repo-B", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("scoped detail should be 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
	if body["repo_id"] != "repo-B" {
		t.Errorf("repo_id=%v want repo-B", body["repo_id"])
	}
	if title, _ := body["title"].(string); title != "title in repo B different here" {
		t.Errorf("title=%q want repo-B's title", title)
	}
}
