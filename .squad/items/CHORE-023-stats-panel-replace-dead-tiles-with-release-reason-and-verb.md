---
id: CHORE-023
title: stats panel replace dead tiles with release reason and verb usage
type: chore
priority: P3
area: spa
status: open
estimate: 2h
risk: low
evidence_required: []
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352112
references: []
relates-to: []
blocked-by: [CHORE-020, FEAT-066]
epic: polish-and-prune-from-usage-data
---

## Problem

The dashboard stats grid (FEAT-056) renders tiles whose underlying
metrics will be zero-forever after the polish-and-prune removals:

- "Blocked items" — count goes to zero permanently when the
  `blocked` status drops (FEAT-066).
- "Refinement queue depth" — goes to zero when the peer-queue
  refine path drops (FEAT-067, parallel item).
- Any tile keyed off `knock` / `answer` chat-verb counts (if
  any) — zero after CHORE-020.

Two replacement tiles fit the new shape better:

1. **Release reasons (last 7d)**: stacked bar showing the
   `released | superseded | blocked | abandoned` breakdown
   from the new enum (FEAT-066 dependency).
2. **Verbs in use (last 7d)**: histogram of chat verbs with
   counts. Operator-facing signal that complements the
   doctor's feature-usage report (FEAT-069).

## Context

Touch the SPA stats page (`internal/server/web/insights.js` and
the matching CSS / template). The backend `internal/stats/`
package supplies snapshots; if the new tiles need new aggregation
queries, add them as additive fields on the existing snapshot
shape.

## Acceptance criteria

- [ ] The "Blocked items" tile is removed from the stats grid
      after FEAT-066 lands.
- [ ] The refinement-queue tile is removed (or its data source
      replaced) after FEAT-067 lands.
- [ ] A new "Release reasons" tile renders the four-way enum
      breakdown over the active window.
- [ ] A new "Verbs in use" tile renders the chat-verb histogram
      (use the operator-facing window selector consistent with
      the rest of the panel).
- [ ] Structural Go test pins the new tile IDs in the embedded
      SPA bytes (same `webFS.ReadFile` pattern as the existing
      stats-tile tests added in FEAT-056).
- [ ] The grid layout doesn't visually regress at the existing
      breakpoints; a manual viewport check + screenshot in the
      resolution notes.

## Notes

- Blocked-by chain: CHORE-020 + FEAT-066 must land before this
  produces meaningful output. FEAT-067 affects only one tile
  removal; this can ship paralleled with it.
- Pure SPA work modulo the optional snapshot fields. No state-
  machine changes.

## Resolution
(Filled in when status → done.)
