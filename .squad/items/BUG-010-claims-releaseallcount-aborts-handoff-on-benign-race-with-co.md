---
id: BUG-010
title: claims ReleaseAllCount aborts handoff on benign race with concurrent claim
type: bug
priority: P1
area: claims
status: open
estimate: 45m
risk: medium
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241460
accepted_by: web
accepted_at: 1777241676
references: []
relates-to: []
blocked-by: []
---

## Problem

`internal/claims/release.go:71-89` (`ReleaseAllCount`) reads the agent's claim list outside any transaction, then calls `s.Release(...)` per item in a loop. Each per-item `Release` opens its own transaction and re-validates the holder via `ErrNotClaimed` / `ErrNotYours`. If a peer's process intervenes between the SELECT and a particular `Release` (e.g., reaper releases a stale claim, or the row gets manipulated through another path), `Release` returns an error and the loop short-circuits — so a `squad handoff` reports failure mid-way and leaves the remaining claims still held.

## Context

`ReleaseAllCount` is what `squad handoff` calls. The user expectation is "release everything I'm holding". A benign race that kills the loop turns handoff from idempotent into flaky and partial. The fix is the same shape as BUG-009: do the SELECT and the per-row writes inside a single transaction so the snapshot we iterate matches the rows we're modifying.

## Acceptance criteria

- [ ] `ReleaseAllCount` performs SELECT + per-item DELETE/INSERT-history inside a single `WithTxRetry`, not per-item transactions.
- [ ] If a row vanishes between SELECT and DELETE inside the same tx (impossible at SQLite serialization, but assert anyway), the loop continues — handoff is best-effort across the snapshot, not all-or-nothing.
- [ ] `ErrNotClaimed` from a per-row release no longer aborts the loop; count returned is the number of claims actually released.
- [ ] Test: with two concurrent agents both calling release on overlapping items, the call from agent-A does not return an error if agent-B grabbed something agent-A no longer holds.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Verified by reading `internal/claims/release.go`. Sibling to BUG-009 (multi-statement write atomicity).

## Resolution
(Filled in when status → done.)
