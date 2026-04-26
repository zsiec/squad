---
id: CHORE-002
title: 'error-swallowing patterns: chat/post.go LastInsertId and store/migrate.go Rollback'
type: chore
priority: P3
area: internal
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
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
(Filled in when status → done.)
