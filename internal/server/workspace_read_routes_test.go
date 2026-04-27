package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/items"
)

// seedWorkspaceRepos materializes two repo roots on disk and registers
// them in the global DB. Returns the (root, repoID) tuples in
// caller-named order.
func seedWorkspaceRepos(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	repoA := filepath.Join(tmp, "repoA")
	repoB := filepath.Join(tmp, "repoB")
	for _, root := range []string{repoA, repoB} {
		if err := os.MkdirAll(filepath.Join(root, ".squad", "items"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	insertRepo(t, db, "repo-A", repoA, "")
	insertRepo(t, db, "repo-B", repoB, "")
	return repoA, repoB
}

func insertMessageRepo(t *testing.T, db *sql.DB, repoID, thread, agent, kind, body string, ts int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO messages (ts, agent_id, thread, kind, body, mentions, priority, repo_id)
		VALUES (?, ?, ?, ?, ?, '[]', '', ?)
	`, ts, agent, thread, kind, body, repoID); err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func insertAgentEventRepo(t *testing.T, db *sql.DB, repoID, agentID, kind, tool, target string, ts int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agent_events (ts, repo_id, agent_id, event_kind, tool, target, exit_code, duration_ms, session_id)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, '')
	`, ts, repoID, agentID, kind, tool, target); err != nil {
		t.Fatalf("insert agent_event: %v", err)
	}
}

func insertAttestationDirect(t *testing.T, db *sql.DB, repoID, itemID, kind string, exitCode int, hash string, createdAt int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO attestations (item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, repo_id)
		VALUES (?, ?, '', ?, ?, '', ?, 'agent-x', ?)
	`, itemID, kind, exitCode, hash, createdAt, repoID); err != nil {
		t.Fatalf("insert attestation: %v", err)
	}
}

func TestHandleMessagesList_WorkspaceModeAggregatesAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	insertMessageRepo(t, db, "repo-A", "global", "agent-a", "say", "hello from A", 100)
	insertMessageRepo(t, db, "repo-B", "global", "agent-b", "say", "hello from B", 200)

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/messages", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, rec.Body.String())
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 messages aggregated across repos, got %d: %v", len(rows), rows)
	}
	repos := map[string]bool{}
	for _, r := range rows {
		if rid, ok := r["repo_id"].(string); ok {
			repos[rid] = true
		}
	}
	if !repos["repo-A"] || !repos["repo-B"] {
		t.Errorf("expected repo_id field tagged with repo-A and repo-B; got %v", repos)
	}
}

func TestHandleMessagesList_SingleRepoModeStillScopes(t *testing.T) {
	db := newTestDB(t)
	insertMessageRepo(t, db, testRepoID, "global", "agent-a", "say", "in scope", 100)
	insertMessageRepo(t, db, "other-repo", "global", "agent-b", "say", "out of scope", 200)

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/messages", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 1 {
		t.Fatalf("single-repo mode must scope to RepoID; got %d rows: %v", len(rows), rows)
	}
}

func TestHandleItemActivity_WorkspaceModeReturnsThreadAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	insertMessageRepo(t, db, "repo-A", "BUG-700", "agent-a", "say", "from A", 100)
	insertMessageRepo(t, db, "repo-B", "BUG-700", "agent-b", "say", "from B", 200)

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-700/activity", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 2 {
		t.Fatalf("activity should aggregate the thread across repos, got %d rows: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r["repo_id"] == nil || r["repo_id"] == "" {
			t.Errorf("row missing repo_id: %v", r)
		}
	}
}

func TestHandleSearch_WorkspaceModeFindsMessagesAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	insertMessageRepo(t, db, "repo-A", "global", "agent-a", "say", "needle hidden inside", 100)
	insertMessageRepo(t, db, "repo-B", "global", "agent-b", "say", "another needle here", 200)

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=needle", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var hits []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &hits)
	msgRepos := map[string]bool{}
	for _, h := range hits {
		if h["kind"] != "message" {
			continue
		}
		rid, _ := h["repo_id"].(string)
		if rid == "" {
			t.Errorf("message hit missing repo_id: %v", h)
		}
		msgRepos[rid] = true
	}
	if !msgRepos["repo-A"] || !msgRepos["repo-B"] {
		t.Fatalf("expected message hits tagged with both repo-A and repo-B; got %v (hits=%v)", msgRepos, hits)
	}
}

func TestHandleAttestationsForItem_WorkspaceModeReturnsAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	insertAttestationDirect(t, db, "repo-A", "BUG-800", "test", 0, "aaa1", 100)
	insertAttestationDirect(t, db, "repo-B", "BUG-800", "review", 0, "bbb2", 200)

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-800/attestations", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 2 {
		t.Fatalf("attestations should aggregate across repos, got %d: %v", len(rows), rows)
	}
}

func TestHandleStats_WorkspaceModeAggregatesAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	for _, repo := range []string{"repo-A", "repo-B"} {
		if _, err := db.ExecContext(context.Background(), `
			INSERT INTO items (repo_id, item_id, title, type, priority, area, status,
			                   estimate, risk, ac_total, ac_checked, archived, path, updated_at)
			VALUES (?, ?, 't', 'feat', 'P2', '', 'open', '', '', 0, 0, 0, '', ?)`,
			repo, repo+"-001", time.Now().Unix()); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var snap map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	itemsBlock, _ := snap["items"].(map[string]any)
	total, _ := itemsBlock["total"].(float64)
	if total != 2 {
		t.Fatalf("workspace-mode stats.items.total=%v, want 2 (cross-repo): %v", total, snap)
	}
}

func TestHandleAgentEvents_WorkspaceModeIncludesRepoID(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	// agents table requires unique id; register the same id under each repo
	// would violate PK — so register the agent once and let the query find
	// its events across all repos.
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('agent-x', 'repo-A', 'X', '/tmp/wt', 1, 0, 0, 'active')
	`); err != nil {
		t.Fatal(err)
	}
	insertAgentEventRepo(t, db, "repo-A", "agent-x", "tool_use", "Read", "/x.go", 100)
	insertAgentEventRepo(t, db, "repo-B", "agent-x", "tool_use", "Bash", "ls", 200)

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-x/events", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	events, _ := resp["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("expected 2 events across repos, got %d: %v", len(events), resp)
	}
	repos := map[string]bool{}
	for _, ev := range events {
		m := ev.(map[string]any)
		if rid, ok := m["repo_id"].(string); ok {
			repos[rid] = true
		}
	}
	if !repos["repo-A"] || !repos["repo-B"] {
		t.Errorf("expected events tagged repo-A and repo-B; got %v", repos)
	}
}

