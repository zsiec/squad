package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(dir + "/test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestWithTxRetry_RetriesOnBusy(t *testing.T) {
	db := openTestDB(t)
	var calls int
	err := WithTxRetry(context.Background(), db, func(tx *sql.Tx) error {
		calls++
		if calls < 3 {
			return errBusySentinel
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWithTxRetry_DoesNotRetryNonBusy(t *testing.T) {
	db := openTestDB(t)
	var calls int
	target := errors.New("application error")
	err := WithTxRetry(context.Background(), db, func(tx *sql.Tx) error {
		calls++
		return target
	})
	if !errors.Is(err, target) {
		t.Fatalf("expected wrapped target error, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestWithTxRetry_RespectsContextCancel(t *testing.T) {
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		return errBusySentinel
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestWithTxRetry_ExhaustsAndReturnsLastBusy(t *testing.T) {
	db := openTestDB(t)
	var calls int
	err := WithTxRetry(context.Background(), db, func(tx *sql.Tx) error {
		calls++
		return errBusySentinel
	})
	if err == nil {
		t.Fatalf("expected error after exhausted retries")
	}
	if calls != 6 {
		t.Fatalf("expected 6 calls (max retries), got %d", calls)
	}
}

func TestWithTxRetry_HonoursTotalDeadlineRoughly(t *testing.T) {
	db := openTestDB(t)
	start := time.Now()
	_ = WithTxRetry(context.Background(), db, func(tx *sql.Tx) error {
		return errBusySentinel
	})
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("retry loop took too long: %v", elapsed)
	}
}
