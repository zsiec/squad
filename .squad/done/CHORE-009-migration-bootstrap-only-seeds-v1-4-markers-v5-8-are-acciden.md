---
id: CHORE-009
title: migration bootstrap only seeds v1-4 markers; v5-8 are accidentally idempotent
type: chore
priority: P3
area: internal/store
status: done
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777292414
accepted_by: web
accepted_at: 1777303726
references: []
relates-to: []
blocked-by: []
---
## Refinement history
### Round 1 — 2026-04-27
please leaern from the active agent and fill out the detauls

## Problem
`bootstrapLegacyVersions` in `internal/store/migrate.go:150` seeds `migration_versions` for legacy DBs by detecting schema markers for v1 (always-on once `items.epic_id` exists), v4 (`items.captured_by`), and v9 (`items.intake_session_id` — added by FEAT-014). Versions 5–8 have no markers, so on a fully-migrated DB whose `migration_versions` row got dropped — or any pre-Task-5 DB inherited via the bootstrap path — those four migrations re-execute on the next `Migrate` call.

This is not just "accidentally idempotent." Tracing what actually happens on re-run:

- 005 (`ALTER TABLE claims RENAME` + `CREATE TABLE claims (...)` without the `worktree` column) wipes live `worktree` values from the in-place `claims` table. The follow-up `INSERT INTO claims … SELECT … FROM claims_old` cannot restore `worktree` because the new table has no such column.
- 007 (`ALTER TABLE claims ADD COLUMN worktree`) then re-adds the column with default `''`.
- Net effect: every in-progress claim with a non-empty worktree path silently loses it. Plus 006 / 008 are no-ops thanks to `CREATE TABLE IF NOT EXISTS`, but that is brittle — the next ALTER-pattern migration to land after a CREATE-pattern migration will fail with "duplicate column name", exactly the way 009 did until v9 got a marker.

## Context
Discovered while landing FEAT-014. The canary is `TestMigrate_BootstrapsLegacyDB` (`internal/store/migrate_test.go:130`), with a sibling `TestMigrate_BootstrapsLegacyDBWithoutIntakeColumns` (`migrate_test.go:266`) covering the pre-FEAT-014 shape. Neither test currently asserts that `worktree` values survive the bootstrap-then-re-migrate path.

Verified at HEAD: migrations 5–8 in `internal/store/migrations/` are `005_claims_repo_scope.sql`, `006_commits.sql`, `007_claim_worktree.sql`, `008_agent_events.sql`. Each leaves a stable, schema-only marker that pragma queries can reach without depending on row data:

- v5 → `claims` PK is composite `(repo_id, item_id)`. Probe: `SELECT count(*) FROM pragma_table_info('claims') WHERE pk > 0` returns 2 post-5, 1 pre-5.
- v6 → `commits` table. Probe: `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='commits'`.
- v7 → `claims.worktree` column. Probe: `SELECT count(*) FROM pragma_table_info('claims') WHERE name='worktree'`.
- v8 → `agent_events` table. Probe: `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='agent_events'`.

## Acceptance criteria
- [ ] `bootstrapLegacyVersions` seeds v5–v8 entries (`legacy-claims_repo_scope`, `legacy-commits`, `legacy-claim_worktree`, `legacy-agent_events`) when each marker is present, using the four pragma probes listed above. Markers gate seeding independently — partial schemas (e.g. v5 present but v8 missing on a half-migrated DB) seed only what they have.
- [ ] New regression test in `internal/store/migrate_test.go`: bootstrap a fully-migrated DB, write a known non-empty `worktree` value into a row in `claims`, drop `migration_versions`, re-run `Migrate`, and assert the `worktree` value is unchanged. Without the v5/v7 markers this fails because migration 5 re-runs and clobbers the column.
- [ ] After bootstrap on a fully-migrated DB and a second `Migrate` call, `loadApplied` returns versions 1..N and the body of no migration re-executes (assert by counting rows in `migration_versions` and by the worktree-preservation probe above).
- [ ] Existing `TestMigrate_BootstrapsLegacyDB` and `TestMigrate_BootstrapsLegacyDBWithoutIntakeColumns` continue to pass unchanged.

## Notes
Minimum scope is the four markers + the worktree-preservation regression test. The data-loss observation strengthens the case for fixing this now rather than waiting for the next ALTER-pattern migration to surface a "duplicate column name" failure. Markers are pragma-only (no row-data dependence), so they survive on a freshly-bootstrapped DB before any user data lands.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
