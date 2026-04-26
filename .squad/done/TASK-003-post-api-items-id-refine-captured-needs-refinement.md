---
id: TASK-003
title: POST /api/items/{id}/refine — captured → needs-refinement
type: task
priority: P2
area: server
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242002
accepted_by: agent-1f3f
accepted_at: 1777242002
epic: inbox-refinement
references:
  - internal/server/items_accept.go
  - internal/server/items_reject.go
  - internal/server/server.go
relates-to: []
blocked-by:
  - TASK-001
  - TASK-002
---

## Problem

Add a new HTTP endpoint that flips a captured (or already-needs-refinement) item to `needs-refinement` and writes the reviewer's comments to the item body via `items.WriteFeedback`.

## Context

Mirror `internal/server/items_accept.go`. New file at `internal/server/items_refine.go`. Wire route at `POST /api/items/{id}/refine` in `internal/server/server.go`. Validate non-empty `comments`, validate status is `captured` or `needs-refinement`, persist file + DB row, emit SSE `inbox_changed` with `action: "refine"`.

Full handler reference + tests in Task 3 of the implementation plan.

## Acceptance criteria

- [x] `POST /api/items/{id}/refine` returns 204 on captured input, with body persisted (`## Reviewer feedback` section present near top of body) and DB `status='needs-refinement'`.
- [x] Empty `comments` → 422.
- [x] Wrong status (e.g. `open`) → 422.
- [x] `inbox_changed` SSE event fires with `action: "refine"` on success.
- [x] 4 tests in `internal/server/items_refine_test.go` cover the above; output pasted.

## Notes

Land after TASK-001 and TASK-002.
