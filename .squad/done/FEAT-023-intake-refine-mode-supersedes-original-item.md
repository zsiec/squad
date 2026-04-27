---
id: FEAT-023
title: 'intake: refine mode supersedes original item'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 1h
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
blocked-by: [FEAT-021]
---

## Problem
When committing a refine-mode session, the original captured item must be archived (not deleted) and the new item must replace it on the active scoreboard.

## Context
Extends `commit.go`. After writing the new item file (single-item bundle):
- Move original item file to `.squad/items/.archive/<original-id>-<slug>.md`.
- Old item row: status transitions to whatever the items state machine accepts for "superseded" — verify in `internal/items/` first; fall back to `done` with reason `intake_refine_superseded` if `superseded` isn't a recognized state.
- Insert `claim_history` row with reason `intake_refine_superseded`.

Plan ref: Task 10.

## Pre-step
Inspect the items status state machine in `internal/items/` and `internal/store/` to confirm whether `superseded` is allowed. Choose the narrowest existing state if not.

## Acceptance criteria
- [ ] Original item file moved to `.squad/items/.archive/`.
- [ ] Original item row's status transitions to a valid superseded equivalent.
- [ ] `claim_history` row inserted with reason `intake_refine_superseded`.
- [ ] Multi-item bundle in refine mode → `IntakeIncomplete` (no archival happens).
- [ ] Refine mode rejected if original item is no longer `captured` (e.g., already claimed).

## Notes
This is reversible by hand (move file back, update row), but should not happen by accident. Tests must verify rollback on partial failure.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
