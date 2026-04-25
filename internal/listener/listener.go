package listener

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

type WakeReason int

const (
	WakeReasonNone WakeReason = iota
	WakeReasonConnect
	WakeReasonFallback
)

func (w WakeReason) String() string {
	switch w {
	case WakeReasonConnect:
		return "connect"
	case WakeReasonFallback:
		return "fallback"
	default:
		return "none"
	}
}

// Listener is a single-shot TCP wake primitive: bind 127.0.0.1:0, accept
// once, exit. Concurrent connects are coalesced — the first accepted
// conn resolves WaitWake, additional conns buffer briefly and are
// closed (either by the next WaitWake reader or by Close drain).
type Listener struct {
	tcp    *net.TCPListener
	conns  chan net.Conn
	closed chan struct{}
	wg     sync.WaitGroup
}

func New(addr string) (*Listener, error) {
	tcp, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	l := &Listener{
		tcp:    tcp.(*net.TCPListener),
		conns:  make(chan net.Conn, 16),
		closed: make(chan struct{}),
	}
	l.wg.Add(1)
	go l.acceptLoop()
	return l, nil
}

func (l *Listener) Port() int {
	return l.tcp.Addr().(*net.TCPAddr).Port
}

func (l *Listener) acceptLoop() {
	defer l.wg.Done()
	for {
		c, err := l.tcp.Accept()
		if err != nil {
			return
		}
		select {
		case l.conns <- c:
		default:
			_ = c.Close()
		}
	}
}

// WaitWake blocks until one of: an external TCP connect lands, the
// fallback timer fires, or ctx is cancelled. fallback is the slice
// duration — typically 30s, set short in tests. fallback ≤ 0 disables
// the fallback timer entirely.
func (l *Listener) WaitWake(ctx context.Context, fallback time.Duration) (WakeReason, error) {
	var t *time.Timer
	var tCh <-chan time.Time
	if fallback > 0 {
		t = time.NewTimer(fallback)
		tCh = t.C
		defer t.Stop()
	}
	select {
	case c := <-l.conns:
		_ = c.Close()
		return WakeReasonConnect, nil
	case <-tCh:
		return WakeReasonFallback, nil
	case <-ctx.Done():
		return WakeReasonNone, ctx.Err()
	case <-l.closed:
		return WakeReasonNone, errors.New("listener closed")
	}
}

func (l *Listener) Close() error {
	select {
	case <-l.closed:
		return nil
	default:
		close(l.closed)
	}
	// Order matters: tcp.Close unblocks Accept, then we wait for
	// acceptLoop to fully exit before closing l.conns. Without the wait,
	// acceptLoop could be mid-send to l.conns concurrently with the
	// close — a data race and a send-on-closed-channel panic.
	err := l.tcp.Close()
	l.wg.Wait()
	close(l.conns)
	for c := range l.conns {
		_ = c.Close()
	}
	return err
}
