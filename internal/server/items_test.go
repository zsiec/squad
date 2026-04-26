package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestItems_List(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 1 || out[0]["id"] != "BUG-100" {
		t.Fatalf("got %v", out)
	}
	if out[0]["progress_pct"].(float64) != 50 {
		t.Fatalf("progress=%v", out[0]["progress_pct"])
	}
}

func TestItems_List_IncludesR3R4Fields(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) == 0 {
		t.Fatal("expected at least one item")
	}
	item := out[0]
	for _, k := range []string{"epic", "depends_on", "parallel", "evidence_required"} {
		if _, ok := item[k]; !ok {
			t.Errorf("response missing key %q: %v", k, item)
		}
	}
}

func TestItems_List_IncludesClaimInfoWhenClaimed(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO claims (item_id, repo_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, ?, ?, '', 0)`,
		"BUG-100", testRepoID, "agent-tester", 1700000000, 1700000060,
	); err != nil {
		t.Fatal(err)
	}
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	var bug100 map[string]any
	for _, it := range out {
		if it["id"] == "BUG-100" {
			bug100 = it
			break
		}
	}
	if bug100 == nil {
		t.Fatal("BUG-100 not in response")
	}
	if bug100["claimed_by"] != "agent-tester" {
		t.Errorf("claimed_by=%v", bug100["claimed_by"])
	}
	if lt, ok := bug100["last_touch"].(float64); !ok || int64(lt) != 1700000060 {
		t.Errorf("last_touch=%v", bug100["last_touch"])
	}
}

func TestItems_Detail(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-100", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["title"] != "example bug for server tests" {
		t.Fatalf("title=%v", out["title"])
	}
	if _, ok := out["body_markdown"]; !ok {
		t.Fatal("expected body_markdown")
	}
}

func TestItems_Detail_404OnMissing(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-NOPE", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}
