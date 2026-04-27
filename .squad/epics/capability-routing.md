---
title: Capability routing
spec: agent-team-management-surface
status: open
parallelism: serial
dependencies:
  - Coordination defaults — opinionated, opt-out
intake_session: intake-20260427-44256e4424c4
---

## Goal

Replace the flat priority queue with per-claim capability matching. Items
declare which capability tags an agent must hold to claim them; agents
register their capability set on `squad register`; the ready stack and
`squad next` only surface items whose `requires_capability` is a subset of
the claiming agent's capabilities.

This is the structural shift from "any ready item is fair game for any
agent" to "ready items are typed, and the ready set you see is filtered
to the work you can actually do."

## Why serial

The four items build a single linear stack:

1. **Schema lands first** — the items column and frontmatter parser are
   the foundation everything else reads from.
2. **CLI verbs build on schema** — `squad register --capability` needs the
   tag concept and the agent-side persistence path established before it
   can store anything meaningful.
3. **Filter logic builds on CLI verbs** — `squad next` cannot intersect an
   item's tags with an agent's tags until both sides exist.
4. **Stats build on the schema** — `squad stats --by capability` reads the
   items column directly and is independent of the filter, but still
   needs the column to exist.

## Items

- **FEAT-045** — items gain `requires_capability` frontmatter and DB column.
- **FEAT-046** — `squad register` accepts repeatable `--capability` flag.
- **FEAT-047** — `squad next` filters by capability intersection.
- **FEAT-048** — `squad stats --by capability`.

## Dependency on coordination defaults

The prior epic establishes per-claim worktree isolation — the primitive
that makes parallel typed work safe. Capability routing assumes that
foundation: once two agents with disjoint capabilities can each claim
without stepping on each other, the routing problem becomes "show each
agent the work it can actually do."

## Success signal

An agent registered with capabilities `{go, sql}` runs `squad next` and
sees only items whose `requires_capability` is a subset of `{go, sql}`.
Items with empty `requires_capability` remain visible to every agent.
A second agent registered with `{frontend, design}` sees a disjoint
slice of the same ready stack. `squad stats --by capability` shows a
nonzero count under `go`, `sql`, `frontend`, `design` once any
capability-tagged items have been completed.
