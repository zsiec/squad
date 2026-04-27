package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServer_Restart_LoopbackSchedulesExit(t *testing.T) {
	db := newTestDB(t)
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
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

func TestServer_Restart_NonLoopbackReturns404(t *testing.T) {
	db := newTestDB(t)
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{})
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
			req.RemoteAddr = ra
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("code=%d body=%s want 404 for %s", rec.Code, rec.Body.String(), ra)
			}
		})
	}

	select {
	case <-exitCh:
		t.Fatal("exit-scheduler invoked despite non-loopback caller")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServer_Restart_IPv6Loopback(t *testing.T) {
	db := newTestDB(t)
	exitCh := make(chan struct{}, 1)
	s := New(db, testRepoID, Config{})
	defer s.Close()
	s.exitFunc = func() { exitCh <- struct{}{} }
	s.restartExitDelay = 10 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/api/_internal/restart", nil)
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
