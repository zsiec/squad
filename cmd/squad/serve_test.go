package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestServe_RefusesNonLoopbackBindWithoutToken(t *testing.T) {
	t.Setenv("SQUAD_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stderr bytes.Buffer
	// 0.0.0.0 is non-loopback. Without --token this should fail-fast with
	// the documented exit code 4 (security gate, distinct from generic
	// startup failures).
	if code := runServeCtx(ctx, 0, "0.0.0.0", ".squad", "", &stderr); code != 4 {
		t.Fatalf("expected exit code 4, got %d", code)
	}
}

func TestServe_RefusesWhitespaceOnlyToken(t *testing.T) {
	t.Setenv("SQUAD_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stderr bytes.Buffer
	// "   " satisfied the gate string-emptiness check but no HTTP header
	// could carry it.
	if code := runServeCtx(ctx, 0, "0.0.0.0", ".squad", "   ", &stderr); code != 4 {
		t.Fatalf("whitespace token should be treated as empty: code=%d", code)
	}
}

func TestServe_RefusesHostPortBind(t *testing.T) {
	t.Setenv("SQUAD_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stderr bytes.Buffer
	if code := runServeCtx(ctx, 0, "127.0.0.1:8080", ".squad", "", &stderr); code != 4 {
		t.Fatalf("--bind host:port should be rejected: code=%d", code)
	}
}

func TestServe_AcceptsLoopbackVariants(t *testing.T) {
	for _, bind := range []string{
		"127.0.0.1", "::1", "localhost",
		"Localhost", "LOCALHOST", "  localhost  ", "localhost.",
		"127.0.0.2", // any 127.0.0.0/8 is loopback per ParseIP
	} {
		if !isLoopbackBind(bind) {
			t.Fatalf("isLoopbackBind(%q) = false, want true", bind)
		}
	}
	for _, bind := range []string{"0.0.0.0", "192.168.1.1", "::", ""} {
		if isLoopbackBind(bind) {
			t.Fatalf("isLoopbackBind(%q) = true, want false", bind)
		}
	}
}

func TestServe_StartsAndShutsDown(t *testing.T) {
	t.Setenv("SQUAD_HOME", t.TempDir())

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	codeCh := make(chan int, 1)
	go func() {
		codeCh <- runServeCtx(ctx, port, "127.0.0.1", ".squad", "", &bytes.Buffer{})
	}()

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/api/health"
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("did not become healthy: err=%v resp=%v", err, resp)
	}

	cancel()

	select {
	case code := <-codeCh:
		if code != 0 {
			t.Fatalf("exit code=%d, want 0", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down within 3s")
	}
}
