package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeRestartToken(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "restart.token")
	if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
		t.Fatalf("write restart token: %v", err)
	}
	return p
}

func TestServer_Restart_ValidTokenSchedulesExit(t *testing.T) {
	db := newTestDB(t)
	tokenPath := writeRestartToken(t, "deadbeef-token")
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{RestartTokenPath: tokenPath})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
	req.Header.Set("X-Squad-Restart-Token", "deadbeef-token")
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("code=%d body=%s want 202", rec.Code, rec.Body.String())
	}
	select {
	case <-exitCh:
	case <-time.After(time.Second):
		t.Fatal("exit-scheduler never invoked")
	}
}

func TestServer_Restart_InvalidTokenReturns401NoExit(t *testing.T) {
	db := newTestDB(t)
	tokenPath := writeRestartToken(t, "the-real-token")
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{RestartTokenPath: tokenPath})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	cases := []struct {
		name   string
		header string
	}{
		{"wrong", "wrong-token"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
			if tc.header != "" {
				req.Header.Set("X-Squad-Restart-Token", tc.header)
			}
			req.RemoteAddr = "127.0.0.1:54321"
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("code=%d body=%s want 401", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"error":"invalid token"`) {
				t.Fatalf("body=%q does not match {\"error\":\"invalid token\"}", rec.Body.String())
			}
		})
	}

	select {
	case <-exitCh:
		t.Fatal("exit-scheduler invoked despite invalid token")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServer_Restart_NonLoopbackReturns404(t *testing.T) {
	db := newTestDB(t)
	tokenPath := writeRestartToken(t, "the-real-token")
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{RestartTokenPath: tokenPath})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	cases := []string{
		"192.168.1.5:9000",
		"10.0.0.1:1234",
		"203.0.113.7:443",
		"[2001:db8::1]:8080",
	}
	for _, ra := range cases {
		t.Run(ra, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
			req.Header.Set("X-Squad-Restart-Token", "the-real-token")
			req.RemoteAddr = ra
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("code=%d body=%s want 404 for %s", rec.Code, rec.Body.String(), ra)
			}
			if strings.Contains(rec.Body.String(), "invalid token") {
				t.Fatalf("non-loopback response leaks restart-token messaging: %s", rec.Body.String())
			}
		})
	}

	select {
	case <-exitCh:
		t.Fatal("exit-scheduler invoked despite non-loopback caller")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServer_Restart_MissingTokenFileReturns401(t *testing.T) {
	db := newTestDB(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{RestartTokenPath: missing})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
	req.Header.Set("X-Squad-Restart-Token", "anything")
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s want 401", rec.Code, rec.Body.String())
	}
	select {
	case <-exitCh:
		t.Fatal("exit-scheduler invoked despite missing token file")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServer_Restart_EmptyTokenFileReturns401(t *testing.T) {
	db := newTestDB(t)
	tokenPath := writeRestartToken(t, "")
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{RestartTokenPath: tokenPath})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
	req.Header.Set("X-Squad-Restart-Token", "")
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s want 401 (empty file must not match empty header)", rec.Code, rec.Body.String())
	}
	select {
	case <-exitCh:
		t.Fatal("exit-scheduler invoked despite empty token file")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServer_Restart_IPv6Loopback(t *testing.T) {
	db := newTestDB(t)
	tokenPath := writeRestartToken(t, "ipv6-token")
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{RestartTokenPath: tokenPath})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
	req.Header.Set("X-Squad-Restart-Token", "ipv6-token")
	req.RemoteAddr = "[::1]:54321"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("code=%d body=%s want 202", rec.Code, rec.Body.String())
	}
	select {
	case <-exitCh:
	case <-time.After(time.Second):
		t.Fatal("exit-scheduler never invoked")
	}
}
