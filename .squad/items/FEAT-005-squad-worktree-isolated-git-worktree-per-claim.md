---
id: FEAT-005
title: squad worktree — provision an isolated git worktree per claim, clean on done
type: feature
priority: P1
area: cli
status: open
estimate: 6h
risk: high
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777250700
accepted_by: agent-401f
accepted_at: 1777250700
references:
  - cmd/squad/claim.go
  - cmd/squad/done.go
  - cmd/squad/handoff.go
  - cmd/squad/register.go
  - internal/claims/claims.go
  - internal/repo/repo.go
  - internal/store/migrations/001_initial.sql
relates-to:
  - TASK-019
  - BUG-019
blocked-by: []
---

## Problem

Two independent in-tree reports surfaced during the `feature-uptake-nudges`
epic close-out:

1. The TASK-016 spec reviewer flagged that running tests during their claim
   picked up uncommitted WIP from a peer agent in the same working tree —
   the test run reflected mixed state, not the reviewer's branch.
2. Agent-401f's handoff confirmed the same pattern from the other side: a
   sibling claim's pending edits flowed into the integration test the
   reviewer ran, producing a misleading green that didn't actually validate
   either claim's code in isolation.

Twice-confirmed, two angles. Today every agent on a repo claims into the
same single working tree; their edits, branches, and local state all
collide. The dogfood data was rescued by manual sequencing; that doesn't
scale, and parallelism is the explicit goal of `superpowers:dispatching-
parallel-agents`. The structural fix is one git worktree per claim.

The `agents` table already has a `worktree TEXT` column
(`internal/store/migrations/001_initial.sql:13`) that records where an agent
started — but nothing provisions it, nothing tears it down, and nothing
keeps two agents from claiming into the same tree.

## Context

- `cmd/squad/claim.go:71-105` (`Claim`): the library entrypoint. Today it
  inserts the claim row, returns. Knows nothing of the filesystem layout
  the agent will work in.
- `cmd/squad/claim.go:117-203`: cobra wrapper. Prints the three nudges, exits.
  No worktree side-effect.
- `internal/claims/claims.go`: claim store. Reads/writes the `claims` table.
  No worktree column today.
- `cmd/squad/done.go:71-143` (`Done`): releases the claim, rewrites the item
  file in done state, records commits. Nothing about worktree teardown.
- `cmd/squad/handoff.go:98-121` (`Handoff`): releases held claims, posts to
  chat. Same gap on teardown.
- `cmd/squad/register.go:206-224` (`upsertAgent`): writes the existing
  `agents.worktree` value to record where the session lives — that's a
  *snapshot of the agent's pwd*, not a per-claim isolated tree.
- `internal/repo/repo.go:15` (`Discover`): walks up to the git root from a
  start dir. The starting point for any worktree work.

No existing code shells out to `git worktree`. This is a net-new dependency
on the `git` binary at runtime — which squad already implicitly requires
(commit-recording in FEAT-002 already shells `git log`).

## Acceptance criteria

### Phase 1 — schema and library

- [ ] Migration `internal/store/migrations/00X_claim_worktree.sql` adds a
  `worktree TEXT` column to the `claims` table. Default empty string; old
  rows tolerated.
