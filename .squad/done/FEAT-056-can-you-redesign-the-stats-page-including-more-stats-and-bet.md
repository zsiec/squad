---
id: FEAT-056
title: can you redesign the stats page including more stats and better visualization?
type: feature
priority: P1
area: web
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: web
captured_at: 1777344924
accepted_by: web
accepted_at: 1777345106
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777344994
auto_refined_by: claude
---

## Problem
The dashboard stats panel (`internal/server/web/insights.js`) renders only three tiles — verification rate, claim p99, WIP violations — out of the much richer `/api/stats` payload (`internal/stats/schema.go`). Operators have to drop to the CLI to see item-mix, agent leaderboards, epic/capability breakdowns, learnings, and token estimates, and the existing tiles use a flat layout with no scope filter or window control.

## Context
- API: `internal/server/stats_api.go` already accepts `?window=`; the `Snapshot` struct exposes `Items`, `Claims` (with full Percentiles), `Verification.ByKind`, `Learnings`, `Tokens`, `ByAgent`, `ByEpic`, `ByCapability`, plus the existing `Series.*` daily series.
- Frontend: `internal/server/web/insights.js` (panel + Chart.js wiring), styles in `internal/server/web/style.css` under `.insights-*`.
- Prior art: `internal/server/prometheus.go` lists every metric the backend already publishes; the redesign should not add new server-side fields, only consume what's there.

## Acceptance criteria
- [ ] The stats panel renders at least 8 distinct tiles backed by `/api/stats` (existing 3 plus item-status mix, claim duration percentile bars P50/P90/P99, verification by-kind, top-N agent leaderboard, and either by-epic or by-capability breakdown).
- [ ] A window selector in the panel header lets the user switch between 24h, 7d, and 30d, re-fetching `/api/stats?window=…` and re-rendering all tiles in place without a full reload.
- [ ] Tiles that have no data render an explicit empty-state ("no data") instead of a blank canvas or a chart.js error in the console.
- [ ] The agent leaderboard tile shows agent_id, claims_completed, release_count, ratio, and verification_rate, sorted by claims_completed descending, capped at the top 10 rows.
- [ ] The redesigned panel does not introduce new fields to the `stats.Snapshot` struct or new endpoints — `git diff internal/stats/ internal/server/stats_api.go` is empty for additive schema changes.
- [ ] A new test in `internal/server/web/` (or extension of an existing SPA test) asserts the panel mounts, calls `/api/stats`, and renders the tile container for each new tile id.
- [ ] `go test ./internal/server/... ./internal/stats/...` passes; `golangci-lint run` is clean.

## Notes
- Keep Chart.js as the only charting dep; no new CDN scripts beyond the ones already loaded in `ensureChartJs`.
- Respect the existing `.insights-grid` / `.insights-tile` styling system; if more density is needed, extend the grid rather than introducing a parallel layout.
- The TUI has its own stats view (`internal/tui/views/`) and is out of scope for this item — file a follow-up if redesign parity is wanted there.

## Resolution
(Filled in when status → done.)
