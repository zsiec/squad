---
id: TASK-035
title: 'MCP parity: expose squad standup (per-agent activity summary)'
type: task
priority: P2
area: mcp
status: open
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-1f3f
captured_at: 1777255765
accepted_by: web
accepted_at: 1777255908
references:
  - cmd/squad/standup.go
  - cmd/squad/mcp_register.go
  - cmd/squad/mcp_schemas.go
relates-to:
  - TASK-025
blocked-by: []
---

## Problem

`squad standup` produces a per-agent activity rollup (claims taken, claims done, commits, attestations) for a recent window. Useful in `squad-handoff` and CI/standup-bot contexts; CLI-only today.

## Context

Cobra constructor: `cmd/squad/standup.go:52 newStandupCmd`. The agent-activity-stream epic just landed `/api/agents/:id/timeline` (TASK-025) which produces a richer per-agent rollup; standup is the *summary* version. The inspection-tool group in `mcp_register.go` is the right home.

## Acceptance criteria

- [ ] `Standup(ctx, StandupArgs) (*StandupResult, error)` pure entry point in `cmd/squad/standup.go`.
- [ ] `StandupResult` is a structured payload (per-agent counts + samples) — not a pre-formatted string.
- [ ] `squad_standup` MCP tool registered in `mcp_register.go` with a JSON schema accepting `since` (optional unix ts; defaults to last 24h), `agent_id` (optional, scope to one agent), and a caller-id `agent_id`.
- [ ] Handler test in `cmd/squad/mcp_test.go` covering populated and empty-window paths.
- [ ] `mcp_test.go`'s expected-tools list includes `squad_standup`.
- [ ] `go test ./cmd/squad/... -count=1` passes.

## Notes

If the standup CLI already shells out to the timeline endpoint internally, the MCP handler should call the same Go function — not the HTTP API.

## Resolution

(Filled in when status → done.)