- [ ] New helper package `internal/worktree/` with:
  - `Provision(repoRoot, baseBranch, itemID, agentID string) (path, branch string, err error)`:
    creates `<repoRoot>/.squad/worktrees/<agentID>-<itemID>/`, runs
    `git worktree add -b squad/<itemID>-<agentID> <path> <baseBranch>`,
    returns the absolute path and branch name. Idempotent — re-running
    against an existing worktree returns the existing path and a
    `worktree.ErrExists` sentinel.
  - `Cleanup(repoRoot, path string) error`: runs `git worktree remove --force <path>`
    then `git branch -D squad/<branch>` if no commits flowed back. If commits
    DID land, leave the branch (the user/PR mechanism owns it). Verify with
    `git rev-list <branch> ^<baseBranch>` exit code.
  - `List(repoRoot string) ([]Worktree, error)`: parses `git worktree list --porcelain`,
    returns slice of paths with their HEAD shas. Used by `squad doctor`.
  - All commands run with `CGO_ENABLED=0`-safe `os/exec`. Handle
    repo-not-a-git-repo gracefully (return descriptive error, don't panic).

### Phase 2 — claim integration

- [ ] `cmd/squad/claim.go`: new `--worktree` flag (default off; opt-in for
  the first ship to limit blast radius). When set:
  - call `worktree.Provision` BEFORE inserting the claim row
  - record the path in the claim insert (extend `claims.Store.Claim` to
    accept a `WorktreePath string` option)
  - on success, print a fourth nudge line:
    `tip: cd into the isolated worktree: cd <path>`
  - on provision failure (e.g. uncommitted changes in main worktree, dirty
    base branch), abort the claim with a clear message and DO NOT insert
    the claim row.
- [ ] When `agent.default_worktree_per_claim: true` is set in
  `.squad/config.yaml`, `--worktree` is implied. New config field, defaults
  false.

### Phase 3 — done/handoff teardown

- [ ] `cmd/squad/done.go`: when the released claim has a non-empty
  `worktree` column, `worktree.Cleanup` runs after the commit-record step.
  Failure to clean (e.g. uncommitted local changes in the worktree) does
  NOT roll back the done — log a warning and leave the worktree for
  inspection. The squad doctor check (below) will surface it.
- [ ] `cmd/squad/handoff.go`: when releasing claims, run `Cleanup` for each.
  Same warn-on-failure semantics.

### Phase 4 — observability

- [ ] `cmd/squad/doctor.go` gains a check `worktree_orphan`: any directory
  under `<repoRoot>/.squad/worktrees/` not corresponding to an active claim
  is reported as a finding with the path and a suggested
  `git worktree remove` command.
- [ ] `internal/server/agents.go` and the SPA agents panel surface the
  per-claim worktree path (column already there for the agent-level value;
  extend the API with `claims[].worktree`). Useful for parallel sessions
  to see where peers are working.

### Phase 5 — tests

- [ ] `internal/worktree/worktree_test.go`:
  - `Provision` against a fixture git repo creates the directory and branch.
  - Re-provisioning the same `(itemID, agentID)` returns `ErrExists` and is
    idempotent.
  - `Cleanup` removes the dir and branch when no commits; preserves the
    branch when commits exist.
  - `List` parses real `git worktree list --porcelain` output.
- [ ] `cmd/squad/claim_worktree_test.go`: end-to-end claim with `--worktree`
  populates the column AND creates the directory; abort path leaves no
  trace in DB or filesystem.
- [ ] `cmd/squad/done_worktree_test.go`: clean teardown after done; warning
  on dirty teardown; uncommitted changes in worktree do NOT roll back done.
- [ ] `cmd/squad/doctor_worktree_test.go`: orphan detection with one
  active and one orphaned worktree directory.
- [ ] All tests use `t.TempDir()` + `git init` fixture repos. No reliance on
  the actual squad repo's git state.
- [ ] `go test ./... -race -count=1` passes; trailing summary pasted at
  close-out.

### Phase 6 — docs and skills

- [ ] `docs/concepts/claims-and-coordination.md` gains a section "Per-claim
  worktrees" describing the opt-in flow and the rationale (test isolation,
  parallel session safety).
- [ ] `docs/recipes/multi-agent-parallel-claude-sessions.md` updated to
  recommend `--worktree` (or the config flag) as the default for parallel
  setups, with the `cd <path>` step shown.
- [ ] New `docs/recipes/recovering-from-orphan-worktrees.md`: short recipe
  pointing at `squad doctor` and `git worktree remove`.
- [ ] Plugin skill `squad-loop` (or wherever the loop is documented) gets a
  one-line addition mentioning worktrees for parallel work.

## Notes

- **Why P1, risk:high.** P1 because shared-tree contamination is producing
  silent test passes — the failure mode is "agents think they're green
  when they aren't," which corrodes trust faster than any other failure
  mode. Risk:high because we are taking on a new operational dependency
  (real `git worktree` calls), changing the meaning of the `claims` table,
  and altering the expected pwd-discipline of every loop. Land it behind
  a flag for at least one full epic before flipping the default.
- **The `--worktree` flag is opt-in for this ship.** Do NOT change defaults
  in this item. A separate FEAT-006 (or a follow-up commit gated on
  dogfood usage data) flips the default to on once the orphan-cleanup
  story is exercised across at least 5 closed claims.
- **Cleanup edge cases worth thinking about up front:**
  - agent crash mid-claim: worktree leaks until `squad doctor` flags it.
    Acceptable for v1; the doctor finding makes it discoverable.
  - branch name collision (someone manually created `squad/<itemID>-<agent>`
    already): provision fails with a clear error. Don't auto-suffix; the
    user's intent is unclear.
  - base branch dirty: provision fails before any worktree work. Surface
    the same `git status` summary the user would see.
- **MCP parity (BUG-019 lineage).** The MCP claim handler must populate
  the same `Tips` field with the worktree path when `--worktree` is in
  effect. Don't forget the parity that BUG-019 is currently fixing.
- **Don't break solo flows.** `--worktree` opt-in means single-agent users
  who never set the flag see no behavior change. Their `done` paths run
  the cleanup ONLY when the column is non-empty.
- Future-work, NOT in this item: `squad worktree promote <ID>` to merge
  the claim's branch back into the base branch on close-out. Today the
  branch persists post-done if commits landed; the user owns the merge
  via PR. That's a deliberate scope hold.

## Resolution
(Filled in when status → done.)
