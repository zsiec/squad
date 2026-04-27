---
id: FEAT-021
title: 'intake: commit item-only bundles atomically'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 2h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290819
accepted_by: web
accepted_at: 1777291152
references: []
relates-to: []
blocked-by: [FEAT-019, FEAT-020]
---

## Problem
Item-only bundles must commit all-or-nothing: every item file written and every row inserted, or nothing. A partial commit leaves the repo in a confusing state.

## Context
`internal/intake/commit.go`. `Commit(ctx, db, squadDir, sessionID, agentID, bundle, ready) (CommitResult, error)`:
- Calls `Validate` (FEAT-019).
- Allocates IDs via `items.NextID`.
- One SQLite tx: writes `.squad/items/<id>-<slug>.md` files (`O_EXCL`), inserts items rows with `intake_session_id`, `captured_by`, `captured_at`, status from `ready` flag.
- On failure: deferred file cleanup, tx rollback, error return.
- Marks session committed on success.

Plan ref: Task 8.

## Acceptance criteria
- [ ] Happy path: files on disk + rows in DB + session marked `committed` + `bundle_json` populated.
- [ ] Default status is `captured`; `ready=true` flag promotes to `ready`.
- [ ] Failure path: inject write failure on second item; assert first item file deleted, no DB rows persisted.
- [ ] Re-committing an already-committed session returns `IntakeAlreadyClosed`.
- [ ] Tests cover all four scenarios.

## Notes
This is the highest-risk item in the core epic — atomicity bugs are nasty. Test the failure rollback explicitly; do not skip.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
