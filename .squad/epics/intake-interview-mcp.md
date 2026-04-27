---
spec: intake-interview
status: open
parallelism: |
  Single-task epic. Depends on intake-interview-core being complete.
---

## Goal

Register four MCP tools — `squad_intake_open`, `squad_intake_turn`,
`squad_intake_status`, `squad_intake_commit` — on the existing MCP server.
Each handler decodes args, calls `internal/intake`, and maps `intake.Error`
values to `mcp.ToolError` with appropriate JSON-RPC error codes.

## Scope

- One handler per tool with arg/result types matching the design doc §3.
- Error mapping: `IntakeNotFound` → not found, `IntakeNotYours` → invalid request,
  `IntakeIncomplete` / `IntakeShapeInvalid` → invalid params, `IntakeSlugConflict`
  → conflict, etc.
- JSON-RPC roundtrip tests using the existing `mcp_register_test.go` scaffold.

## Out of scope

- A `squad_intake_cancel` tool. By design, Claude does not have a tool for
  throwing user work away — cancel is CLI-only.
