---
id: BUG-004
title: web UI cannot view closed/past items; graph and stats views are broken
type: bug
priority: P2
area: web-ui
status: open
estimate: 2h
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-1f3f
captured_at: 1777238416
accepted_by: agent-1f3f
accepted_at: 1777238416
references: []
relates-to: []
blocked-by: []
---

## Problem

User-reported issues against the web dashboard SPA (`internal/server/web/`):

1. **No way to view closed / past items.** The board only surfaces in-flight or ready work — there is no view, filter, or tab for `done` items, so historical context (what was shipped, when, by whom) is invisible from the web UI.
2. **Graph view is broken.** The dependency-graph view does not render / does not work.
3. **Stats view is broken.** The stats / insights view does not render / does not work.

Need to verify whether the TUI has the same gap for #1 (closed-item visibility). Initial scan shows TUI views in `internal/tui/views/` include `history.go`, which suggests the TUI may already expose history; needs confirmation by running `squad tui` and checking the menu/keybindings.

## Context

The web SPA lives at `internal/server/web/` and is served by `internal/server/`. Relevant files:

- `board.js` — primary kanban-style view; check whether it filters out `done`.
- `depgraph.js` — dependency graph view (suspected broken).
- `insights.js` — stats/insights view (suspected broken).
- `sidebar.js` — left-nav; check whether a "history" / "closed" entry exists.

Backend endpoints in `internal/server/` already include `history.go`, `stats.go`, `items.go` — so the data is likely available; the gap may be purely on the frontend.

For TUI parity, see `internal/tui/views/history.go` and `internal/tui/views/items.go`.

## Acceptance criteria

- [ ] Reproduction steps captured for all three failures (closed-items invisibility, graph broken, stats broken) with browser console errors and/or network traces noted.
- [ ] Root cause identified for each — separate or shared.
- [ ] TUI parity check documented: confirm whether closed items are reachable from `squad tui`, and how (which view / keybinding).
- [ ] Fix lands so that: (a) the web UI exposes a way to view closed/past items, (b) the graph view renders correctly, (c) the stats view renders correctly.
- [ ] Tests cover the fix at the appropriate layer (server handler tests if the gap is backend; SPA smoke test or manual evidence pasted into the resolution if purely frontend).

## Notes

Filed during a session with the user — they reported all three symptoms together. Treat as one bug for triage; if root causes diverge during investigation, split into BUG-005/BUG-006 rather than ballooning this item.

## Resolution

### Reproduction
Headless browser test against `squad serve` (Playwright + system Chrome) on the dev workspace:
- **Closed items**: board renders only `In Progress / Ready / Blocked` tabs; no element queries `?status=done`. Confirmed via `tabs: ['In Progress 1', 'Ready 3', 'Blocked 0']`.
- **Graph**: modal opens, SVG renders, but visual is broken — viewBox is 250×140 while CSS `.depgraph-svg { min-width: 100%; min-height: 100%; }` stretches it to fill a 90vw × 90vh panel, scaling 2 nodes to comically large size. Same-layer `relates-to` edges drew loopy backward arcs because `edgePath` always assumed forward (left→right) routing.
- **Stats**: modal opens, two of three Chart.js charts render; the WIP-violations tile has an early `return` when `series` is empty, leaving the canvas untouched and the tile blank.

### Root causes
- **Closed items**: missing UI feature (no Done tab; backend `/api/items?status=done` already worked).
- **Graph**: (a) CSS `min-width/min-height: 100%` on `.depgraph-svg`; (b) `edgePath` had only one routing strategy (forward bezier).
- **Stats**: `drawWIP` returned silently on empty data instead of writing a fallback message to the canvas.

### TUI parity
`squad tui` items view counts done items in its filter band (`internal/tui/views/items.go:166`) and `[all]`/`[open]` filters include them (filterOpen returns `status != "captured"`, so done passes). No dedicated "done" filter, but reachable.

### Fix
- `internal/server/items.go` — surface `created`/`updated` in the list-row JSON (TDD: `TestItems_List_IncludesCreatedAndUpdated`).
- `internal/server/web/index.html` — added 4th board tab "Done".
- `internal/server/web/board.js` — done bucket sorted by `updated` desc; renders the date in the claim cell (taking precedence over any stale claim row).
- `internal/server/web/depgraph.js` — empty-state message when no nodes; `edgePath` now routes same-layer / backward edges via top↔bottom with a horizontal bow when columns coincide.
- `internal/server/web/style.css` — dropped `.depgraph-svg` `min-width/min-height: 100%`; added `.depgraph-empty`.
- `internal/server/web/insights.js` — `drawWIP` writes "no violations recorded" onto a properly-sized canvas instead of leaving it blank.

### Evidence
- `go test ./...` — all packages pass.
- `CGO_ENABLED=0 go build ./cmd/squad` — pure-Go build succeeds.
- Headless visual check (`/tmp/browser-test/test.js`): `tabs: ['In Progress 2', 'Ready 10', 'Blocked 0', 'Done 6']`; done tab lists 6 done items with their dates; graph renders 2 nodes/1 edge at intrinsic size with proper routing; stats tile shows "no violations recorded".
