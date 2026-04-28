---
id: FEAT-063
title: If there is an idle agent, there should be a way in the SPA for me to click and drag an item to that agent and have it start working on it.
type: feature
priority: P1
area: web
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: web
captured_at: 1777345774
accepted_by: web
accepted_at: 1777346349
references: []
relates-to: []
blocked-by: []
auto_refined_by: claude
auto_refined_at: 1777345896
---

## Problem
The SPA has no way to assign a specific ready item to a specific idle agent. Today work is pull-only: an agent must run `squad next` / `squad claim` itself, so an operator watching the dashboard cannot direct an idle agent to a particular item without dropping into a terminal.

## Context
The SPA already renders an agent rail with per-agent state ("active" / "idle" / "stopped") in `internal/server/web/board.js` (`renderAgents`, around line 269) and a ready-items table in the same file (around line 127). The only server path that creates a claim is the pull-style `POST /api/items/{id}/claim` in `internal/server/claim.go`; there is no operator-driven dispatch endpoint. The web layer is vanilla JS with no drag-and-drop dependency, so the implementation is HTML5 native DnD plus a new HTTP handler.

## Acceptance criteria
- [ ] A new HTTP endpoint accepts (item id, agent id), writes a claim row owned by that agent for that item on success, and is covered by a Go unit test asserting the persisted claim's `agent_id` and `item_id`.
- [ ] The same endpoint returns a 4xx error and writes no claim row when the target item is already claimed, blocked, or already done; this conflict path is covered by a Go unit test.
- [ ] In the SPA, while a ready-item card is being dragged, agent rail rows whose state is "idle" render a visible drop-target affordance (e.g. highlighted border/background); rows whose state is "active" or "stopped" show no affordance and reject the drop.
- [ ] Dropping a ready-item card onto an idle agent row issues exactly one request to the new endpoint; the resulting claim becomes visible in the SPA via the existing SSE stream within 2 seconds without a manual page refresh.
- [ ] Dropping a ready-item card anywhere other than an idle agent row (empty space, board background, a non-idle agent) is a no-op: no HTTP request is sent and the item remains in the Ready tab.

## Resolution
(Filled in when status → done.)
