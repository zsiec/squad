---
id: TASK-001
title: body-rewrite parser — WriteFeedback / MoveFeedbackToHistory
type: task
priority: P2
area: items
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777242000
accepted_by: agent-1f3f
accepted_at: 1777242000
epic: inbox-refinement
references:
  - internal/items/walk.go
  - internal/items/parse.go
relates-to: []
blocked-by: []
---

## Problem

The inbox-refinement feature needs a pure-string body-rewrite helper that both the CLI and HTTP handlers call into. Without it, every caller would re-implement section-aware markdown manipulation and round-trip behavior.

## Context

Lives in `internal/items/refine.go` (new). Two functions:

- `WriteFeedback(body, comments string) string` — inserts (or replaces) a `## Reviewer feedback` section above `## Problem`. If no `## Problem` exists, prepend at top.
- `MoveFeedbackToHistory(body, date string) string` — moves the existing feedback section into `## Refinement history` (appending as `### Round N — YYYY-MM-DD`), removes the working feedback. No-op if no feedback section present.

Full spec + reference implementation: see Task 1 of `/Users/zsiec/dev/switchframe/docs/plans/2026-04-26-squad-inbox-refinement.md`.

## Acceptance criteria

- [x] `internal/items/refine.go` exists with `WriteFeedback` and `MoveFeedbackToHistory`.
- [x] `internal/items/refine_test.go` covers: insert above Problem, replace existing feedback, no-Problem prepends at top, trailing newline preserved, first-round move-to-history, append round 2 to existing history, no-op when no feedback present.
- [x] All 7+ tests pass; output pasted in the close-out chat.
- [x] No I/O in either function (pure string→string).

## Notes

Foundation for TASK-003, TASK-004, TASK-006, TASK-007. Land first.
