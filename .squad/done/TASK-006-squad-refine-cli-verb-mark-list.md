---
id: TASK-006
title: squad refine CLI verb (mark + list)
type: task
priority: P2
area: cli
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242005
accepted_by: agent-1f3f
accepted_at: 1777242005
epic: inbox-refinement
references:
  - cmd/squad/reject.go
  - cmd/squad/inbox.go
relates-to: []
blocked-by:
  - TASK-001
---

## Problem

CLI parity for the inbox refinement loop. `squad refine <ID> --comments "..."` flips a captured item to `needs-refinement`. `squad refine` with no args lists items in that status.

## Context

New file `cmd/squad/refine.go`. Mirror `cmd/squad/reject.go` for the marking path and `cmd/squad/inbox.go` for the list path. Calls `items.WriteFeedback` directly (no need to round-trip through HTTP). Registered in the rootCmd builder.

Full reference in Task 6 of the implementation plan.

## Acceptance criteria

- [x] `squad refine <ID> --comments "..."` writes `## Reviewer feedback` into the item body and sets `status: needs-refinement` in frontmatter.
- [x] `squad refine <ID>` without `--comments` (or with empty string) errors with a message mentioning `--comments`.
- [x] `squad refine` with no args lists `needs-refinement` items in oldest-first order; format mirrors `squad inbox`.
- [x] 3 CLI tests in `cmd/squad/refine_test.go`; output pasted.

## Notes

Depends on TASK-001 (parser).
