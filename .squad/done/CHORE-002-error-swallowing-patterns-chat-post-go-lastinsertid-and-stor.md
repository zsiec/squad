---
id: CHORE-002
title: 'error-swallowing patterns: chat/post.go LastInsertId and store/migrate.go Rollback'
type: chore
priority: P3
area: internal
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777241487
accepted_by: web
accepted_at: 1777241683
references: []
relates-to: []
blocked-by: []
---

## Problem

Two minor swallowed-error sites worth one cleanup pass:

1. `internal/chat/post.go:63` — `id, _ = res.LastInsertId()`. If the call fails, `id` is `0` and the published bus event carries `id: 0`. SSE clients then see a placeholder id and discard.
2. `internal/store/migrate.go:125` — `defer func() { _ = tx.Rollback() }()`. If `Rollback` itself fails (disk full, DB corruption), we lose the signal that the migration is in an inconsistent state.

## Context

Per CLAUDE.md, "trust internal invariants — validate only at system boundaries". These two are precisely the kind of system-boundary errors (DB driver) that should NOT be silently swallowed. Neither is severe today, but both quietly hide failure modes that would be painful to debug if they ever surfaced.

## Acceptance criteria

- [ ] `chat/post.go:63` returns the `LastInsertId` error (or logs it if there's no error path on the caller).
- [ ] `store/migrate.go:125` checks `tx.Rollback()` and either returns the rollback error or logs it (filter `sql.ErrTxDone` so a successful commit doesn't trip the log).
- [ ] No new silent `_ =` introduced; existing intentional ones (e.g., `_ = rows.Close()` after iteration error) left alone — call those out in the PR description so reviewers know the scope.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Small, low-risk cleanup — good "warm up" item before the bigger BUG-009 / BUG-010 transactional fixes.

## Resolution

### Fix

`internal/chat/post.go` — `id, err = res.LastInsertId()` now propagates the error inside the WithTxRetry closure. The id feeds the post-commit `bus.Publish` event payload; silent id=0 would corrupt every downstream listener, so failing the tx is the right call. modernc/sqlite computes LastInsertId locally with no roundtrip, so the practical-cost of this guard is near-zero — but the correctness benefit if the impossible ever happens is real.

`internal/store/migrate.go` — deferred rollback now checks the result and `fmt.Fprintf`s any non-`sql.ErrTxDone` error to stderr. ErrTxDone is the expected post-Commit case and stays silent. Library packages don't normally log to stderr, but `applyMigration` runs once at bootstrap, has no caller-supplied logger, and the deferred function can't return — stderr is exactly where an operator looks during init/up.

Sibling `_ = tx.Rollback()` at `internal/store/store.go:57` (in `WithTxRetry`) was explicitly out of scope per the AC and left untouched.

### Evidence

```
$ go test ./internal/chat ./internal/store
ok  	github.com/zsiec/squad/internal/chat   0.669s
ok  	github.com/zsiec/squad/internal/store  1.017s

$ go test ./... -count=1 -race
... (0 FAIL lines)
```
