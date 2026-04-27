---
id: FEAT-012
title: first-run banner in MCP tools call response
type: feature
priority: P1
area: mcp
epic: first-run-dashboard
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777290336
accepted_by: web
accepted_at: 1777290585
references:
  - .squad/specs/mcp-dashboard-bootstrap.md
  - docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md
relates-to: []
blocked-by:
  - FEAT-008
---

## Problem

Auto-open lands the user on the dashboard, but the URL also needs to be discoverable in the chat — so the user can re-find it later, and so users on platforms where auto-open got swallowed (sandboxed environments, no default browser, headless dotfile dev VMs) still see the URL once.

## Context

`internal/mcp/server.go` already handles `tools/call` and returns a structured response with a `content` array. Prepending a text block is straightforward. The trick is consume-and-clear: the banner should appear **once** across the lifetime of a process, regardless of which tool the user calls first.

The banner string itself is set by `Ensure` at the moment it knows what happened: install, upgrade, port-conflict skip, unsupported-platform skip.

## Acceptance criteria

- [ ] `bootstrap.SetBanner(s string)` and `bootstrap.ConsumeBanner() string` exist (filled in here per the seams from FEAT-008)
- [ ] `ConsumeBanner` returns the current banner and atomically clears it; subsequent calls return ""
- [ ] `internal/mcp/server.go` `callTool` calls `ConsumeBanner` once per response; on non-empty value, prepends a leading `{"type": "text", "text": <banner>}` content block
- [ ] First successful `tools/call` carries the banner; subsequent calls do not
- [ ] If the very first call errors (and would not normally include a content block), the banner is still emitted in the next successful response
- [ ] Banner copy templates (set by `Ensure`):
    - `Squad dashboard ready at http://localhost:7777` (fresh install)
    - `Squad upgraded to <ver>; dashboard restarted` (upgrade flow)
    - `Squad dashboard unavailable: port 7777 in use` (port conflict skip)
    - `Squad dashboard auto-install not supported on this platform; run "squad serve" manually` (BUG-020)
- [ ] Unit test exercises consume-and-clear semantics across a sequence of tool calls

## Notes

Use `atomic.Value` keyed on `string` for the banner store, with a sentinel for "consumed." Or a `sync.Mutex` around a string + bool. Either works; pick the simpler one.

## Resolution
(Filled in when status → done.)
