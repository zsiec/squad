---
title: Per-agent activity stream — Claude Code tool-call events surfaced in the SPA
motivation: |
  Today the SPA's AGENTS panel shows status, current claim, and last-tick timestamp. To understand
  *what* an agent is actually doing — what files it's reading, what Bash commands it's running, when
  it spawns subagents — the operator has to either watch the agent's terminal or read the squad chat
  thread (which only captures explicit chat verbs, not tool calls). Squad's hook system (PreToolUse,
  PostToolUse, SubagentStart, SubagentStop) already fires on every tool call across every Claude Code
  session in a squad-managed repo. This spec adds a thin event-recording layer so those hook firings
  land in a durable per-agent stream that the SPA can render as a click-through "what is this agent
  doing right now" view, with live SSE updates and configurable filtering to keep volume tractable.
acceptance:
  - "A new `agent_events` table records (ts, repo_id, agent_id, session_id, event_kind, tool, target, exit_code, duration_ms) for every tool-call hook firing in a squad-managed repo, written by the existing PreToolUse/PostToolUse/SubagentStart/SubagentStop hooks."
  - "A new `squad event record` internal CLI verb is the single entry point for hook scripts to insert event rows; honors a per-repo redaction policy for tool args (truncate to N chars, drop arg payloads matching a regex)."
  - "`GET /api/agents/:id/events` returns the event stream for one agent, paginated by ts; supports `?since=<ts>` for incremental queries."
  - "`GET /api/agents/:id/timeline` returns a unified rollup — chat verbs + claims + commits + attestations + events — sorted by ts, paginated."
  - "The SPA's AGENTS panel is click-through: clicking an agent row opens an agent-detail drawer rendering the timeline."
  - "The drawer auto-updates via SSE — new events appear without a page refresh while the drawer is open."
  - "The drawer's filter UI defaults to hiding `Read` events (high-volume, low-info) and shows Edit/Write/Bash/Subagent by default, with toggles persisted to localStorage."
  - "Tool-arg redaction is configurable: defaults to truncating to 200 chars and dropping anything matching a sensitive-pattern regex (`SQUAD_REDACT_REGEX` env var or per-repo config), with an explicit comment in the docs warning operators that even truncated args may leak file paths."
  - "Volume sanity: a single Claude Code session running a 30-minute work block produces ≤2000 event rows by default (Read events filtered out at the hook layer when `SQUAD_EVENTS_FILTER_READ=1` is set)."
  - "Dogfood walkthrough: open the SPA, click an active agent, observe live tool-call events appearing in the drawer as the agent runs commands. Document by transcript or screenshot."
non_goals:
  - "Capturing tool *output* (stdout/stderr) — only metadata. Output is too large and too sensitive."
  - "Replaying or re-executing agent actions from the stream. This is a read-only audit/visibility surface."
  - "Per-tool-call attestation. Attestations remain explicit, gated, hash-verified; events are unverified telemetry."
  - "Cross-repo aggregation. Events are scoped to the repo the hook fired in."
  - "Backfilling pre-existing sessions. Events start landing once the hooks are wired; prior history stays in chat/claims/commits."
  - "A dashboard-style aggregate view (charts, percentiles). The drawer is a per-agent stream; aggregation can come later if there's appetite."
integration:
  - "internal/store/migrations/ — new migration for the agent_events table"
  - "cmd/squad/event.go (new) — `squad event record` internal CLI verb"
  - "plugin/hooks/ — extend post_tool_flush.sh, subagent_event.sh; possibly add a thin pre_tool_event.sh"
  - "internal/server/ — new endpoints under /api/agents/:id/events and /api/agents/:id/timeline; SSE channel extension"
  - "internal/server/web/ — new agent-detail drawer + timeline component + SSE wire-up + filter UI"
  - "docs/reference/ — note the redaction policy and the events-table privacy implications"
---

## Background

The SPA's AGENTS panel today (rendered from `internal/server/web/agents.js` consuming `GET /api/agents`) shows a roster: agent id, status, current claim, last tick. The operator can see *that* an agent is alive but not *what* the agent is doing this minute. The information they want — "agent-401f is currently running `go test ./...` after editing `internal/items/new.go`" — exists at the moment a hook fires but is discarded.

Squad's hook system (`plugin/hooks.json` + `plugin/hooks/*.sh`) already fires on PreToolUse, PostToolUse, SubagentStart, SubagentStop, Stop, SessionStart, SessionEnd, asyncRewake, PreCompact, UserPromptSubmit. Two of them write to chat/state today: `post_tool_flush.sh` flushes pending mailbox messages; `subagent_event.sh` posts subagent lifecycle to the agent's claim thread. The rest run for side effects (cadence ticks, hygiene, learning prompts) but don't persist anything queryable.

Adding a thin recording layer turns this latent signal into a stream:

1. **Schema:** a single `agent_events` table indexed by `(repo_id, agent_id, ts)` and `(repo_id, ts)`. STRICT mode, append-only, no foreign keys (agents can be GC'd before their events).
2. **Recording entry point:** `squad event record --kind <event_kind> --tool <name> --target <path-or-arg> --exit <code> --duration-ms <n> --session <id>`. Hook scripts call this; the verb is the single redaction/validation point.
3. **Hook wire-up:** existing scripts gain one extra line each that calls `squad event record` with the relevant fields. Backwards-compatible — the existing chat/state behavior is preserved.
4. **API + SPA:** new endpoints feed an agent-detail drawer in the SPA; the drawer subscribes to an SSE channel for live updates.
5. **Filters + redaction:** UI-side filters keep noise low; binary-side redaction protects against arg payloads leaking secrets.

The volume tradeoff is real: every Edit/Read/Write/Bash from every Claude Code session in a squad-managed repo produces a row. The mitigations layered into the AC — `SQUAD_EVENTS_FILTER_READ` for hook-layer filtering, default UI filter for `Read`, configurable arg truncation — keep the operator's signal-to-noise tractable. If the defaults prove wrong in practice, follow-up items can adjust the cutoff line; the schema doesn't need to change.

The implementation plan and a more detailed phase breakdown can live at `docs/plans/2026-04-27-squad-agent-activity-stream.md` (gitignored locally) when an executor decides to write one.

## Reading the success bar

After rollout, an operator should be able to:
- Open the SPA's AGENTS panel
- Click on `agent-401f`
- See a live timeline showing the agent's current activity (last edit, last bash command, current subagent if any), updating without refresh
- Toggle off `Read` events and see the noise drop by ~70%
- Read the chat verbs (`thinking`, `milestone`, `fyi`) inline alongside the tool-call events for context

If the drawer feels noisy or empty, the spec is wrong — file a follow-up. If the drawer crashes the SPA on a busy session (>10 events/sec sustained), volume controls need tightening; that's a Phase 2.

## Privacy note

Even truncated tool args leak information — file paths reveal repo structure, Bash commands reveal infra dependencies, target paths reveal what code is in flight. The events table is *operator telemetry*, not a public surface. Treat the SPA as authenticated; do not expose `agent_events` over public webhooks; document the redaction config prominently in `docs/reference/`.
