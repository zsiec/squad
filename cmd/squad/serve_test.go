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
	// 0.0.0.0 is non-loopback. Without --token this should fail-fast.
	code := runServeCtx(ctx, 0, "0.0.0.0", ".squad", "", &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0")
	}
}

func TestServe_AcceptsLoopbackVariants(t *testing.T) {
	for _, bind := range []string{"127.0.0.1", "::1", "localhost"} {
		if !isLoopbackBind(bind) {
			t.Fatalf("isLoopbackBind(%q) = false, want true", bind)
		}
	}
	for _, bind := range []string{"0.0.0.0", "192.168.1.1", "::"} {
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
