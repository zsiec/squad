---
id: CHORE-016
title: smoke test for worktree-per-claim isolation
type: chore
priority: P2
area: meta
status: done
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777331887
accepted_by: agent-afcd
accepted_at: 1777331905
references: []
relates-to: []
blocked-by: []
---

## Problem

`agent.default_worktree_per_claim: true` was just enabled in
`.squad/config.yaml`. Confirm the config actually drives worktree
creation by claiming a real item and observing an isolated worktree
materializes under `.squad/worktrees/`.

## Context

End-to-end smoke. Not a code change — pure operational check.

## Acceptance criteria

- [ ] `squad claim CHORE-016` (or `squad go` if it picks this item)
      creates a worktree under `.squad/worktrees/<agent>-CHORE-016`.
- [ ] The worktree contains a dedicated branch and a working copy
      separate from the parent repo's tree.
- [ ] Closing the item via `squad done` cleans up the worktree.
