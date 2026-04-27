---
id: FEAT-030
title: POST /api/items/{id}/auto-refine endpoint with subprocess and narrow MCP config
type: feature
priority: P2
area: server
status: done
estimate: 3h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308127
accepted_by: web
accepted_at: 1777308350
epic: auto-refine-inbox
references:
  - internal/server/server.go
  - internal/server/items_refine.go
  - internal/items/items.go
relates-to: []
blocked-by: [TASK-040, FEAT-029]
---

## Problem

The dashboard click for Auto-refine needs a server-side handler that spawns the user's local `claude` CLI in print mode with a narrow MCP config, waits up to 90 seconds for completion, and returns either the rewritten item or a structured error. No such endpoint exists today; the existing `POST /api/items/{id}/refine` handler at `internal/server/items_refine.go` only writes reviewer comments to a working section, it does not invoke a subprocess.

## Context

The route is wired alongside the existing refine route in `internal/server/server.go:117`. Implementation pieces:

- **Subprocess invocation**: `exec.CommandContext(ctx, "claude", "-p", prompt, "--mcp-config", tmpConfigPath)` with the context bound to a 90s deadline. On context cancel or timeout, kill the process group (set `Setpgid: true` then `syscall.Kill(-pid, SIGKILL)`).
- **Narrow MCP config**: per-call temp file (or stable per-server file) listing only the read tools (`squad_get_item`, `squad_inbox`, `squad_history`) plus `squad_auto_refine_apply` (FEAT-029). The squad MCP server is invoked with this config; the spawned CLI sees only those tools.
- **Per-item dedupe**: a `map[string]chan struct{}` guarded by a mutex on the server; an in-flight click holds an entry; the second click for the same id returns 409 Conflict.
- **Verification**: after the subprocess exits successfully, the handler re-parses the item file and confirms `auto_refined_at` advanced past the start-of-call timestamp. If not, return 500 ("claude did not produce a draft").
- **Prompt**: a system prompt that frames the model's job — "you are auto-refining a captured squad item; read the item via squad_get_item; replace its body with a Problem / Context / Acceptance criteria block that satisfies the squad-new template-not-placeholder DoR rule; call squad_auto_refine_apply exactly once, then stop." Pinned in source for v1; revisit when we tune draft quality.

## Acceptance criteria

- [ ] New `POST /api/items/{id}/auto-refine` route registered in `internal/server/server.go`, handler in a new `internal/server/items_auto_refine.go`.
- [ ] Handler spawns `claude -p <prompt> --mcp-config <narrow config>` via `exec.CommandContext` with a 90s context deadline; the subprocess runs in its own process group and is killed on context cancel/timeout.
- [ ] Narrow MCP config is generated server-side (either per-call temp file under `os.TempDir` cleaned up via `defer os.Remove`, or a stable per-server file written once at startup); it exposes exactly: `squad_get_item`, `squad_inbox`, `squad_history`, `squad_auto_refine_apply`. No other tools.
- [ ] Per-item dedupe: a second concurrent click for the same id returns 409 with body `{"error": "auto-refine already in flight for {id}"}`. First click completing releases the slot.
- [ ] On success the handler re-parses the item and returns 200 with the parsed item JSON (frontmatter + body); the SPA renders directly from the response without an extra round-trip.
- [ ] On success the handler publishes the existing `inbox_changed` SSE event with action `"auto-refine"` (mirroring how `items_refine.go:29` and `items_recapture.go:25` publish) so other open dashboards refresh without polling. The publish happens here rather than in the MCP tool because `chat.Bus` is an in-memory bus owned by the HTTP server process; the MCP tool runs in a separate `squad mcp` subprocess with no bus access.
- [ ] On `claude` not found in PATH, return 503 with body `{"error": "claude CLI not found on PATH"}` (operator-actionable).
- [ ] On subprocess exit-non-zero, return 502 with body `{"error": "...", "stderr": "<truncated stderr>"}` (truncated to 512 bytes to avoid leaking large outputs).
- [ ] On 90s timeout, return 504 with body `{"error": "auto-refine timed out after 90s"}` and the subprocess + process group are killed.
- [ ] On subprocess success but `auto_refined_at` not advanced (the model never called the write tool), return 500 with `{"error": "claude exited without drafting; run again"}`.
- [ ] Item must be `status: captured`; otherwise 409 with `{"error": "auto-refine only applies to captured items; current status: {status}"}`.
- [ ] Tests: handler unit test that stubs the `exec.Command` factory (inject via a package-level `commandRunner` var) to exercise success / timeout / not-found / non-zero / no-write paths without touching a real subprocess; assert the dedupe map releases the slot after each.

## Notes

`exec.LookPath("claude")` at handler start lets us return the missing-PATH error before forking. Process-group kill on timeout is critical — `claude -p` may itself spawn helpers; a bare `Process.Kill()` can leak children. Setting `Setpgid: true` and killing the negative-pid is the standard Linux/macOS pattern; on Windows squad does not currently target the dashboard, so skip the Windows path.

Truncating subprocess stderr in the response is intentional — squad serve is loopback by default but a future deployment with `--token` could expose this endpoint to a wider audience; full stderr might leak file paths.
