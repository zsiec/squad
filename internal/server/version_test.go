package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestServer_Version_Loopback(t *testing.T) {
	db := newTestDB(t)
	before := time.Now()
	s := New(db, testRepoID, Config{
		Version:    "9.9.9-test",
		BinaryPath: "/opt/squad/bin/squad",
	})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}

	var body struct {
		Version    string `json:"version"`
		BinaryPath string `json:"binary_path"`
		StartedAt  string `json:"started_at"`
		PID        int    `json:"pid"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if body.Version != "9.9.9-test" {
		t.Errorf("version=%q want 9.9.9-test", body.Version)
	}
	if body.BinaryPath != "/opt/squad/bin/squad" {
		t.Errorf("binary_path=%q want /opt/squad/bin/squad", body.BinaryPath)
	}
	if body.PID != os.Getpid() {
		t.Errorf("pid=%d want %d", body.PID, os.Getpid())
	}
	ts, err := time.Parse(time.RFC3339, body.StartedAt)
	if err != nil {
		t.Fatalf("started_at not RFC3339: %v (%q)", err, body.StartedAt)
	}
	if ts.Before(before.Add(-time.Second)) || ts.After(time.Now().Add(time.Second)) {
		t.Errorf("started_at=%s outside [%s, now]", ts, before)
	}
}

func TestServer_Version_IPv6Loopback(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{Version: "v", BinaryPath: "/x"})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.RemoteAddr = "[::1]:54321"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServer_Version_RejectsNonLoopback(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{Version: "v", BinaryPath: "/x"})
	defer s.Close()

	cases := []string{
		"192.168.1.5:9000",
		"10.0.0.1:1234",
		"203.0.113.7:443",
		"[2001:db8::1]:8080",
	}
	for _, ra := range cases {
		t.Run(ra, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
			req.RemoteAddr = ra
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("code=%d body=%s; want 404 for %s", rec.Code, rec.Body.String(), ra)
			}
			// Must NOT leak version payload on rejection.
			if strings.Contains(rec.Body.String(), "binary_path") {
				t.Fatalf("body leaks version payload on non-loopback: %s", rec.Body.String())
			}
		})
	}
}
