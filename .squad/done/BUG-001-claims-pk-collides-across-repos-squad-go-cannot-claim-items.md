---
id: BUG-001
title: claims PK collides across repos — squad go cannot claim items with names already claimed elsewhere
type: bug
priority: P1
area: store
status: done
estimate: 2h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-401f
captured_at: 1777236975
accepted_by: agent-401f
accepted_at: 1777236975
references: []
relates-to: []
blocked-by: []
---

## Problem

`claims.item_id` is declared `PRIMARY KEY` alone in `internal/store/migrations/001_initial.sql`. Item IDs are scoped per repo (e.g., every freshly-init'd repo seeds `EXAMPLE-001`), so once one repo claims `EXAMPLE-001`, every other repo's attempt to claim its own `EXAMPLE-001` fails with `ErrClaimTaken` — silently, because callers like `squad go` treat the error as "already taken, try the next one".

This was discovered when `squad go` reported "no ready items" in `/Users/zsiec/dev/squad` while `squad next` and `squad status` correctly listed `EXAMPLE-001` as ready. A stale claim row from `/private/tmp/squad-audit-go` (a tmp dir that no longer exists) was holding the PK.

## Context

Other repo-scoped tables (`items`, `specs`, `epics`) use `PRIMARY KEY (repo_id, name)`. Only `claims` got it wrong.

Affected callsites read/write claims by `item_id` alone:
- `cmd/squad/go.go:138` `loadClaimedSet`
- `internal/claims/claims.go` (Claim/Release/etc.)
- anywhere that does `WHERE item_id = ?` on the claims table

A grep for `FROM claims` and `UPDATE claims` will find them.

## Acceptance criteria

- [x] New test in `internal/claims/` proves two repos can independently claim the same `item_id` (currently fails with `ErrClaimTaken`).
- [x] Migration `005_claims_repo_scope.sql` recreates `claims` with `PRIMARY KEY (repo_id, item_id)`, preserving existing rows.
- [x] All claims-table queries scope by `repo_id` where the unique row is needed (Release, heartbeat, etc.).
- [x] `squad go` in a repo claims its `EXAMPLE-001` even when another repo already holds a claim on `EXAMPLE-001`.
- [x] `go test ./...` green; `CGO_ENABLED=0 go build ./...` green.

## Notes

The `progress` table also lacks a `repo_id` column entirely — separate latent bug, not in scope here. File as a follow-up.

The migration should preserve all existing rows; even orphan rows from deleted repos are harmless once the PK is composite (they just sit there). Cleanup of orphan rows is hygiene, not part of this fix.

## Resolution

Migration `internal/store/migrations/005_claims_repo_scope.sql` recreates
`claims` with `PRIMARY KEY (repo_id, item_id)` and copies existing rows.
Production callsites that read/write claims by `item_id` alone (or by
`agent_id` alone) were rescoped to include `repo_id`:
`internal/claims/{release,force_release,heartbeat}.go` and
`internal/chat/digest.go:74`. Test
`TestClaim_SameItemIDInDifferentRepos` proves two repos can independently
claim the same item id; bumped `TestMigrate_BootstrapsLegacyDBWithoutIntakeColumns`
expected version 4 → 5.

Two latent follow-ups discovered, not in scope here:
1. `release.go` / `force_release.go` UPDATE on `touches` is unscoped by
   `repo_id` — same agent touching same item id in two repos can have
   touches cleared in the wrong repo.
2. `progress` table has no `repo_id` column at all.
3. `internal/chat/digest.go:33` unread-message query and `messages` thread
   reads in general are unscoped by `repo_id`, so threads with the same
   name (e.g. `EXAMPLE-001`) bleed across repos.

