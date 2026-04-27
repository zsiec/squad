---
id: CHORE-014
title: default_worktree_per_claim true in squad init scaffold
type: chore
priority: P2
area: cmd/squad
status: done
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309556
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: coordination-defaults-opinionated-opt-out
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Make worktree-per-claim the default for fresh repos so multi-agent
sessions get isolation without an opt-in step. Today the migration at
`internal/store/migrate.go:bootstrapLegacyVersions` protects
`claims.worktree`, the `--worktree` flag exists on `squad claim`
(`cmd/squad/claim.go:255`), and the config knob exists at
`internal/config/config.go:57` — but `default_worktree_per_claim` ships
false and no agent in the audited dogfood session opted in.

## Context

The scaffold template at
`internal/scaffold/templates/config.yaml.tmpl` writes the project's
default `.squad/config.yaml` and currently has no `default_worktree_per_claim`
key at all, so fresh repos inherit the Go zero value (false). Flipping
the default needs to happen in the template — `internal/scaffold/write.go:12`
calls `writeIfAbsent` so existing repos are not touched on a re-run of
`squad init`. The AGENTS.md scaffold at
`internal/scaffold/templates/AGENTS.md.tmpl` should mention the default
and the per-claim opt-out (`squad claim --worktree=false` is not yet a
flag — opt-out is by editing the config or by leaving the per-claim
isolated checkout alone, which is fine).

## Acceptance criteria

- [ ] `squad init` writes `default_worktree_per_claim: true` into
      `.squad/config.yaml` (under the existing `agent:` block in
      `internal/scaffold/templates/config.yaml.tmpl`).
- [ ] Existing repos are unaffected: `writeIfAbsent` keeps a
      pre-existing config intact, and no migration backfills the key.
- [ ] AGENTS.md scaffold mentions the default and how to opt out
      (set `agent.default_worktree_per_claim: false` in
      `.squad/config.yaml`).

## Notes

The behavior is already plumbed end-to-end via `worktreeDefault()` at
`cmd/squad/claim.go:259` — this item is purely a scaffold default flip
plus the AGENTS.md callout. No code path changes.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
