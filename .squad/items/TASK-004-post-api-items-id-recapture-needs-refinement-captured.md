---
id: TASK-004
title: POST /api/items/{id}/recapture — needs-refinement → captured
type: task
priority: P2
area: server
status: open
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-1f3f
captured_at: 1777242003
accepted_by: agent-1f3f
accepted_at: 1777242003
epic: inbox-refinement
references:
  - internal/server/items_accept.go
  - internal/server/server.go
relates-to: []
blocked-by:
  - TASK-001
  - TASK-002
---

## Problem

Add the recapture endpoint the refining agent calls when their edit pass is finished. Releases the agent's claim, moves `## Reviewer feedback` into `## Refinement history`, flips status back to `captured`.

## Context

New file `internal/server/items_recapture.go`; route `POST /api/items/{id}/recapture` in `server.go`. Requires `X-Squad-Agent` header. Refuses unless the calling agent currently holds the claim. Calls `items.MoveFeedbackToHistory(body, today)`, persists, deletes the claim row, emits SSE `inbox_changed` with `action: "recapture"`.

Full reference in Task 4 of the implementation plan.

## Acceptance criteria

- [ ] Happy path on a held `needs-refinement` claim returns 204; `## Reviewer feedback` is gone, `## Refinement history` exists with `### Round N`, status flips to `captured`, claim row deleted.
- [ ] No claim or wrong agent → 403.
- [ ] Round 2: existing `## Refinement history` is preserved; new round appended.
- [ ] `inbox_changed` SSE fires with `action: "recapture"`.
- [ ] 3 tests in `internal/server/items_recapture_test.go`; output pasted.

## Notes

Pairs with TASK-003. Both depend on TASK-001 (parser) and TASK-002 (status enum).
