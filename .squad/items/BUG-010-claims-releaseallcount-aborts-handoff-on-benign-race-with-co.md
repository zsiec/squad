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

### Reproduction

`TestReleaseAllCount_NoErrorWhenPeerStealsClaimMidFlight` (30 trials of `agent-a` running `ReleaseAllCount` while a parallel goroutine `ForceRelease`s two of agent-a's items). Confirmed RED on the pre-fix code — trials 2/3/5 returned `claims: no active claim on item`, aborting handoff mid-loop. GREEN on the fix.

### Root cause

`ReleaseAllCount` ran the SELECT outside any tx, then per-item `Release` opened its own tx. Between SELECT and per-row Release, a peer (`ForceRelease`, reaper, etc.) could drop one of the listed items. The per-row `Release` then returned `ErrNotClaimed` / `ErrNotYours`, the loop short-circuited, and the user's handoff reported failure with claims still held.

### Fix

`internal/claims/release.go` — `ReleaseAllCount` now wraps the SELECT and per-row releases in a single `s.withTx`, calls `s.releaseInTx` directly, and tolerates per-row `ErrNotClaimed` / `ErrNotYours` (continues the loop). Returns the actual count released. The `released` counter is reset inside the closure so `WithTxRetry`'s retry-on-busy doesn't double-count.

### Tests

`internal/claims/release_test.go`:
- `TestReleaseAllCount_ReleasesEverythingHeld` — happy path: 3 claims → count=3, history has 3 release rows.
- `TestReleaseAllCount_TolerantToVanishedRow` — vanished-before-snapshot: row deleted before call returns count=2, no error.
- `TestReleaseAllCount_NoErrorWhenPeerStealsClaimMidFlight` — 30 trials with concurrent `ForceRelease`. RED on old code, GREEN on fix.

### Evidence

```
$ go test ./... -race
... ok  github.com/zsiec/squad/internal/claims  ...
```
All packages pass.
