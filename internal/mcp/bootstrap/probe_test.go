package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setProbeBase(t *testing.T, url string) {
	t.Helper()
	prev := probeBase
	probeBase = url
	t.Cleanup(func() { probeBase = prev })
}

func TestProbe_DaemonReachable_PopulatesResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"9.9.9","binary_path":"/opt/squad/bin/squad","started_at":"2026-04-27T07:00:00Z","pid":4242}`))
	}))
	defer ts.Close()
	setProbeBase(t, ts.URL)

	got, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !got.Present {
		t.Errorf("Present=false, want true")
	}
	if got.Version != "9.9.9" {
		t.Errorf("Version=%q want 9.9.9", got.Version)
	}
	if got.BinaryPath != "/opt/squad/bin/squad" {
		t.Errorf("BinaryPath=%q", got.BinaryPath)
	}
	if got.PID != 4242 {
		t.Errorf("PID=%d want 4242", got.PID)
	}
	want, _ := time.Parse(time.RFC3339, "2026-04-27T07:00:00Z")
	if !got.StartedAt.Equal(want) {
		t.Errorf("StartedAt=%v want %v", got.StartedAt, want)
	}
}

func TestProbe_NoDaemon_ReturnsAbsent(t *testing.T) {
	// Bind+release a port so we have a definitely-closed address.
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := ts.URL
	ts.Close()
	setProbeBase(t, url)

	got, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v want nil on connection refused", err)
	}
	if got.Present {
		t.Errorf("Present=true, want false on connection refused")
	}
}

func TestProbe_SlowDaemon_TimesOutAsAbsent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	setProbeBase(t, ts.URL)

	prev := probeTimeout
	probeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { probeTimeout = prev })

	got, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v want nil on timeout", err)
	}
	if got.Present {
		t.Error("Present=true on slow daemon, want false (timeout)")
	}
}

func TestProbe_Non200_ReturnsAbsent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()
	setProbeBase(t, ts.URL)

	got, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v want nil on non-200", err)
	}
	if got.Present {
		t.Error("Present=true on non-200, want false")
	}
}
