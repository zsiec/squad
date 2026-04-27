---
id: FEAT-015
title: 'intake: session open cancel and one-open-per-agent invariant'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 1h30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290812
accepted_by: web
accepted_at: 1777291151
references: []
relates-to: []
blocked-by: [FEAT-014]
---

## Problem
Interview sessions need a lifecycle: open or resume, cancel, and a guarantee that any one agent has at most one open session per repo at a time.

## Context
Sits in `internal/intake/session.go`. The partial unique index from FEAT-014 enforces the invariant at the DB level; this item provides the Go API on top.

Plan ref: Task 2 in `~/dev/switchframe/docs/plans/2026-04-27-intake-interview.md`.

## Acceptance criteria
- [ ] `Open(ctx, db, repoID, agentID, mode, ideaSeed)` returns existing open session for `(repo, agent)` if any (`resumed=true`); else creates fresh with id `intake-YYYYMMDD-<6-byte-hex>`.
- [ ] `Cancel(ctx, db, sessionID, agentID)` sets `status='cancelled'`. Returns `IntakeNotYours` / `IntakeAlreadyClosed` where appropriate.
- [ ] Tests: open-new is fresh; open-twice-same-agent resumes; open-different-agent-same-repo creates new; cancel marks closed; cancel-by-other-agent rejected.

## Notes
No new ID prefix needed — sessions live in their own table, not the items table.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
