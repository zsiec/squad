---
id: CHORE-004
title: agent_events.session_id and duration_ms written but unused downstream
type: chore
priority: P3
area: server
status: open
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777255606
accepted_by: web
accepted_at: 1777255753
references: []
relates-to: []
blocked-by: []
---

## Problem

The `agent_events` schema carries two columns that are written on every hook fire but consumed by nothing downstream:

- `session_id` — set by `cmd/squad/event.go` (defaults to `$SQUAD_SESSION_ID`); returned by the timeline endpoint in `internal/server/agent_events.go` but the SPA drawer (`internal/server/web/drawer.js`) does not render it, hygiene does not group by it, and no metrics aggregate it.
- `duration_ms` — populated by the post-tool hook from `tool_response.duration_ms` when jq is available; same downstream silence.

Either these columns are forward-looking schema (analytics / per-session filters / SLO panels not yet built) or they are dead weight that should be stripped to keep the table lean and the privacy surface small.

## Context

Code surface:
- Writer: `cmd/squad/event.go` (insert SQL), `plugin/hooks/post_tool_event.sh` (extracts duration_ms via jq).
- Migration: `internal/store/migrations/008_agent_events.sql`.
- Server: `internal/server/agent_events.go` returns these fields in the timeline payload.
- Consumers: none — verified by grep for `session_id` / `duration_ms` outside the writer/server surfaces.

Per CLAUDE.md, "Don't design for hypothetical future requirements" — but the columns are already shipped, so the cost is symmetric: keep and use, or strip and re-add later.

## Acceptance criteria

- [ ] Decide: build the consumer or strip the columns.
- [ ] If building: at minimum surface `duration_ms` as a tooltip on post_tool rows in the SPA drawer (TASK-027 timeline renderer is the natural seam) and add a session_id filter chip; otherwise document the decision in this item's Resolution.
- [ ] If stripping: add a follow-up migration that drops the two columns; update the writer SQL, hook script's jq extraction, and the server response shape; update the unit tests in `cmd/squad/event_test.go` and `internal/store/migrate_test.go`.

## Notes

P3, low priority — neither bug nor blocker. File it before the team forgets the columns exist.

## Resolution

Decision: BUILD the consumer rather than strip. Both fields surfaced via the timeline endpoint and the SPA drawer:

- `internal/server/agent_events.go` — `timelineRow` gained `duration_ms` and `session_id` (omitempty); the agent_events subquery now selects both.
- `internal/server/web/agent_timeline.js` — post_tool event rows render a `<span class="tl-duration">` showing ms / s with sub-second precision; row gets `data-session=` so a session-aware filter can match. A `<select class="tl-session">` is rendered above the timeline only when ≥2 distinct sessions are observed (single-session is the common case; the picker would be noise). Selection persists to `localStorage['squad.timeline.filters'].session`.
- `internal/server/web/style.css` — `.tl-duration` + `.tl-session` styles.

Strip path was rejected because the columns are correctly populated, the privacy surface is bounded (session_id is a session env var, not PII; duration_ms is millisecond integer), and the build cost was lower than a follow-up migration + writer + hook + server + 2 test files.
