---
id: TASK-027
title: SPA timeline view rendering with kind filters + localStorage persistence
type: task
priority: P2
area: spa
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: 2026-04-27
captured_by: agent-bbf6
captured_at: 1777251709
accepted_by: agent-bbf6
accepted_at: 1777251709
epic: agent-activity-stream
references:
  - internal/server/web/agents.js
  - internal/server/web/style.css
relates-to:
  - TASK-026
blocked-by:
  - TASK-026
---

## Problem

The drawer scaffolding exists (TASK-026) but renders nothing in the content area. This item plugs in the actual timeline render: read events from the cached fetch, lay them out chronologically with appropriate icons / colors per kind, and provide kind-filter toggles persisted to localStorage.

## Context

The drawer's `renderTimeline(events)` callback (placeholder from TASK-026) is the integration point. Each event row's shape comes from `GET /api/agents/:id/events`: `{ts, event_kind, tool, target, exit_code, duration_ms}`. The "timeline" view should ALSO consume the unified `/api/agents/:id/timeline` endpoint (chat + claims + commits + attestations + events) for a full picture — but this item can render either or both depending on what the drawer fetches in TASK-026.

## Acceptance criteria

- [ ] `renderTimeline(items)` populates the drawer content area with one DOM element per item, ordered by ts (most recent first or oldest first — pick whatever feels right and stay consistent).
- [ ] Each item renders the relevant fields:
  - chat verb → verb badge + body text
  - claim/release/done → lifecycle badge + summary if present
  - commit → SHA prefix + subject
  - attestation → kind badge + command
  - event → tool name + truncated target + exit/duration if non-zero
- [ ] Filter UI above the timeline: a row of toggleable chips for each kind (chat, claim, commit, attestation, pre_tool, post_tool, subagent_start, subagent_stop).
- [ ] Defaults:
  - `pre_tool` for `Read` tool: hidden by default
  - all other tool kinds and lifecycle kinds: visible
- [ ] Toggle state persists to `localStorage['squad.timeline.filters.<agent_id>']` (or per-user, not per-agent — pick one and document) so the next drawer-open restores it.
- [ ] Empty state: "no activity yet — agent is registered but hasn't done anything we record."
- [ ] Loading state: spinner inherited from TASK-026.
- [ ] Error state: "couldn't load timeline" + retry button if the fetch fails.
- [ ] Manual smoke: open the drawer, see events render, toggle off `Read`, see Read rows disappear, refresh the page, drawer reopens with `Read` still off (localStorage worked).
- [ ] No tests required (JS/SPA), but document the smoke-test steps in close-out.

## Notes

- The filter persistence key: per-agent vs per-user is a design call. Per-user is simpler; per-agent supports "always show Read for agent X but not Y" use cases. Lean per-user unless there's a concrete reason.
- Don't pre-build virtual scrolling — initial fetch is `limit=50`, drawer opens once, no perf concern. If a future item needs virtualized rendering, add then.
- Match the existing SPA's icon set (likely no icon library — just emoji or unicode glyphs). Don't introduce a font.

## Resolution

(Filled in when status → done.)
