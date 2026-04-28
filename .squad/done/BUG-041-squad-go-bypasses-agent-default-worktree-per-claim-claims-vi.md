---
id: BUG-041
title: squad go bypasses agent.default_worktree_per_claim — claims via direct store call without worktree provisioning
type: bug
priority: P2
area: cmd/squad
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777331966
accepted_by: agent-afcd
accepted_at: 1777332106
references: []
relates-to: []
blocked-by: []
---

## Problem

`squad go` (the recommended one-command flow per the squad-loop skill
and `CLAUDE.md` "Resume a session") does not honor
`agent.default_worktree_per_claim: true`. Setting the config and
running `squad go` claims the next ready item but no worktree is
provisioned. The same item, claimed via `squad claim <ID>` (no
`--worktree` flag), correctly reads the config and creates the
worktree. The two paths diverge silently.

## Context

Reproduce on this repo:

```
$ grep default_worktree_per_claim .squad/config.yaml
  default_worktree_per_claim: true

$ squad go             # → claims the next ready item
$ git worktree list    # → only the parent repo; no .squad/worktrees/agent-afcd-<ID>
```

Then:

```
$ squad release <ID>
$ squad claim <ID> --intent "..."   # no --worktree flag
$ git worktree list    # → /Users/zsiec/dev/squad/.squad/worktrees/agent-<id>-<ID> on its own branch
$ # squad claim also prints "tip: cd into the isolated worktree: ..."
```

Root cause is in `cmd/squad/go.go:108-110`:

```go
err := bc.store.Claim(context.Background(), it.ID, bc.agentID,
    "squad go auto-claim", nil, false,
    claims.ClaimWithPreflight(bc.itemsDir, bc.doneDir))
```

`squad go` calls the claim store directly with the worktree-arg
(fifth positional, here `false`) hard-coded, skipping the flag
resolution that `cmd/squad/claim.go:192` does:

```go
useWorktree := worktreeFlag || worktreeDefault()
```

`worktreeDefault()` (`cmd/squad/claim.go:267`) reads
`agent.default_worktree_per_claim`. Only the explicit `squad claim`
verb consults it; `squad go` passes `false` regardless.

## Acceptance criteria

- [ ] `squad go` consults `agent.default_worktree_per_claim` (and
      any future `--worktree` override at the verb level) before
      calling the claim store; when true, the claim provisions a
      worktree under `.squad/worktrees/<agentID>-<itemID>` on a
      dedicated branch, exactly like `squad claim` does.
- [ ] After a `squad go` claim, `git worktree list` shows the new
      worktree and `squad claim ... --worktree`'s "cd into the
      isolated worktree" tip is printed (or its equivalent for
      `squad go`).
- [ ] `squad done` cleans up the worktree on close, same as today's
      `squad claim --worktree` flow.
- [ ] An integration test in `cmd/squad/` covers: config flips
      default to true → `squad go` claims item → worktree exists.
      A second sub-test pins the override path (config off, no
      worktree).

## Notes

Cheapest fix: have `runGo` reuse `worktreeDefault()` (or extract a
shared helper) and pass the resolved bool to `bc.store.Claim`. The
worktree.Provision call lives in `claim.go:114` — likely worth
extracting into a small helper so both verbs share it.

Discovered while smoke-testing the worktree-per-claim default
landing — `squad claim CHORE-016` worked, `squad go` on the same
config did not.
