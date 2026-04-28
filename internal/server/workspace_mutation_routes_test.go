package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

// seedItemInRepo materializes a captured item file in <root>/.squad/items/
// and a matching items table row tagged with repoID. Returns the item id
// (NextID is monotonic so the caller can derive a stable string from
// prefix + count).
func seedItemInRepo(t *testing.T, db *sql.DB, root, repoID, prefix, title string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := items.NewWithOptions(filepath.Join(root, ".squad"), prefix, title, items.Options{Area: "test"}); err != nil {
		t.Fatalf("create item in %s: %v", repoID, err)
	}
	walk, err := items.Walk(filepath.Join(root, ".squad"))
	if err != nil || len(walk.Active) == 0 {
		t.Fatalf("walk %s: err=%v active=%d", root, err, len(walk.Active))
	}
	it := walk.Active[len(walk.Active)-1]
	if err := items.Persist(context.Background(), db, repoID, it, false); err != nil {
		t.Fatalf("persist %s in %s: %v", it.ID, repoID, err)
	}
	return it.ID
}

func wsServer(t *testing.T, db *sql.DB) *Server {
	t.Helper()
	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	return s
}

func postJSONReq(method, path string, body any) *http.Request {
	var r *http.Request
	if body != nil {
		buf, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(buf))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Squad-Agent", "agent-tester")
	return r
}

func TestHandleItemsAutoRefine_WorkspaceMode_FindsItemInResolvedRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs auto refining today")

	s := wsServer(t, db)
	// Inject a no-op runner so we don't shell out to claude.
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath, repoRoot string) autoRefineRunResult {
		return autoRefineRunResult{}
	})

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/auto-refine", map[string]any{}))

	// Without an actual auto_refined_at bump from the runner the handler
	// returns 500 "claude exited without drafting" — but crucially NOT 404.
	// The bug under test is "item not found" in workspace mode; any
	// outcome other than 404 confirms the lookup found the right repo.
	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode auto-refine returned 404 (lookup failed); body=%s", rec.Body.String())
	}
}

func TestHandleItemsAutoRefine_WorkspaceMode_AmbiguousReturns409(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	idA := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "title in repo A here today")
	// Force a colliding id by inserting a second items row with the same
	// item_id under repo-B (real squad numbers monotonically, but the
	// disambiguation behavior must hold even if a future change allows
	// collisions).
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO items (repo_id, item_id, title, type, priority, area, status,
		                   estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES ('repo-B', ?, 'collision', 'bug', 'P2', '', 'open', '', '', 0, 0, 0, '', 0)`,
		idA); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+idA+"/auto-refine", map[string]any{}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("ambiguous lookup should be 409; got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error      string   `json:"error"`
		Message    string   `json:"message"`
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
	want := map[string]bool{"repo-A": true, "repo-B": true}
	got := map[string]bool{}
	for _, c := range body.Candidates {
		got[c] = true
	}
	if len(got) != 2 || !got["repo-A"] || !got["repo-B"] {
		t.Errorf("candidates=%v, want both repo-A and repo-B (%v)", body.Candidates, want)
	}
}

func TestHandleItemsAutoRefine_WorkspaceMode_RepoIDDisambiguates(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	idA := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "title in repo A here today")
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO items (repo_id, item_id, title, type, priority, area, status,
		                   estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES ('repo-B', ?, 'collision', 'bug', 'P2', '', 'open', '', '', 0, 0, 0, '', 0)`,
		idA); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath, repoRoot string) autoRefineRunResult {
		return autoRefineRunResult{}
	})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+idA+"/auto-refine?repo_id=repo-A", map[string]any{}))

	if rec.Code == http.StatusConflict || rec.Code == http.StatusNotFound {
		t.Fatalf("?repo_id=repo-A should resolve unambiguously; got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleItemsRecapture_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs recapture in workspace mode")

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/recapture", map[string]any{}))

	// Status="captured" not "needs-refinement" → ErrWrongStatusForRecapture
	// (422). Bug under test: workspace-mode lookup MUST NOT 404.
	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode recapture returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemsAccept_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs accept in workspace mode here")

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/accept", map[string]any{}))

	// Item body is placeholder; DoR check fails with 422. The bug under
	// test is "item not found" in workspace mode; 404 means lookup failed.
	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode accept returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemsRefine_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs refine in workspace mode here")

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/refine",
		map[string]any{"comments": "AC bullets too vague; tighten and resubmit."}))

	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode refine returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemsReject_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs reject in workspace mode here")

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/reject",
		map[string]any{"reason": "duplicate of older item; closing."}))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("reject in workspace mode should succeed (204); got %d body=%s", rec.Code, rec.Body.String())
	}
	// Verify the rejection log lives in the matched repo's .squad, not in
	// some root-relative .squad/.
	logPath := filepath.Join(repoA, ".squad", "rejected.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("rejected.log not written to repo-A's .squad: %v", err)
	}
}

