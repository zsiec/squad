---
id: TASK-034
title: 'MCP parity: expose squad analyze (epic decomposition)'
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
accepted_at: 1777255896
references:
  - cmd/squad/analyze.go
  - internal/analyze/
  - cmd/squad/mcp_register.go
  - cmd/squad/mcp_schemas.go
relates-to: []
blocked-by: []
---

## Problem

`squad analyze` decomposes an epic into a parallel-friendly task graph. Without MCP exposure, agents can only consume the analysis by piping the CLI output back through the model.

## Context

Cobra constructor: `cmd/squad/analyze.go:16 newAnalyzeCmd`. Logic in `internal/analyze/`. The inspection-tool group in `mcp_register.go` is the right home — output is read-only.

## Acceptance criteria

- [ ] `Analyze(ctx, AnalyzeArgs) (*AnalyzeResult, error)` pure entry point in `cmd/squad/analyze.go`.
- [ ] `AnalyzeResult` is a structured payload (per-task rows, dependency edges, parallel-stream groups) — not a pre-formatted string.
- [ ] `squad_analyze` MCP tool registered in `mcp_register.go` with a JSON schema accepting `epic_name` (required), `agent_id` (optional).
- [ ] Handler test in `cmd/squad/mcp_test.go` covering a real fixture epic and the unknown-epic error path.
- [ ] `mcp_test.go`'s expected-tools list includes `squad_analyze`.
- [ ] `go test ./cmd/squad/... ./internal/analyze/... -count=1` passes.

## Notes

The cobra wrapper's pretty-print stays CLI-only. MCP returns the raw graph for the model to reason over.

## Resolution

(Filled in when status → done.)
