---
id: BUG-051
title: squad done leaves worktree branch orphaned — commits never reach main
type: bug
priority: P1
area: cmd/squad
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-afcd
captured_at: 1777336648
accepted_by: web
accepted_at: 1777336809
references: []
relates-to: []
blocked-by: []
---

## Problem

The worktree-per-claim flow (`agent.default_worktree_per_claim: true`)
creates a branch `squad/<ID>-<agent>` and provisions a worktree at
`.squad/worktrees/<agent>-<ID>` on that branch. The agent commits
their work to that branch, then runs `squad done <ID>` from the
parent repo. `squad done` rewrites the item file to `status: done`,
moves it into `.squad/done/`, releases the claim — but **never
folds the branch's commits back into `main`**. The work is left
orphaned on `squad/<ID>-<agent>`, which:

1. Means the actual code change never lands on `main`. Any
   subsequent `go install ./cmd/squad` (or CI build, or release)
   produces a binary that lacks the fix that was supposedly "done."
2. Leaves a growing pile of `squad/*` branches on the repo. After
   N items, `git branch` is unreadable noise.
3. Forces every contributor or peer who needs the change to manually
   `git cherry-pick` the worktree's HEAD onto `main`. We've already
   seen this — agent-bbf6 cherry-picked the BUG-045 fix into main
   (commit `325b026`), but missed BUG-041's fix entirely (it sat on
   `squad/BUG-041-agent-afcd` until I cherry-picked it manually
   while diagnosing the present bug).

The user-visible symptom that surfaced this: after closing BUG-041
("squad go bypasses worktree default"), `squad go` still didn't
provision worktrees because the binary on PATH was rebuilt from
`main`, which didn't have my fix. The fix existed only on the
worktree's branch.

## Context

Reproduce against this repo, post-rebuild:

```
$ git log --all --oneline | grep BUG-041
eabcc38 fix(go): honor agent.default_worktree_per_claim on the squad go path

$ git log main --oneline | grep -c BUG-041   # before manual cherry-pick
0

$ git branch | grep squad/
  squad/BUG-041-agent-afcd
  squad/BUG-043-agent-afcd
  squad/BUG-045-agent-afcd
  ...
```

Looking at `cmd/squad/done.go` (or wherever the squad-done flow
lives): no `git merge` / `git rebase` / `git cherry-pick` step
exists. The worktree itself gets cleaned up via `worktree.Cleanup`
but the branch is not deleted and not folded.

## Acceptance criteria

- [ ] `squad done <ID>` (when the item was claimed via worktree)
      folds the worktree's branch into `main` — fast-forward when
      possible, falling back to a merge commit (or rebase, design
      choice for the implementer) when the branch has diverged.
      The user does not need to remember to `git cherry-pick`.
- [ ] On successful fold, the per-claim branch is deleted (via
      `git branch -d squad/<id>-<agent>`).
- [ ] On a merge conflict, `squad done` exits non-zero with a
      clear message naming the branch and instructing the user
      to resolve manually — the item file is not yet moved to
      `.squad/done/` (or `--force` overrides). State stays
      recoverable.
- [ ] A test in `cmd/squad/` exercises the worktree-claim → commit
      → done flow end-to-end against a fixture git repo and
      asserts (a) the commit is on `main` after `squad done`, (b)
      the per-claim branch is gone, (c) the worktree dir is gone.
- [ ] `squad release <ID>` (without close-out) leaves the branch
      and worktree alone — only `squad done` triggers the fold.

## Notes

Design questions worth chewing on at refinement time:

1. **Fast-forward, merge, or rebase?** Fast-forward is cleanest
   when nobody else has advanced `main` while the worktree was in
   flight. Merge commits add noise but tolerate divergence. Rebase
   rewrites the worktree's history but produces linear log. Squad
   doctrine ("frequent small commits beat one big one") leans
   merge / FF; rebase loses the commit-author timing.
2. **What about `release --outcome released`?** That's the
   handoff path, not done. Branch should stay so the next claimer
   can resume the work.
3. **Multiple commits on the worktree branch.** The fold needs to
   bring all of them, not just HEAD. A naive `git cherry-pick`
   would only get one — `git merge --ff-only` brings the whole
   chain.
4. **What about non-worktree claims?** `squad done` from the
   parent repo (no worktree) currently works because the user
   commits directly to `main`. The new logic must no-op when no
   worktree branch exists for the closing item.

This bug has been actively biting the team — BUG-041 sat unmerged,
agent-bbf6 had to cherry-pick BUG-045, and the workflow is silently
broken for every worktree-per-claim user. P1 because every closed
worktree-claim item is a potential "fix landed on a branch nobody
sees."
