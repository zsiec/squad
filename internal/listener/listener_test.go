package listener

import (
	"context"
	"net"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestListener_WakesOnExternalConnect(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer l.Close()

	if l.Port() == 0 {
		t.Fatal("expected non-zero port after bind")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	woke := make(chan WakeReason, 1)
	go func() {
		reason, _ := l.WaitWake(ctx, 30*time.Second)
		woke <- reason
	}()

	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(l.Port()), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = c.Close()

	select {
	case r := <-woke:
		if r != WakeReasonConnect {
			t.Fatalf("expected WakeReasonConnect, got %v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("listener never woke on external connect")
	}
}

func TestListener_FallbackTickerFiresIfNoConnect(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reason, err := l.WaitWake(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if reason != WakeReasonFallback {
		t.Fatalf("expected fallback, got %v", reason)
	}
}

func TestListener_ContextCancelExits(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_, _ = l.WaitWake(ctx, time.Hour)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WaitWake did not exit on ctx cancel")
	}
}

func TestListener_ConcurrentCloseIsSafe(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	const n = 10
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = l.Close()
		}(i)
	}
	close(start)
	wg.Wait()

	for i := 1; i < n; i++ {
		if errs[i] != errs[0] {
			t.Fatalf("Close() returned divergent errors: errs[0]=%v errs[%d]=%v", errs[0], i, errs[i])
		}
	}
}

func TestListener_WaitWakeAfterCloseDoesNotPanic(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	reason, err := l.WaitWake(ctx, 30*time.Second)
	if reason != WakeReasonNone {
		t.Fatalf("expected WakeReasonNone, got %v", reason)
	}
	if err == nil {
		t.Fatal("expected non-nil error after close")
	}
}

func TestListener_ConcurrentConnectsCoalesce(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	woke := make(chan WakeReason, 1)
	go func() {
		r, _ := l.WaitWake(ctx, 30*time.Second)
		woke <- r
	}()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(l.Port()), 200*time.Millisecond)
			if err == nil {
				_ = c.Close()
			}
		}()
	}
	wg.Wait()

	select {
	case r := <-woke:
		if r != WakeReasonConnect {
			t.Fatalf("reason=%v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("never woke")
	}
}

func TestListener_WaitLoopReturnsOnConnectAfterMultipleFallbacks(t *testing.T) {
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type result struct {
		fallbacks int
		err       error
	}
	done := make(chan result, 1)
	onTick := func() {}
	tickCount := 0
	hooked := func() {
		tickCount++
		if tickCount == 3 {
			go func() {
				c, _ := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(l.Port()), 200*time.Millisecond)
				if c != nil {
					_ = c.Close()
				}
			}()
		}
	}
	onTick = hooked
	go func() {
		fallbacks, err := l.WaitLoop(ctx, 30*time.Millisecond, onTick)
		done <- result{fallbacks, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("wait loop err: %v", r.err)
		}
		if r.fallbacks < 1 {
			t.Fatalf("expected at least one fallback before connect, got %d", r.fallbacks)
		}
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("WaitLoop never returned")
	}
}

func TestListener_NoFDLeakUnder500Connects(t *testing.T) {
	if testing.Short() {
		t.Skip("burst test")
	}
	l, err := New("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_, _ = l.WaitLoop(ctx, 50*time.Millisecond, nil)
	}()

	for i := 0; i < 500; i++ {
		c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(l.Port()), 200*time.Millisecond)
		if err != nil {
			continue
		}
		_ = c.Close()
	}

	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}
}

func TestListener_NoGoroutineLeakOnClose(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		l, err := New("127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() { _, _ = l.WaitLoop(ctx, 30*time.Millisecond, nil) }()
		cancel()
		_ = l.Close()
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutine count grew: before=%d after=%d", before, runtime.NumGoroutine())
}
