---
id: CHORE-007
title: extract claim-holder lookup helper for repeated SELECT agent_id FROM claims
type: chore
priority: P3
area: claims
status: done
estimate: 45m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777255655
accepted_by: web
accepted_at: 1777255754
references: []
relates-to: []
blocked-by: []
---

## Problem

The SQL `SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?` (or its arg-flipped twin) appears in 10+ Go files, mostly tests but also two prod paths:
- `cmd/squad/claim.go:159`
- `cmd/squad/progress.go:47`
- `internal/items/refine.go:76`
- and 7+ test files under `internal/server/` repeating the same shape.

Per CLAUDE.md "three real callers before extracting" — the threshold is well past, and the duplication has produced argument-order skew (`repo_id=? AND item_id=?` vs `item_id=? AND repo_id=?`).

## Context

A small helper like `claims.HolderOf(ctx, db, repoID, itemID) (string, error)` would centralize the query, normalize the argument order, and turn the test repetition into a single wrapper call.

Worth checking: `lookupClaimHolderDB` in `cmd/squad/claim.go:156` already does this — but only for the cobra path. Promoting it to `internal/claims` and refactoring the call sites would let tests reuse it too.

## Acceptance criteria

- [x] `claims.HolderOf(ctx, q rowQuerier, repoID, itemID) (string, error)`
  in `internal/claims/holder.go`. The `rowQuerier` interface accepts both
  `*sql.DB` and `*sql.Tx` so the helper works inside or outside a tx.
- [~] Production callers migrated where the package boundary allows:
  `cmd/squad/claim.go:lookupClaimHolderDB` and `cmd/squad/progress.go`
  both call `claims.HolderOf`. The third site,
  `internal/items/refine.go:Recapture`, cannot — `internal/claims`
  imports `internal/items`, so calling claims from items would close
  an import cycle. Left in place with a comment recording the cycle
  constraint; the inline query is byte-identical to what the helper
  would emit.
- [x] Test callers migrated: `internal/server/{release,
  items_recapture, blocked, handoff, force_release, done}_test.go`
  (six files, seven sites). `internal/items/refine_test.go:404` left
  inline for the same cycle reason as the production caller.
- [x] No behavior change — full `go test ./... -count=1` passes
  (see Resolution evidence). `golangci-lint run` returns 0 issues.

## Notes

P3, low risk. Pure refactor; no behavior change. Worth doing while the surface is small.

## Resolution

### Fix

`internal/claims/holder.go` (new) — `HolderOf(ctx, q rowQuerier,
repoID, itemID) (string, error)`. The `rowQuerier` interface is the
intersection of `*sql.DB.QueryRowContext` and `*sql.Tx.QueryRowContext`
(matching method signature) so callers don't have to choose between
near-identical helpers. The function returns `("", sql.ErrNoRows)`
when no claim exists so callers can distinguish unclaimed from a
real query failure.

`cmd/squad/claim.go:lookupClaimHolderDB` — wraps `claims.HolderOf`
and discards the error (the cobra wrapper renders the holder string
or empty; that contract is preserved).

`cmd/squad/progress.go:Progress` — replaced the inline
`QueryRowContext.Scan` with a `claims.HolderOf` call, then a switch
on `errors.Is(err, sql.ErrNoRows)` / `holder != args.AgentID`. Same
error semantics as before (`ErrNotClaimed`, `ErrNotYours`).

`internal/server/{release, items_recapture, blocked, handoff,
force_release, done}_test.go` — collapsed `var holder string` +
multi-line `QueryRowContext.Scan` into single-line
`holder, err := claims.HolderOf(...)` calls. One `done_test.go`
site discards the value (the assertion is "claim still exists").

### Import-cycle constraint (deferred caller)

`internal/items/refine.go:Recapture` and
`internal/items/refine_test.go:404` cannot use the helper.
`internal/claims` already imports `internal/items` (see
`internal/claims/blocked.go`, `done.go`, `preflight.go`), so calling
back from items would close the cycle. Both sites carry the same
inline `SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`
query; production site has a comment explaining the cycle. Future
work: relocate the helper to a neutral package (e.g.
`internal/claimsdb`) or break the items→claims dependency direction.

### Evidence

```
$ go test ./... -count=1
... (37 packages green, no failures)

$ go vet ./...
(no output)

$ golangci-lint run
0 issues.
```
