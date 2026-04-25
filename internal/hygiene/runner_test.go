package hygiene

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunner_DebouncesBackToBackTicks(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "hygiene.lock")
	var calls atomic.Int32
	work := func(ctx context.Context) error { calls.Add(1); return nil }
	r := NewRunner(lock, 10*time.Second)
	if err := r.RunIfDue(context.Background(), work); err != nil {
		t.Fatal(err)
	}
	if err := r.RunIfDue(context.Background(), work); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls=%d want 1 (second call should be debounced)", got)
	}
}

func TestRunner_RunsAgainAfterWindow(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "hygiene.lock")
	var calls atomic.Int32
	work := func(ctx context.Context) error { calls.Add(1); return nil }
	r := NewRunner(lock, 1*time.Millisecond)
	_ = r.RunIfDue(context.Background(), work)
	time.Sleep(1100 * time.Millisecond) // need >1s to push past unix-second resolution
	_ = r.RunIfDue(context.Background(), work)
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls=%d want 2", got)
	}
}
