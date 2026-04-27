---
id: FEAT-020
title: 'intake: slug conflict detection across DB and disk'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290812
accepted_by: web
accepted_at: 1777291152
references: []
relates-to: []
blocked-by: [FEAT-014]
---

## Problem
Spec/epic slugs must be unique across both the DB tables and the on-disk markdown files. Either source can be ahead of the other; both must be checked.

## Context
Extends `internal/intake/commit.go`. `CheckSlugAvailable(ctx, db, repoID, squadDir, kind, slug) error` returns `IntakeSlugConflict` if a row exists in `specs`/`epics` table OR a file exists at `.squad/<kind>s/<slug>.md`.

Plan ref: Task 7.

## Acceptance criteria
- [ ] Returns nil when neither DB row nor file exists.
- [ ] Returns `IntakeSlugConflict` when a DB row exists.
- [ ] Returns `IntakeSlugConflict` when a file exists on disk (even if DB out of sync).
- [ ] Available for kind=spec doesn't conflict with same slug for kind=epic, and vice versa.
- [ ] Tests cover all four cases.

## Notes
This is a pre-flight check; the actual write uses `O_EXCL` for the race-tight version.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