func TestHandleAgentTimeline_WorkspaceModeAggregates(t *testing.T) {
	db := newTestDB(t)
	seedWorkspaceRepos(t, db)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('agent-y', 'repo-A', 'Y', '/tmp/wt', 1, 0, 0, 'active')
	`); err != nil {
		t.Fatal(err)
	}
	insertMessageRepo(t, db, "repo-A", "global", "agent-y", "say", "from A", 100)
	insertMessageRepo(t, db, "repo-B", "global", "agent-y", "say", "from B", 200)
	// Seed an attestation in a third source so a regression that drops
	// repo_id from a single per-source SELECT is caught even if chat works.
	insertAttestationDirect(t, db, "repo-B", "BUG-900", "test", 0, "cccc", 300)
	if _, err := db.ExecContext(context.Background(), `
		UPDATE attestations SET agent_id = 'agent-y' WHERE output_hash = 'cccc'
	`); err != nil {
		t.Fatal(err)
	}

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-y/timeline", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rows, _ := resp["timeline"].([]any)
	if len(rows) < 3 {
		t.Fatalf("expected timeline rows from chat + attestation across repos, got %d: %v", len(rows), resp)
	}
	repos := map[string]bool{}
	sources := map[string]bool{}
	for _, raw := range rows {
		m := raw.(map[string]any)
		if rid, ok := m["repo_id"].(string); ok && rid != "" {
			repos[rid] = true
		} else {
			t.Errorf("timeline row missing repo_id: %v", m)
		}
		if src, ok := m["source"].(string); ok {
			sources[src] = true
		}
	}
	if !repos["repo-A"] || !repos["repo-B"] {
		t.Errorf("timeline rows should be tagged with both repo-A and repo-B; got %v", repos)
	}
	if !sources["chat"] || !sources["attestation"] {
		t.Errorf("timeline should include both chat and attestation sources; got %v", sources)
	}
}

// seedSpecsAndEpicsInRepo creates a spec markdown and an epic markdown
// with the given names under <root>/.squad/specs/ and /epics/ for the
// epics+specs aggregation tests.
func seedSpecAndEpic(t *testing.T, root, specName, epicName string) {
	t.Helper()
	dir := filepath.Join(root, ".squad")
	if err := os.MkdirAll(filepath.Join(dir, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "epics"), 0o755); err != nil {
		t.Fatal(err)
	}
	specBody := "---\ntitle: " + specName + "\n---\n\n# Spec: " + specName + "\n"
	if err := os.WriteFile(filepath.Join(dir, "specs", specName+".md"), []byte(specBody), 0o644); err != nil {
		t.Fatal(err)
	}
	epicBody := "---\nspec: " + specName + "\nstatus: open\nparallelism: serial\n---\n\n# Epic: " + epicName + "\n"
	if err := os.WriteFile(filepath.Join(dir, "epics", epicName+".md"), []byte(epicBody), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandleSpecsList_WorkspaceModeAggregatesAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	repoA, repoB := seedWorkspaceRepos(t, db)
	seedSpecAndEpic(t, repoA, "auth", "auth-mvp")
	seedSpecAndEpic(t, repoB, "billing", "billing-mvp")

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/specs", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	names := map[string]string{}
	for _, r := range rows {
		n, _ := r["name"].(string)
		rid, _ := r["repo_id"].(string)
		names[n] = rid
	}
	if names["auth"] != "repo-A" || names["billing"] != "repo-B" {
		t.Fatalf("workspace-mode specs should aggregate cross-repo with repo_id tag; got %v", names)
	}
}

func TestHandleSpecDetail_RepoIDDisambiguates(t *testing.T) {
	db := newTestDB(t)
	repoA, repoB := seedWorkspaceRepos(t, db)
	// Same spec name in both repos.
	seedSpecAndEpic(t, repoA, "auth", "auth-A")
	seedSpecAndEpic(t, repoB, "auth", "auth-B")

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)

	// Without ?repo_id=, returns the first match deterministically
	// (repos enumerated ORDER BY id, so the lexicographically smaller
	// id wins). Response carries repo_id so the SPA can render a
	// disambiguating link to the other repo's spec.
	first := map[string]any{}
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/specs/auth", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var got map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if i == 0 {
			first = got
			if first["repo_id"] != "repo-A" {
				t.Errorf("first match should be repo-A (smallest id under ORDER BY id); got repo_id=%v", first["repo_id"])
			}
		} else if got["repo_id"] != first["repo_id"] {
			t.Errorf("call %d returned repo_id=%v, want stable %v across calls", i, got["repo_id"], first["repo_id"])
		}
	}

	// With ?repo_id=repo-B, returns specifically that repo's spec.
	req := httptest.NewRequest(http.MethodGet, "/api/specs/auth?repo_id=repo-B", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var scoped map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &scoped)
	if scoped["repo_id"] != "repo-B" {
		t.Errorf("?repo_id=repo-B should return that repo's spec; got %v", scoped)
	}
}

func TestHandleEpicsList_WorkspaceModeAggregatesAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	repoA, repoB := seedWorkspaceRepos(t, db)
	seedSpecAndEpic(t, repoA, "auth", "auth-mvp")
	seedSpecAndEpic(t, repoB, "billing", "billing-mvp")

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/epics", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &rows)
	names := map[string]string{}
	for _, r := range rows {
		n, _ := r["name"].(string)
		rid, _ := r["repo_id"].(string)
		names[n] = rid
	}
	if names["auth-mvp"] != "repo-A" || names["billing-mvp"] != "repo-B" {
		t.Fatalf("workspace-mode epics should aggregate cross-repo with repo_id tag; got %v", names)
	}
}

func TestHandleEpicDetail_RepoIDDisambiguates(t *testing.T) {
	db := newTestDB(t)
	repoA, repoB := seedWorkspaceRepos(t, db)
	seedSpecAndEpic(t, repoA, "auth", "shared-name")
	seedSpecAndEpic(t, repoB, "auth", "shared-name")

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/epics/shared-name?repo_id=repo-B", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["repo_id"] != "repo-B" {
		t.Errorf("?repo_id=repo-B should return that repo's epic; got %v", got)
	}
}

func TestHandleItemLinks_WorkspaceModeFindsItemAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	repoA, _ := seedWorkspaceRepos(t, db)
	// Materialize one item in repo-A so walkAll can find it.
	if _, err := items.NewWithOptions(filepath.Join(repoA, ".squad"), "BUG", "alpha bravo charlie delta echo foxtrot", items.Options{Area: "test"}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	walk, err := items.Walk(filepath.Join(repoA, ".squad"))
	if err != nil || len(walk.Active) == 0 {
		t.Fatalf("locate item: walk err=%v active=%d", err, len(walk.Active))
	}
	itemID := walk.Active[0].ID

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/items/"+itemID+"/links", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s — links should not 404 in workspace mode", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"commits"`) {
		t.Errorf("links response missing commits field: %s", rec.Body.String())
	}
}
