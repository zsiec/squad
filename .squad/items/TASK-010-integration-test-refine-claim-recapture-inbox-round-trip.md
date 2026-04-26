---
id: TASK-010
title: integration test — refine → claim → recapture → inbox round-trip
type: task
priority: P2
area: server
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-1f3f
captured_at: 1777242009
accepted_by: agent-1f3f
accepted_at: 1777242009
epic: inbox-refinement
references:
  - internal/server/items_accept_test.go
  - internal/server/inbox_changed_test.go
relates-to: []
blocked-by:
  - TASK-003
  - TASK-004
  - TASK-005
---

## Problem

Need an end-to-end integration test that exercises the full loop: a captured item is sent for refinement, the refining agent claims, recaptures, the item lands back in the inbox.

## Context

New file `internal/server/integration_refine_test.go`. Single test function `TestIntegration_RefineRoundTrip` that POSTs through every endpoint, asserts `/api/refine` lists the item between refine and recapture, and `/api/inbox` lists it again after recapture.

Also includes manual UI smoke validation by booting `squad serve --port 9999 --token test --bind 127.0.0.1` and running through the steps in Task 10 of the implementation plan, with screenshots or log paste in the close-out.

## Acceptance criteria

- [ ] `internal/server/integration_refine_test.go` exists, single passing test that round-trips refine → list → claim → recapture → inbox.
- [ ] `go test ./... -count=1 -race` passes; trailing summary pasted.
- [ ] Manual UI smoke notes pasted in the close-out chat (button visible, composer focuses, item disappears, recapture restores item to inbox).

## Notes

Lands after TASK-003, TASK-004, TASK-005, and ideally after TASK-008/TASK-009 so the manual smoke covers UI too.
