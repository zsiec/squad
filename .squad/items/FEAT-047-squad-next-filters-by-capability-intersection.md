---
id: FEAT-047
title: squad next filters by capability intersection
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
blocked-by: [FEAT-045, FEAT-046]
parent_spec: agent-team-management-surface
epic: capability-routing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Once items declare `requires_capability` (FEAT-045) and agents
register a capability set (FEAT-046), the ready stack still shows
every ready item to every agent. The router is the missing piece:
an agent should never see a ready item it cannot claim.

## Context

`cmd/squad/next.go` runs the priority-ordered ready iteration —
`internal/items/walk.go` (or its current equivalent) is the source of
truth for which items are ready and in what order. The filter slots
in after the ready set is materialized: for each candidate, check
that `item.RequiresCapability ⊆ agent.Capabilities`.

Subset, not equality. An agent with `{go, sql, frontend}` should see
items requiring `{go}` and items requiring `{go, sql}`. An item with
empty `requires_capability` is universally claimable and stays
visible.

`squad next --all` is the inspection escape hatch. Operators
debugging the stack need to see every ready item regardless of fit;
without this flag the filter becomes a black box. The flag does not
change claim eligibility — `squad claim <ID>` on a mismatched item
should still fail at the claim layer (or at minimum print a warning;
exact behavior to confirm during implementation).

## Acceptance criteria

- [ ] `squad next` omits items whose `requires_capability` is not a
  subset of the calling agent's capability set.
- [ ] Items with empty `requires_capability` remain visible to every
  agent.
- [ ] `squad next --all` returns the unfiltered ready stack.
- [ ] Test covers: agent with subset capabilities, agent with
  superset capabilities, agent with disjoint capabilities, and the
  `--all` bypass.
- [ ] Filter is a Go-level intersection; no SQL changes needed.

## Notes

Priority ordering is preserved — the filter removes items, it does
not re-rank. If the top-priority ready item requires capabilities
the calling agent lacks, `squad next` returns the next-priority item
the agent can actually claim, not nothing.

## Resolution

(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
