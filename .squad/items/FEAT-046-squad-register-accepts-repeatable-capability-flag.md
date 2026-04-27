---
id: FEAT-046
title: squad register accepts repeatable --capability flag
type: feature
priority: P2
area: cmd/squad
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309559
references: []
relates-to: []
blocked-by: [FEAT-045]
parent_spec: agent-team-management-surface
epic: capability-routing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Agents have no way to declare what they can do. Without a per-agent
capability set, the ready-stack filter in FEAT-047 has nothing to
intersect against — every agent looks identical to the router.

## Context

`cmd/squad/register.go` defines the existing register command and
writes the agent row through `internal/identity`. The agents table
(`internal/store/migrations/001_initial.sql`) has columns for id,
repo, display name, worktree, pid, timestamps, and status — no
capability column today.

FEAT-045 adds the items-side column only. This item owns the
agent-side schema change as well: a follow-up migration that adds
`agents.capabilities TEXT NOT NULL DEFAULT '[]'`, plus the CLI flag
plumbing and `whoami` rendering. Bundling them keeps the agent-side
story in one item; splitting the migration off would create a
half-finished CLI flag with nowhere to write.

Set semantics: re-registering with a new `--capability` set replaces
the prior set. Append-on-re-register would silently accumulate stale
tags and is the wrong default — registration is the agent declaring
its current shape, not a history of every shape it has ever had.

## Acceptance criteria

- [ ] Migration `011_agents_capabilities.sql` adds
  `agents.capabilities TEXT NOT NULL DEFAULT '[]'` with the matching
  marker probe in `bootstrapLegacyVersions`.
- [ ] `squad register --capability go --capability sql` persists
  `["go","sql"]` on the agent row.
- [ ] Re-running `squad register --capability frontend` replaces the
  set; the agent row reads `["frontend"]`, not `["go","sql","frontend"]`.
- [ ] `squad whoami` renders the registered capability set (empty set
  prints as `(none)` or equivalent).
- [ ] Test covers register-then-re-register and the empty-set case.

## Notes

Tag values are free-form strings, lowercased on input. No central
registry of allowed tags — the operator chooses their own taxonomy
(`go`, `sql`, `frontend`, `design`, `planning`, etc.). FEAT-047's
intersection logic treats unknown tags as opaque, which keeps the
feature usable without a config step.

## Resolution

(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
