---
id: FEAT-071
title: code reviewer working tree hygiene check and blocking finding field
type: feature
priority: P3
area: review
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352045
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

`superpowers:code-reviewer` is the highest-leverage subagent in
the system — 239 dispatches in 7d, by far the most-used Agent
type. Two recurring polish gaps surface across recent sessions:

1. The reviewer's "empirically verify each finding" pass
   sometimes patches files temporarily to test a behavior. The
   skill prompt asks the reviewer to restore them, but there's
   no mechanical check; a missed restoration leaks into the
   author's working tree.

2. Reviewer findings use `Critical / High / Medium / Low` but
   whether each finding is *blocking* vs. *non-blocking* is
   buried in prose ("ship it after H1+M1 land", "non-blocking
   M2"). `squad done` can't auto-detect "review has open
   blockers" without natural-language parsing of the report.

Both are small UX fixes; both compound across hundreds of reviews.

## Context

Surface to touch:

- `plugin/skills/superpowers/code-reviewer/` (or wherever the
  reviewer prompt lives — locate exact path).
- The reviewer agent's response format / skill prompt.
- Optionally: `cmd/squad/done.go` consumes the structured
  blocking field if present.

For (1): after the reviewer finishes, run `git status --short`
inside the worktree and refuse the review report if any new files
or modifications are present that weren't there at start. Capture
the start state in a sentinel file or via `git stash list` length.
The exact mechanism is a design call — what matters is that an
unrestored scratch edit is rejected before the report comes back.

For (2): require each finding line to include `blocking: true` or
`blocking: false`. The reviewer's prompt names this; the parser in
`squad done` (if it exists) reads the field. If `squad done` doesn't
parse review output today, this item only updates the prompt — the
parsing is downstream.

## Acceptance criteria

- [ ] After a reviewer subagent completes, the squad runtime
      asserts the worktree is clean of any modifications the
      reviewer introduced (no `.bak` files, no stash entries
      added during review). On dirty exit, the review fails with
      a message naming the offending paths.
- [ ] Each reviewer finding in the report carries an explicit
      `blocking: true|false` field. The skill prompt mandates it;
      a finding without it is a malformed report.
- [ ] The reviewer skill prompt is updated to define
      "blocking" precisely: a finding is blocking if it would
      cause data loss, security regression, contract violation,
      or test failure on merge. Style-only nits are non-blocking.
- [ ] A test (or stub-driven scenario) exercises both gates: a
      reviewer that leaves a `.bak` file fails the cleanliness
      gate; a reviewer that omits the `blocking` field fails the
      schema gate.

## Notes

- Don't try to make `squad done` auto-block on findings yet —
  the explicit `blocking` field is the *enabling* change for that.
  Consuming it is a follow-up if/when desired.
- Risk: the cleanliness gate could false-positive against
  legitimate worktree changes that happened concurrently from
  the parent session. Worktree-per-claim isolation should make
  this rare; revisit if it bites in practice.

## Resolution
(Filled in when status → done.)
