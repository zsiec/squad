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

## Resolution

### Test

`internal/server/integration_refine_test.go` — `TestIntegration_RefineRoundTrip` exercises the full server-side loop:

1. Seed a captured `FEAT-950`.
2. POST `/api/items/FEAT-950/refine` with comments → `204`, status flips to `needs-refinement`.
3. GET `/api/refine` → list contains `FEAT-950`.
4. POST `/api/items/FEAT-950/claim` (with `X-Squad-Agent: agent-zzzz`) → `200`.
5. POST `/api/items/FEAT-950/recapture` (same agent) → `204`, claim row gone, status back to `captured`.
6. GET `/api/inbox` → list contains `FEAT-950` again.

### Evidence

```
$ go test ./... -count=1 -race
... ok github.com/zsiec/squad/internal/server (...)
ok  github.com/zsiec/squad/templates/github-actions  1.335s
(0 FAIL lines)
```

### UI smoke

Deferred until TASK-008 lands the Refine button + composer wiring in `internal/server/web/inbox.js`. TASK-009's CSS already exists; once TASK-008 plugs the markup and POST calls in, the manual smoke (button visible → composer focuses → item disappears from inbox → recapture restores it) can run against `squad serve` end-to-end. The backend round-trip is fully covered by `TestIntegration_RefineRoundTrip` in the meantime.

### AC verification

- [x] `internal/server/integration_refine_test.go` exists, single passing test that round-trips refine → list → claim → recapture → inbox.
- [x] `go test ./... -count=1 -race` passes; trailing summary above.
- [ ] Manual UI smoke — deferred until TASK-008 lands the SPA wiring (see UI smoke note above).
