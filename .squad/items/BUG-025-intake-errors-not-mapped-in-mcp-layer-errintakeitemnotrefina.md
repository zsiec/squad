---
id: BUG-025
title: intake errors not mapped in MCP layer (ErrIntakeItemNotRefinable surfaces as -32603 Internal)
type: bug
priority: P3
area: intake
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777298722
accepted_by: web
accepted_at: 1777298901
references: []
relates-to: []
blocked-by: []
---

## Problem
Two typed sentinels in `internal/intake/session.go` — `ErrIntakeItemNotRefinable` and `ErrIntakeRefineItemMismatch` — fall through `intakeErrToToolError` and surface to MCP callers as generic `-32603 Internal`. Both are user-input errors (caller passed a bad `refine_item_id` or a mismatched resume id), so the agent / skill cannot distinguish "you sent bad input" from "the server crashed."

## Context
The mapping function lives at `cmd/squad/mcp_intake_interview.go:174`. It already handles `ErrIntakeNotFound`, `ErrIntakeNotYours`, `ErrIntakeAlreadyClosed`, `IntakeSlugConflict`, `IntakeIncomplete`, and `IntakeShapeInvalid`. The two refine-mode sentinels were added later (see `internal/intake/session.go:31-32`) but never wired into the switch, so they hit the catch-all `return err` and the JSON-RPC layer renders them as `-32603 Internal`. `IntakeIncomplete` and `IntakeShapeInvalid` already map to `CodeInvalidParams`, which is the right code here too — both refine-mode errors mean the caller sent something the server can't accept.

The wrapped form returned by `commit_run.go:224` and `session.go:158` is `fmt.Errorf("%w: %s is %q", ErrIntakeItemNotRefinable, ...)`, so any test or mapping must match via `errors.Is`, not pointer equality.

## Acceptance criteria
- [ ] `intakeErrToToolError` maps `intake.ErrIntakeItemNotRefinable` to `mcp.CodeInvalidParams`.
- [ ] `intakeErrToToolError` maps `intake.ErrIntakeRefineItemMismatch` to `mcp.CodeInvalidParams`.
- [ ] A test in `cmd/squad/` directly exercises `intakeErrToToolError` for both sentinels — including the `fmt.Errorf("%w: ...", ...)` wrapped form — and asserts the resulting `*mcp.ToolError.Code` is `CodeInvalidParams`.
- [ ] `go test ./cmd/squad/... ./internal/intake/... ./internal/mcp/...` passes.

## Notes
Per the existing comment block on `intakeErrToToolError`: untyped errors intentionally fall through to `-32603`, and new mappings are only added for typed sentinels in the intake package. Both errors here are already typed sentinels, so this is purely a wiring gap.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