func TestHandleItemClaim_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs claim in workspace mode here")
	// Promote to open so the claim preflight passes.
	if _, err := db.ExecContext(context.Background(),
		`UPDATE items SET status = 'open' WHERE repo_id = 'repo-A' AND item_id = ?`, id); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/claim",
		map[string]any{"intent": "ship the thing"}))

	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode claim returned 404; lookup is broken: %s", rec.Body.String())
	}
	// Verify the claim landed in the right repo.
	var n int
	_ = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims WHERE repo_id = 'repo-A' AND item_id = ?`, id).Scan(&n)
	if n != 1 {
		t.Errorf("claim should land in repo-A; got %d rows for that scope", n)
	}
}

func TestHandleItemDone_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs done in workspace mode here")
	// Set up: promote to open, claim it as the test agent.
	if _, err := db.ExecContext(context.Background(),
		`UPDATE items SET status = 'open' WHERE repo_id = 'repo-A' AND item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES ('repo-A', ?, 'agent-tester', 100, 100, 'test', 0)`, id); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/done",
		map[string]any{"summary": "shipped it", "evidence_force": true}))

	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode done returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemBlocked_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs blocked in workspace mode here")
	if _, err := db.ExecContext(context.Background(),
		`UPDATE items SET status = 'open' WHERE repo_id = 'repo-A' AND item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES ('repo-A', ?, 'agent-tester', 100, 100, 'test', 0)`, id); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/blocked",
		map[string]any{"reason": "waiting on upstream"}))

	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode blocked returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemForceRelease_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs force release workspace mode")
	if _, err := db.ExecContext(context.Background(),
		`UPDATE items SET status = 'open' WHERE repo_id = 'repo-A' AND item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES ('repo-A', ?, 'agent-other', 100, 100, 'test', 0)`, id); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/force-release",
		map[string]any{"reason": "agent disappeared"}))

	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode force-release returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemRelease_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs release in workspace mode here")
	if _, err := db.ExecContext(context.Background(),
		`UPDATE items SET status = 'open' WHERE repo_id = 'repo-A' AND item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES ('repo-A', ?, 'agent-tester', 100, 100, 'test', 0)`, id); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/release",
		map[string]any{"outcome": "released"}))

	if rec.Code == http.StatusNotFound {
		t.Fatalf("workspace-mode release returned 404; lookup is broken: %s", rec.Body.String())
	}
}

func TestHandleItemsCreate_WorkspaceMode_RequiresRepoID(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	// Drop a config file in repoA so config.Load wouldn't even run if the
	// resolver let us through — the AC requires the resolver to reject
	// before any creation work begins.

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items",
		map[string]any{"type": "bug", "title": "no repo specified"}))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("workspace-mode create without ?repo_id= must 400; got %d body=%s",
			rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "repo_id") {
		t.Errorf("error body should name the missing parameter; got %s", rec.Body.String())
	}
}

func TestHandleItemsCreate_WorkspaceMode_UsesRequestedRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	// Seed a minimal squad config in repoA so config.Load succeeds.
	if err := os.WriteFile(filepath.Join(repoA, ".squad", "config.yaml"),
		[]byte("id_prefixes: [BUG]\ndefaults:\n  priority: P2\n  estimate: 1h\n  risk: low\n  area: workspace-test\n"),
		0o644); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items?repo_id=repo-A",
		map[string]any{"type": "bug", "title": "should land in repo-A's tree"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("workspace-mode create with ?repo_id=repo-A should succeed; got %d body=%s",
			rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	pathStr, _ := resp["path"].(string)
	if !strings.HasPrefix(pathStr, repoA) {
		t.Errorf("created item path %q should be under repo-A root %q", pathStr, repoA)
	}
	// And the items table row should be tagged with repo-A.
	var n int
	_ = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM items WHERE repo_id = 'repo-A' AND item_id = ?`, resp["id"]).Scan(&n)
	if n != 1 {
		t.Errorf("items row should be tagged with repo-A; got %d", n)
	}
}

