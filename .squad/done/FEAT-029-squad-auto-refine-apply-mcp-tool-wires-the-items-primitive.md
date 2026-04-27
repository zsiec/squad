---
id: FEAT-029
title: squad_auto_refine_apply MCP tool wires the items primitive
type: feature
priority: P2
area: mcp
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308127
accepted_by: web
accepted_at: 1777308350
epic: auto-refine-inbox
references:
  - internal/mcp/server.go
relates-to: []
blocked-by: [TASK-040]
---

## Problem

The auto-refine flow needs a single MCP tool — `squad_auto_refine_apply(item_id, new_body)` — that the spawned `claude` CLI subprocess calls exactly once to persist its drafted body. Without this tool the CLI cannot write through the constrained MCP surface and we would have to widen the toolset (explicitly rejected at design time).

## Context

`internal/mcp/server.go` registers every existing squad tool (`squad_get_item`, `squad_inbox`, `squad_refine`, `squad_recapture`, etc.). We add one more that delegates to `items.AutoRefineApply` (TASK-040). FEAT-030 materializes a per-call narrow MCP config that exposes only the read tools + this tool to the spawned CLI — that gating happens at config-export time, not at server-registration time, so this item simply registers the tool unconditionally and lets FEAT-030 decide which sessions can see it.

## Acceptance criteria

- [ ] New MCP tool `squad_auto_refine_apply` registered in `cmd/squad/mcp_register.go` (the actual tool registration site, mirroring how `squad_recapture` is registered there) with input schema `{item_id: string, new_body: string}` and a description that it stamps `auto_refined_at` / `auto_refined_by`, refuses non-captured items, and is intended for the auto-refine subprocess flow.
- [ ] The handler delegates to `items.AutoRefineApply(squadDir, itemID, newBody, "claude")`; DoR violations and status errors flow back as structured error messages the LLM can read and react to.
- [ ] Unit tests cover: happy path (body rewrite, success response); DoR-fail path (error with the failing rule name); non-captured path (error with the current status).
- [ ] The success response does not include the full body or absolute file path — it returns `{ok: true, item_id, auto_refined_at}`. The dashboard re-fetches the body via the existing item-detail endpoint.

## Notes

Tool registration is unconditional in `cmd/squad/mcp_register.go`. Restricting visibility of this tool to the auto-refine subprocess belongs to FEAT-030's narrow MCP config, not this item.

The `inbox_changed` SSE publish is intentionally NOT this item's responsibility. The MCP tool runs in a separate `squad mcp` process (typically spawned by `claude -p` which is itself spawned by the FEAT-030 HTTP handler), with no shared memory and no access to the HTTP server's in-memory `chat.Bus`. The publish lives in FEAT-030's HTTP handler, which wraps the subprocess invocation and emits `inbox_changed` after the subprocess returns success. Existing MCP mutation tools (`squad_recapture`, `squad_done`, etc.) follow the same pattern — only their parallel HTTP handlers in `internal/server/items_*.go` publish.
