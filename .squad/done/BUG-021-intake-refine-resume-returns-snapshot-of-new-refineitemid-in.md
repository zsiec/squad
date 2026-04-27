---
id: BUG-021
title: intake refine resume returns snapshot of new RefineItemID instead of session's pinned item
type: bug
priority: P3
area: internal/intake
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777293708
accepted_by: web
accepted_at: 1777293922
references: []
relates-to: []
blocked-by: []
---

## Problem
`intake.Open(ctx, db, OpenParams{Mode: ModeRefine, RefineItemID: X, ...})` reuses an existing open session for (repo, agent) but loads `ItemSnapshot` from the *current call's* `RefineItemID`, not the session's persisted one. So if the original session was opened against `Y`, the resumed call returns `Session.RefineItemID=Y` and `ItemSnapshot.ID=X` — silent divergence.

## Context
Surfaced during code review of FEAT-016 (`internal/intake/session.go:91-104`): `loadItemSnapshot` runs before `findOpen`, so the resume path never re-aligns. Refine sessions are conceptually pinned to one item — switching mid-session should not be implicit.

## Acceptance criteria
- [ ] When `findOpen` returns a resumed session and the call's `RefineItemID` differs from the session's persisted `refine_item_id`, return a typed error (suggest `ErrIntakeRefineItemMismatch`) instead of a silently-mismatched snapshot.
- [ ] Test: open refine for FEAT-A, then call Open with `RefineItemID=FEAT-B` for the same (repo, agent) → expect the new sentinel error.

## Notes
Alternative (lenient) fix: re-load snapshot using the session's persisted `refine_item_id` instead of the call's value. Strict-error variant preferred per code-reviewer recommendation — silently swapping the item under the agent is too surprising.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
