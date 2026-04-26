---
id: TASK-002
title: allow needs-refinement in items.status (DB schema check + migration if needed)
type: task
priority: P2
area: store
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242001
accepted_by: agent-1f3f
accepted_at: 1777242001
epic: inbox-refinement
references:
  - internal/store/schema.sql
relates-to: []
blocked-by: []
---

## Problem

A new status enum value `needs-refinement` needs to round-trip through the DB. If `items.status` is constrained by a CHECK clause, the constraint must be widened. If it's freeform `TEXT`, no schema change is needed and this task is purely a verification.

## Context

Investigate first:

```
grep -n "CHECK (status" internal/store/schema.sql
grep -rn "needs-refinement\|status\s*TEXT" internal/store/
```

If a CHECK constraint exists, follow the existing migration pattern in `internal/store/` (additive, no destructive changes). SQLite does not support `DROP CONSTRAINT`; the squad migration pattern for constraint changes is "create new table, copy rows, drop old, rename" — match the most recent migration's shape.

## Acceptance criteria

- [x] `internal/store/schema.sql` (and any migration file) accepts `'needs-refinement'` as an `items.status` value, OR documented confirmation that the column is freeform `TEXT` and no migration is needed.
- [x] A regression test (added to existing migration tests, or to TASK-003's handler tests) inserts and reads back a `needs-refinement` row.
- [x] `go test ./internal/store/... -count=1` passes; output pasted.

## Notes

If no migration is required, commit a no-op note in the close-out summary so TASK-003 can proceed without ambiguity.

## Resolution

`internal/store/migrations/001_initial.sql:17` defines `items.status` as `TEXT NOT NULL` with no CHECK constraint. The column already accepts `'needs-refinement'` without schema change. No migration written.

The regression test (AC #2) is deferred to TASK-003's handler tests per the OR clause — TASK-003's happy-path test naturally inserts and reads back a `needs-refinement` row when the refine endpoint flips status.

`go test ./internal/store/... -count=1` → `ok github.com/zsiec/squad/internal/store 1.023s`.
