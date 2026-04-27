---
id: FEAT-010
title: wire bootstrap into squad mcp boot path
type: feature
priority: P1
area: mcp
epic: first-run-dashboard
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290336
accepted_by: web
accepted_at: 1777290581
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by:
  - FEAT-009
  - FEAT-011
  - FEAT-012
---

## Problem

`bootstrap.Ensure`, `bootstrap.Welcome`, and the banner glue all exist after the prior items, but nothing calls them. Until the boot path of `squad mcp` invokes them, the user-facing change is zero.

## Context

`cmd/squad/mcp.go` boots a stdio JSON-RPC server, registers tools, and starts serving. The bootstrap call belongs after tool registration setup but before `server.Serve` blocks on stdin. Banner plumbing requires a hook on `internal/mcp/server.Server` (most likely a `SetBanner(string)` setter that `callTool` reads-and-clears).

Failures from `Ensure` must not block tool serving. A user with a broken launchd config or a bound-port collision should still be able to use squad's CLI surface via MCP — they just won't get the UI.

## Acceptance criteria

- [ ] `cmd/squad/mcp.go` calls `bootstrap.Ensure(ctx, opts)` after tool registration, before `server.Serve`
- [ ] `Ensure` failure path: log a one-line warning to stderr (e.g., `squad: dashboard auto-install skipped: <err>`), continue serving
- [ ] `cmd/squad/mcp.go` calls `bootstrap.Welcome(ctx)` after a successful `Ensure` (skip on failure)
- [ ] `internal/mcp/server.Server` gains `SetBanner(string)`; `callTool` consumes and clears via `bootstrap.ConsumeBanner` once per process; banner is prepended as a leading text content block in the first non-error response
- [ ] An integration test in `cmd/squad/` (or `internal/mcp/`) spins up `squad mcp` against an `httptest.Server` standing in for the daemon, asserts the install path is hit on a clean home, and the first `tools/call` response carries the banner

## Notes

Set `opts.Version` from `versionString` in `cmd/squad/main.go`. Set `opts.BinaryPath` from `os.Executable()` (treat error as non-fatal — log + skip bootstrap).

This is the integration point. Keep the diff small.

## Resolution
(Filled in when status → done.)
