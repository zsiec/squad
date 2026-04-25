package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestE2E_PostsThenListsCleanState(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "thomas", "Thomas")
	s := New(db, testRepoID, Config{SquadDir: "testdata", RepoID: testRepoID})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	doJSON := func(method, path string, body any) *http.Response {
		var b io.Reader
		if body != nil {
			buf, _ := json.Marshal(body)
			b = bytes.NewReader(buf)
		}
		req, _ := http.NewRequest(method, srv.URL+path, b)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Squad-Agent", "thomas")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	r := doJSON(http.MethodPost, "/api/messages", map[string]any{"thread": "global", "body": "hello squad"})
	if r.StatusCode != http.StatusOK {
		t.Fatalf("post messages: %d", r.StatusCode)
	}
	r.Body.Close()

	r = doJSON(http.MethodGet, "/api/messages?thread=global", nil)
	defer r.Body.Close()
	var msgs []map[string]any
	_ = json.NewDecoder(r.Body).Decode(&msgs)
	if len(msgs) != 1 || msgs[0]["body"] != "hello squad" {
		t.Fatalf("messages: %v", msgs)
	}

	r2 := doJSON(http.MethodGet, "/api/items", nil)
	defer r2.Body.Close()
	var items []map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&items)
	if len(items) != 1 || items[0]["id"] != "BUG-100" {
		t.Fatalf("items: %v", items)
	}

	r3 := doJSON(http.MethodGet, "/api/health", nil)
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("health: %d", r3.StatusCode)
	}
	r3.Body.Close()
}
