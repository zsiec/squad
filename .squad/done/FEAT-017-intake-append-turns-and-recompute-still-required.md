---
id: FEAT-017
title: 'intake: append turns and recompute still_required'
type: feature
priority: P2
area: internal/intake
parent_spec: intake-interview
parent_epic: intake-interview-core
status: done
estimate: 1h
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
Each interview turn must be persisted with monotonic ordering, and the server-tracked `still_required` checklist gap set must shrink as the agent claims fields.

## Context
New file `internal/intake/turns.go`. Turns are appended within the same SQLite tx that increments `seq` (max+1 in tx). `still_required` is computed by walking the shape's required-set and subtracting the union of all `fields_filled` across prior turns.

Plan ref: Task 4.

## Acceptance criteria
- [ ] `AppendTurn(ctx, db, sessionID, agentID, role, content, fieldsFilled)` returns `(seq, stillRequired, err)`.
- [ ] `seq` is monotonic and unique per session (DB unique constraint enforces).
- [ ] `still_required` strictly shrinks (or stays equal) as `fields_filled` accumulates.
- [ ] Validates role ∈ {user, agent, system}; rejects empty content; rejects appends to closed sessions.
- [ ] Tests: monotonic seq; shrinking still_required; empty content rejected; closed session rejected.

## Notes
Honor system on `fields_filled` — squad doesn't NLU the content. Real gate is structural validation at commit (FEAT-019).

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
