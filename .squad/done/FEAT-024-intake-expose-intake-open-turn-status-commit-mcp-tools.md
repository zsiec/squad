---
id: FEAT-024
title: 'intake: expose intake_open/turn/status/commit MCP tools'
type: feature
priority: P2
area: cmd/squad/mcp
parent_spec: intake-interview
parent_epic: intake-interview-mcp
status: done
estimate: 1h30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290819
accepted_by: web
accepted_at: 1777291152
references: []
relates-to: []
blocked-by: [FEAT-016, FEAT-018, FEAT-022, FEAT-023]
---

## Problem
Claude needs MCP-callable tools to drive the interview. The four tools wrap `internal/intake` and translate errors into JSON-RPC error codes.

## Context
New file `cmd/squad/mcp_intake_interview.go`. Registers `squad_intake_open`, `squad_intake_turn`, `squad_intake_status`, `squad_intake_commit` on the existing MCP server alongside the other intake-family tools (see `cmd/squad/mcp_intake.go` for the existing capture/refine/decompose tools — different scope).

Plan ref: Task 11.

## Acceptance criteria
- [ ] All four tools registered with arg/result types matching design doc §3.
- [ ] Error mapping: `IntakeNotFound` → not-found; `IntakeNotYours` → invalid-request; `IntakeIncomplete` and `IntakeShapeInvalid` → invalid-params; `IntakeSlugConflict` → conflict; `IntakeAlreadyClosed` → invalid-request.
- [ ] No `squad_intake_cancel` tool — by design, Claude does not have a tool for throwing user work away.
- [ ] JSON-RPC roundtrip tests per tool, including error mapping. Reuse `mcp_register_test.go` scaffold.

## Notes
Decompose still lives in its own file/registration — do not collapse the two. The interview tools are a separate concern.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
