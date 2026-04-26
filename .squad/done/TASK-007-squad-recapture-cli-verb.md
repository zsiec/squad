---
id: TASK-007
title: squad recapture CLI verb
type: task
priority: P2
area: cli
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242006
accepted_by: agent-1f3f
accepted_at: 1777242006
epic: inbox-refinement
references:
  - cmd/squad/done.go
  - cmd/squad/release.go
relates-to: []
blocked-by:
  - TASK-001
---

## Problem

The refining agent needs a verb to send their edited item back to the inbox: claim must be released, `## Reviewer feedback` must be moved to history, status flips back to `captured`. Done's evidence-gates and done-dir-move semantics are wrong here, so `done` cannot be reused.

## Context

New file `cmd/squad/recapture.go`. Mirror `cmd/squad/done.go` structurally (claim-required, mutates state, prints confirmation), but skip the evidence gate and the file move. Calls `items.MoveFeedbackToHistory(body, today)`, releases the claim, rewrites frontmatter status.

Full reference in Task 7 of the implementation plan.

## Acceptance criteria

- [x] Happy path: `squad recapture <ID>` (with held claim, item in `needs-refinement`) produces a body with `## Reviewer feedback` removed, `## Refinement history` containing the round, frontmatter `status: captured`. Claim released.
- [x] No-claim case errors with a message mentioning "claim".
- [x] 2 CLI tests in `cmd/squad/recapture_test.go`; output pasted.

## Notes

Depends on TASK-001 (parser). Pairs with TASK-006.
