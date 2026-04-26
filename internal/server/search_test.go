package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSearch_ItemAndMessage(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")

	dir := t.TempDir()
	itemsDir := filepath.Join(dir, "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("---\nid: BUG-901\ntype: BUG\npriority: P1\narea: server\nstatus: open\n---\n# Searchable widget bug\n\nThe payment widget overflows on Safari.\n")
	if err := os.WriteFile(filepath.Join(itemsDir, "BUG-901.md"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: dir})
	defer s.Close()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	mb, _ := json.Marshal(map[string]any{"thread": "global", "body": "remember the pancake recipe ingredients"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/messages", bytes.NewReader(mb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	resp2, err := srv.Client().Get(srv.URL + "/api/search?q=widget")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("search status=%d", resp2.StatusCode)
	}
	var hits []map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&hits); err != nil {
		t.Fatal(err)
	}
	foundItem := false
	for _, h := range hits {
		if h["kind"] == "item" && h["id"] == "BUG-901" {
			foundItem = true
		}
	}
	if !foundItem {
		t.Fatalf("expected item hit for widget search, got %v", hits)
	}

	resp3, err := srv.Client().Get(srv.URL + "/api/search?q=pancake")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var hits2 []map[string]any
	if err := json.NewDecoder(resp3.Body).Decode(&hits2); err != nil {
		t.Fatal(err)
	}
	foundMsg := false
	for _, h := range hits2 {
		if h["kind"] == "message" {
			foundMsg = true
		}
	}
	if !foundMsg {
		t.Fatalf("expected message hit for pancake search, got %v", hits2)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty q should be 400, got %d", rec.Code)
	}
}
