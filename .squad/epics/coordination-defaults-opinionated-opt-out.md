---
title: Coordination defaults — opinionated, opt-out
spec: agent-team-management-surface
status: open
parallelism: parallel
dependencies: []
intake_session: intake-20260427-44256e4424c4
---

## Goal

Flip the built coordination primitives — per-claim worktrees and the
pre-edit touch ledger — from opt-in to default-on, so a fresh `squad init`
produces a config that prevents the multi-agent collision pattern observed
in the dogfood session of 2026-04-27 without requiring any per-agent
ceremony.

## Observation

The audit on 2026-04-27 found two primitives in the tree that nobody is
using in practice:

- `--worktree` on `squad claim` provisions an isolated checkout under
  `.squad/worktrees/<agent>-<item>/` (`cmd/squad/claim.go:255`,
  `internal/worktree/`). The store migration at
  `internal/store/migrate.go:bootstrapLegacyVersions` already protects
  the `claims.worktree` column, but no agent in the dogfood session
  passed the flag and `agent.default_worktree_per_claim` is false out
  of the box (`internal/config/config.go:53`).
- The `pre_edit_touch_check.sh` hook is registered on PreToolUse for
  Edit/Write (`plugin/hooks.json:102-110`) and reads peer conflicts via
  `touch.Tracker.Conflicts`, but the `touches` table only gets rows
  when an agent explicitly runs `squad touch` or passes `--touches` on
  claim. In the audited session no agent did either, so the table
  stayed empty and the hook had nothing to warn about.

The shape of the bug is the same in both cases: shipped primitive,
default-off knob, zero practical use.

## Items in this epic

- **CHORE-014** — flip `default_worktree_per_claim` to true in the
  scaffold's `.squad/config.yaml` so fresh repos start with isolation
  on. Existing repos are untouched.
- **FEAT-033** — actually populate the touches table from Edit/Write
  hooks so the conflict reads in `touches_policy.go:46` have something
  to find. Today the policy verb only reads; nothing writes from the
  hook path.
- **FEAT-034** — surface peer-touch overlaps at `squad claim` time as a
  one-line warning, so the agent sees collision risk before they edit.
  Depends on FEAT-033 because there is no useful query before touches
  get written automatically.
- **FEAT-035** — break the flat `claimed: N` line in `squad status`
  (`cmd/squad/status.go:119`) into a per-agent breakdown when claimed
  > 1, so an operator can see which agents are holding what.

## Success signal

A fresh `git clone` followed by `squad init` produces a `.squad/config.yaml`
that turns on per-claim worktrees, with a hook stack that records every
Edit/Write to the touches table and a `squad claim` that warns on
peer overlap. Two agents running `squad go` against the same fresh repo
should see isolation by default and a coordination warning before they
collide on a file — neither of which happened in the 2026-04-27 dogfood
session.
