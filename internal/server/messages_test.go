package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessages_PostThenList(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"thread": "global", "body": "hello @bob"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("post: %d", resp.StatusCode)
	}
	resp.Body.Close()

	listResp, _ := srv.Client().Get(srv.URL + "/api/messages?thread=global")
	defer listResp.Body.Close()
	var out []map[string]any
	_ = json.NewDecoder(listResp.Body).Decode(&out)
	if len(out) != 1 || out[0]["body"] != "hello @bob" {
		t.Fatalf("got %v", out)
	}
}

func TestMessages_PostBadJSON(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestMessages_PostEmptyBodyRejected(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	body, _ := json.Marshal(map[string]any{"thread": "global", "body": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}
