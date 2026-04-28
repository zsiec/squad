---
id: BUG-056
title: stats panel not wide enough — needs horizontal scroll on narrower viewports
type: bug
priority: P2
area: web
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-310b
captured_at: 1777351347
accepted_by: web
accepted_at: 1777351401
references: []
relates-to: []
blocked-by: []
---

## Problem

The stats panel (opened via the SPA's STATS button) is not wide
enough on the viewports operators actually use — content overflows
the visible width and the panel currently does not horizontal-scroll,
so tiles are clipped instead of being reachable.

## Context

The panel is rendered by `internal/server/web/insights.js` into
`<aside class="insights-panel">` (style at
`internal/server/web/style.css`). The 8-tile grid landed in FEAT-056;
the redesign assumed a wide-enough side panel but the actual width is
constrained by `aside` rules. The leaderboard tile (`insights-tile-wide`
spans 2 columns) is the most visible cliff — it gets clipped before
anything else.

Reported by the operator after the FEAT-056 redesign + CHORE-019
race fix — the data is correct; the layout is the issue.

## Acceptance criteria

- [x] On a viewport narrower than the natural grid width, the
      `.insights-panel` body horizontal-scrolls instead of clipping
      tile content (the user can reach every tile and read every
      summary line).
- [x] No regression on wide viewports — when there is room, the grid
      lays out at full width without an unwanted scrollbar.
- [x] Tile internals (canvas, leaderboard table) stay legible at the
      intended layout dimensions; horizontal scroll moves the
      whole grid, not individual tiles.

## Notes

- Cheapest likely fix: add `overflow-x: auto` to the panel's body
  container plus a `min-width` on `.insights-grid` matching its
  natural 2-column layout. Vertical scroll already works.
- Touches `internal/server/web/style.css` (the `.insights-panel`
  / `.insights-grid` / `.insights-body` rules). No JS change
  expected.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
