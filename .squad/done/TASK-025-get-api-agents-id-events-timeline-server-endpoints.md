---
id: TASK-025
title: GET /api/agents/:id/events + /timeline server endpoints
type: task
priority: P1
area: server
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777251706
accepted_by: agent-bbf6
accepted_at: 1777251706
epic: agent-activity-stream
references:
  - internal/server/
  - internal/store/
relates-to:
  - TASK-021
blocked-by:
  - TASK-021
---

## Problem

The SPA needs a read API to render the agent-detail drawer. Two endpoints: `GET /api/agents/:id/events` for the raw event stream, `GET /api/agents/:id/timeline` for a unified rollup including chat verbs, claims, commits, attestations, and events. Without these, the drawer cannot render.

## Context

Existing handlers live under `internal/server/`. Look at how `GET /api/agents` is implemented as the pattern (likely in `agents.go` or `handlers.go`). Routes are registered in the router setup function — find it via `grep -n "/api/agents" internal/server/`. The DB connection is plumbed through; the table from TASK-021 is queryable directly.

## Acceptance criteria

- [ ] `GET /api/agents/:id/events` returns JSON `{"events": [...], "next_cursor": <ts or null>}`:
  - Each event row: `{ts, event_kind, tool, target, exit_code, duration_ms, session_id}`.
  - Query param `?since=<ts>` returns rows with `ts >= since` (incremental polling).
  - Query param `?limit=<n>` (default 100, max 500). Default ordering: ts DESC for the initial fetch (most recent first). With `?since=`, ts ASC (oldest first within the window) for natural append.
  - 404 if the agent isn't registered. Empty `events: []` if the agent exists but has no events.
- [ ] `GET /api/agents/:id/timeline` returns a unified rollup:
  - `{"timeline": [{"ts", "kind": "chat"|"claim"|"release"|"done"|"commit"|"attestation"|"event", ...}, ...]}` sorted by ts.
  - Same `?since=` and `?limit=` semantics.
  - 404 same as above.
  - **Implementation note:** UNION across `messages`, `claims`, `claim_history`, `commits`, `attestations`, `agent_events` filtered by agent_id. Pagination via cursor on (ts, source-discriminator) so cross-table ties are stable.
- [ ] Both endpoints return 400 with a clear error body on malformed `since` / `limit`.
- [ ] Handler tests in `internal/server/handlers_test.go` (or wherever the existing agent-handler tests live):
  - Empty agent → empty response, not 500.
  - Populated agent → expected rows, ordering correct.
  - `?since=` cuts the response correctly.
  - `?limit=` caps the response correctly.
  - Cross-source ts-tie ordering is deterministic.
- [ ] `go test ./internal/server/...` passes.

## Notes

- No SSE in this item — that's TASK-028. These endpoints are HTTP request/response only.
- Don't expose any data not already exposed elsewhere in the API — these endpoints are projections of existing tables.
- The `events` endpoint is straightforward; the `timeline` rollup is the harder query. If you find yourself writing a 200-line SQL UNION, extract a helper in `internal/agentstream/` or similar.
- Auth: the existing `/api/*` endpoints have whatever auth the server is configured with. Inherit; don't add a new auth check.

## Resolution

(Filled in when status → done.)
