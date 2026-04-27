---
id: FEAT-018
title: 'intake: session status returns transcript and gaps'
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
accepted_at: 1777291151
references: []
relates-to: []
blocked-by: [FEAT-015, FEAT-017]
---

## Problem
Resuming a session across Claude restarts requires a single call that returns the full transcript + checklist gaps + session metadata.

## Context
Extends `internal/intake/session.go` with `Status(ctx, db, sessionID, agentID) (StatusResult, error)`. Backed by SELECTs against `intake_sessions` + `intake_turns ORDER BY seq`.

Plan ref: Task 5.

## Acceptance criteria
- [ ] Returns full transcript ordered by `seq`, current `still_required`, and session metadata.
- [ ] Rejects calls from agents who don't own the session (`IntakeNotYours`).
- [ ] Tests: full transcript returned; still_required computed correctly; foreign agent rejected.

## Notes
Read-only — no side effects. Distinct from `intake_turn` which mutates.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