func TestHandleItemsCreate_WorkspaceMode_TwoReposNoLeakage(t *testing.T) {
	db := newTestDB(t)
	repoA, repoB := seedWorkspaceRepos(t, db)
	cfg := []byte("id_prefixes: [BUG, FEAT]\ndefaults:\n  priority: P2\n  estimate: 1h\n  risk: low\n  area: workspace-test\n")
	for _, root := range []string{repoA, repoB} {
		if err := os.WriteFile(filepath.Join(root, ".squad", "config.yaml"), cfg, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := wsServer(t, db)

	// Create one item per repo via the same workspace-mode handler.
	postType := func(typ, repoID, title string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost,
			"/api/items?repo_id="+repoID,
			map[string]any{"type": typ, "title": title}))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create in %s: code=%d body=%s", repoID, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode create response for %s: %v", repoID, err)
		}
		return resp
	}
	// Use distinct prefixes so the resulting ids diverge — otherwise the
	// per-repo BUG counters would each emit BUG-001 and "leakage" becomes
	// indistinguishable from "two legitimate items happen to share an id".
	respA := postType("bug", "repo-A", "lands in A only")
	respB := postType("feat", "repo-B", "lands in B only")

	pathA, _ := respA["path"].(string)
	pathB, _ := respB["path"].(string)
	if !strings.HasPrefix(pathA, repoA) {
		t.Errorf("item A path %q should be under repo-A root %q", pathA, repoA)
	}
	if !strings.HasPrefix(pathB, repoB) {
		t.Errorf("item B path %q should be under repo-B root %q", pathB, repoB)
	}
	if strings.HasPrefix(pathA, repoB) || strings.HasPrefix(pathB, repoA) {
		t.Errorf("cross-repo leakage: A=%q B=%q", pathA, pathB)
	}

	// Items table rows: the item created in each repo must be tagged with
	// exactly that repo. Item IDs may collide across repos (BUG counter is
	// per-repo), so the lookup must scope by (item_id, repo_id).
	idA, _ := respA["id"].(string)
	idB, _ := respB["id"].(string)
	for _, c := range []struct{ id, repo string }{{idA, "repo-A"}, {idB, "repo-B"}} {
		var n int
		if err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM items WHERE item_id = ? AND repo_id = ?`, c.id, c.repo).Scan(&n); err != nil {
			t.Fatalf("query items for %s/%s: %v", c.id, c.repo, err)
		}
		if n != 1 {
			t.Errorf("expected exactly one items row tagged %s in %s; got %d", c.id, c.repo, n)
		}
	}

	// /api/items in workspace mode aggregates across repos; each new item
	// must surface tagged with its own repo, never with the other repo's
	// tag. Pair on (id, repo_id) since IDs collide across repos.
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/items", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	type pair struct{ id, repo string }
	seen := map[pair]int{}
	for _, r := range rows {
		id, _ := r["id"].(string)
		rid, _ := r["repo_id"].(string)
		seen[pair{id, rid}]++
	}
	if seen[pair{idA, "repo-A"}] != 1 {
		t.Errorf("listed (%s, repo-A) count=%d want 1", idA, seen[pair{idA, "repo-A"}])
	}
	if seen[pair{idB, "repo-B"}] != 1 {
		t.Errorf("listed (%s, repo-B) count=%d want 1", idB, seen[pair{idB, "repo-B"}])
	}
	if seen[pair{idA, "repo-B"}] != 0 {
		t.Errorf("cross-repo leakage: (%s, repo-B) appeared in list", idA)
	}
	if seen[pair{idB, "repo-A"}] != 0 {
		t.Errorf("cross-repo leakage: (%s, repo-A) appeared in list", idB)
	}
}

func TestHandleItemHandoff_WorkspaceMode_TagsMessageWithResolvedRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs handoff in workspace mode")
	if _, err := db.ExecContext(context.Background(),
		`UPDATE items SET status = 'open' WHERE repo_id = 'repo-A' AND item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES ('repo-A', ?, 'agent-tester', 100, 100, 'test', 0)`, id); err != nil {
		t.Fatal(err)
	}
	// agents table needs the receiving agent so reassign succeeds; also
	// register the actor so message foreign-keys (last_tick_at update) hit.
	for _, who := range []struct{ id, repo string }{{"agent-tester", "repo-A"}, {"agent-receiver", "repo-A"}} {
		if _, err := db.ExecContext(context.Background(), `
			INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
			VALUES (?, ?, ?, '/tmp/wt', 1, 0, 0, 'active')
		`, who.id, who.repo, who.id); err != nil {
			t.Fatal(err)
		}
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/handoff",
		map[string]any{"to": "@agent-receiver", "summary": "you take it"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("workspace-mode handoff should succeed; got %d body=%s", rec.Code, rec.Body.String())
	}
	// The handoff message must be tagged with repo-A so per-repo activity
	// feeds render it; pre-fix it would have landed with repo_id=''.
	var n int
	_ = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM messages WHERE repo_id = 'repo-A' AND kind = 'handoff'`).Scan(&n)
	if n != 1 {
		t.Errorf("handoff message should be tagged with repo-A; got %d rows for that scope", n)
	}
	var orphan int
	_ = db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM messages WHERE repo_id = '' AND kind = 'handoff'`).Scan(&orphan)
	if orphan != 0 {
		t.Errorf("handoff message must not land with empty repo_id in workspace mode; got %d orphans", orphan)
	}
}

func TestHandleItemTouch_WorkspaceMode_ResolvesRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "needs touch in workspace mode here")
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES ('repo-A', ?, 'agent-tester', 100, 100, 'test', 0)`, id); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/items/"+id+"/touch", map[string]any{}))

	if rec.Code != http.StatusOK {
		t.Fatalf("workspace-mode touch should succeed (claim exists in repo-A); got %d body=%s",
			rec.Code, rec.Body.String())
	}
}

func TestHandleMessagesPost_WorkspaceMode_TagsItemThreadWithResolvedRepo(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	id := seedItemInRepo(t, db, repoA, "repo-A", "BUG", "thread message in workspace mode here")
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('agent-tester', 'repo-A', 'tester', '/tmp/wt', 1, 0, 0, 'active')
	`); err != nil {
		t.Fatal(err)
	}

	s := wsServer(t, db)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, postJSONReq(http.MethodPost, "/api/messages",
		map[string]any{"thread": id, "body": "ack from SPA", "kind": "say"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("workspace-mode messages POST should succeed; got %d body=%s", rec.Code, rec.Body.String())
	}
	var rid string
	if err := db.QueryRow(`SELECT repo_id FROM messages WHERE thread = ?`, id).Scan(&rid); err != nil {
		t.Fatalf("locate posted message: %v", err)
	}
	if rid != "repo-A" {
		t.Errorf("posted message repo_id=%q, want repo-A (resolved from item thread)", rid)
	}
}
