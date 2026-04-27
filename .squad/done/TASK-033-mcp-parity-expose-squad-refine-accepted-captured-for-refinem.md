---
id: TASK-033
title: 'MCP parity: expose squad refine (accepted → captured for refinement)'
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
accepted_at: 1777255894
references:
  - cmd/squad/refine.go
  - cmd/squad/mcp_register.go
  - cmd/squad/mcp_schemas.go
relates-to: []
blocked-by: []
---

## Problem

`squad refine` sends an accepted item back for refinement (writes the reviewer's notes under `## Reviewer feedback`, flips status to `needs-refinement`). The other half of the recapture loop. CLI-only today.

## Context

Cobra constructor: `cmd/squad/refine.go:21 newRefineCmd`. The intake-tool group in `mcp_register.go` is the right home (next to `squad_accept` / `squad_reject`).

There's a sibling endpoint `POST /api/items/{id}/refine` in the server (see `internal/server/items_refine.go`) — the MCP handler should reuse the same pure entry point so the two paths can't drift.

## Acceptance criteria

- [ ] `Refine(ctx, RefineArgs) (*RefineResult, error)` pure entry point in `cmd/squad/refine.go` (or shared with the server's call site).
- [ ] `squad_refine` MCP tool registered in `mcp_register.go` (intake group) with a JSON schema accepting `item_id` (required), `comments` (required, the reviewer feedback body), and `agent_id` (optional).
- [ ] Handler test in `cmd/squad/mcp_test.go` covering populated comments, empty/whitespace-only comments → 400-equivalent, and item-not-found.
- [ ] `mcp_test.go`'s expected-tools list includes `squad_refine`.
- [ ] `go test ./cmd/squad/... -count=1` passes.

## Notes

Comments body should be byte-identical to what `squad refine` would write to the file; no newline surgery in the MCP path that the CLI doesn't do.

## Resolution

(Filled in when status → done.)
