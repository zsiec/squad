package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsAPIRespectsToken(t *testing.T) {
	db := newTestDB(t)
	s := New(db, "repo-1", Config{Token: "secret"})
	defer s.Close()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/stats")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/stats", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorized: %d", resp.StatusCode)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["schema_version"] == nil {
		t.Errorf("missing schema_version")
	}
}
