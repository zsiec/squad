---
id: TASK-030
title: 'MCP parity: expose squad ready (DoR lint + promote-to-open)'
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
accepted_at: 1777255809
references:
  - cmd/squad/ready.go
  - cmd/squad/mcp_register.go
  - cmd/squad/mcp_schemas.go
relates-to: []
blocked-by: []
---

## Problem

`squad ready` runs the Definition-of-Ready lint and (if it passes) promotes a captured item to `open`. CLI-only — agents using MCP can't trigger the same flow without shelling out.

## Context

Cobra constructor: `cmd/squad/ready.go:19 newReadyCmd`. The pure entry point should be extracted (matching the pattern in `Claim`/`Done`/`Handoff`) so MCP can call it without touching cobra.

Current MCP registry: `cmd/squad/mcp_register.go` (registerLifecycleTools / registerIntakeTools — `ready` is intake-shaped, so register it there). Schemas live in `cmd/squad/mcp_schemas.go`.

## Acceptance criteria

- [ ] `Ready(ctx, ReadyArgs) (*ReadyResult, error)` pure entry point in `cmd/squad/ready.go`, mirroring `Claim`/`Done`. The cobra wrapper calls it.
- [ ] `squad_ready` MCP tool registered in `mcp_register.go` (intake group) with a JSON schema in `mcp_schemas.go`.
- [ ] Tool returns the structured DoR-lint findings (per-rule pass/fail) plus the new status when promotion happened.
- [ ] Handler test in `cmd/squad/mcp_test.go` covering the happy path and the DoR-fail-doesn't-promote path.
- [ ] `mcp_test.go`'s expected-tools list includes `squad_ready`.
- [ ] `go test ./cmd/squad/... -count=1` passes.

## Notes

Tips parity per BUG-019: if the cobra wrapper prints any nudge to stderr, the MCP handler should surface it via `Tips []string`.

## Resolution

(Filled in when status → done.)
