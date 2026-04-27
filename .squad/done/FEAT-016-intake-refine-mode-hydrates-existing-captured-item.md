---
id: FEAT-016
title: 'intake: refine mode hydrates existing captured item'
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
blocked-by: [FEAT-015]
---

## Problem
`squad intake refine <id>` needs to load the existing captured item into the interview as starting context so the agent can ask "what's missing" instead of starting from scratch.

## Context
Extends `internal/intake/session.go`. When `mode='refine'`, Open verifies the item exists and is `captured`, then returns an `ItemSnapshot{ID, Title, Body, Area, ...}` in the open result.

Plan ref: Task 3.

## Acceptance criteria
- [ ] `Open` with `mode='refine'` and `refineItemID` set returns `ItemSnapshot` populated from disk.
- [ ] Open rejects refine mode if the item doesn't exist (`IntakeNotFound`) or isn't in `captured` status.
- [ ] Tests: hydrated snapshot on happy path; rejection of missing item; rejection of claimed item; rejection of done item.

## Notes
Snapshot is a value, not a pointer — interview reads the snapshot, not the live row.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
