---
id: TASK-031
title: 'MCP parity: expose squad stats (operational statistics)'
type: task
priority: P2
area: mcp
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-1f3f
captured_at: 1777255765
accepted_by: web
accepted_at: 1777255891
references:
  - cmd/squad/stats.go
  - internal/stats/
  - cmd/squad/mcp_register.go
  - cmd/squad/mcp_schemas.go
relates-to: []
blocked-by: []
---

## Problem

`squad stats` reports operational metrics (verification rate, claim p99, WIP cap usage). MCP callers can't read the same numbers without shelling.

## Context

Cobra constructor: `cmd/squad/stats.go:17 newStatsCmd`. Underlying queries live in `internal/stats/`. MCP layer: `cmd/squad/mcp_register.go` (inspection group is the right home — `squad_status`, `squad_who`, `squad_doctor` live there).

## Acceptance criteria

- [ ] `Stats(ctx, StatsArgs) (*StatsResult, error)` pure entry point in `cmd/squad/stats.go`. The cobra wrapper calls it.
- [ ] `StatsResult` is a structured payload (verification rate, claim latency p50/p99, WIP cap, current open claims, ghost-agent count) — not a pre-formatted string.
- [ ] `squad_stats` MCP tool registered in `mcp_register.go` (inspection group) with a JSON schema in `mcp_schemas.go`.
- [ ] Handler test in `cmd/squad/mcp_test.go` covering populated and empty-repo paths.
- [ ] `mcp_test.go`'s expected-tools list includes `squad_stats`.
- [ ] `go test ./cmd/squad/... ./internal/stats/... -count=1` passes.

## Notes

The CLI's pretty-print formatting stays in the cobra wrapper. MCP returns raw numbers.

## Resolution

(Filled in when status → done.)
