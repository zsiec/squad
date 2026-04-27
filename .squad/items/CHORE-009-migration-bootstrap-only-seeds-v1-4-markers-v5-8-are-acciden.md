---
id: CHORE-009
title: migration bootstrap only seeds v1-4 markers; v5-8 are accidentally idempotent
type: chore
priority: P3
area: internal/store
status: needs-refinement
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777292414
accepted_by: ""
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---

## Reviewer feedback
please leaern from the active agent and fill out the detauls

## Problem
`bootstrapLegacyVersions` in `internal/store/migrate.go` seeds `migration_versions` for legacy DBs by detecting schema markers up to v4 (`items.captured_by`) and v9 (`items.intake_session_id` — added by FEAT-014). Versions 5..8 have no markers, so on a legacy DB whose `migration_versions` row got dropped, those four migrations re-execute. They happen to be accidentally idempotent today (005's RENAME pattern wipes columns that 007 then re-adds), but any new ALTER TABLE migration landing after a CREATE-pattern migration will fail with "duplicate column name" — the same way 009 did until I added a v9 marker.

## Context
Discovered while landing FEAT-014. `TestMigrate_BootstrapsLegacyDB` in `internal/store/migrate_test.go:130` exercises the dropped-migration_versions path and is the canary. Lives in `internal/store/migrate.go:150-186`.

## Acceptance criteria
- [ ] `bootstrapLegacyVersions` detects v5..v8 by stable schema markers and seeds them when present.
  - v5 candidate marker: claims composite PK (probe via `pragma_index_list` for the multi-column PK index).
  - v6 candidate marker: `commits` table existence.
  - v7 candidate marker: `claims.worktree` column.
  - v8 candidate marker: `agent_events` table existence.
- [ ] After dropping `migration_versions` on a fully-migrated DB and re-running `Migrate`, no migration's body re-executes (assert via a side-effect probe that would visibly fail on second-run, e.g. inserting into a table whose ALTER was supposed to be a one-shot).
- [ ] Existing `TestMigrate_BootstrapsLegacyDB` and `TestMigrate_BootstrapsLegacyDBWithoutIntakeColumns` continue to pass.

## Notes
Minimum scope is the four markers + a strengthened test. The next migration that needs a non-idempotent ALTER TABLE will hit the same bug — solving it once is cheaper than diagnosing again.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
