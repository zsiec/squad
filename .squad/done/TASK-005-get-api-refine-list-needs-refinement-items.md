---
id: TASK-005
title: GET /api/refine — list needs-refinement items
type: task
priority: P2
area: server
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242004
accepted_by: agent-1f3f
accepted_at: 1777242004
epic: inbox-refinement
references:
  - internal/server/inbox.go
  - internal/server/server.go
relates-to: []
blocked-by:
  - TASK-002
---

## Problem

The SPA and the new `squad refine` (no-args) CLI both need a list endpoint to surface items in `needs-refinement` status.

## Context

Mirror `internal/server/inbox.go`. New file `internal/server/refine_list.go`. Route `GET /api/refine` in `server.go`. Returns rows where `status='needs-refinement'` ordered by `updated_at ASC`. Shape: `id`, `title`, `captured_by`, `captured_at`, `refined_at` (= `updated_at`).

Full reference in Task 5 of the implementation plan.

## Acceptance criteria

- [x] `GET /api/refine` returns 200 with the array of needs-refinement entries, sorted oldest-first by `updated_at`.
- [x] Items in other statuses (`captured`, `open`, `done`, `rejected`) are NOT included.
- [x] At least one test in `internal/server/refine_list_test.go` covers list + sort + filter; output pasted.

## Notes

Depends only on the status enum (TASK-002).
