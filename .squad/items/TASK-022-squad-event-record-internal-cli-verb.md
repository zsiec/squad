---
id: TASK-022
title: squad event record internal CLI verb
type: task
priority: P1
area: cli
status: open
estimate: 45m
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: 2026-04-27
captured_by: agent-bbf6
captured_at: 1777251703
accepted_by: agent-bbf6
accepted_at: 1777251703
epic: agent-activity-stream
references:
  - cmd/squad/event.go (new)
  - internal/store/
  - cmd/squad/main.go (cobra root for command registration)
relates-to:
  - TASK-021
blocked-by:
  - TASK-021
---

## Problem

Hook scripts need a single CLI entry point to write `agent_events` rows. Inlining SQL in shell hooks is fragile, hard to test, and would replicate redaction/validation logic across four hook scripts. The fix is one Go-side CLI verb that takes flags, validates, applies redaction (CHORE-003), and inserts a single row.

## Context

Pattern to follow: `cmd/squad/attest.go` already exposes a CLI that takes flags, opens the DB, inserts a record into a STRICT table, exits 0. New verb should mirror that shape — short, no surprise behavior, designed for hook-script invocation. Register the new command in `cmd/squad/main.go` (find the existing `rootCmd.AddCommand` block and add this one).

## Acceptance criteria

- [ ] New file `cmd/squad/event.go` with a `newEventCmd() *cobra.Command` returning a parent command with one subcommand: `record`.
- [ ] `squad event record` accepts flags:
  - `--kind <event_kind>` (required; one of `pre_tool`, `post_tool`, `subagent_start`, `subagent_stop`)
  - `--tool <name>` (e.g. `Bash`, `Edit`, `Read`)
  - `--target <path-or-arg>`
  - `--exit <code>` (default 0)
  - `--duration-ms <n>` (default 0)
  - `--session <id>` (default empty; falls back to `$SQUAD_SESSION_ID` env var if unset)
  - `--agent <id>` (default: resolved via existing identity helpers)
- [ ] Inserts one row into `agent_events` with `ts = time.Now().Unix()`. Repo id resolved via existing `repo.Discover()` helper.
- [ ] On any error (DB closed, repo not discoverable, etc.) the command exits 0 silently and prints to stderr — hook scripts must NEVER fail the parent Claude Code session because of a recorder failure. Defensive on the boundary, idempotent per row.
- [ ] Wire the command into `cmd/squad/main.go` via `rootCmd.AddCommand(newEventCmd())`.
- [ ] Apply the redaction policy from CHORE-003 if available; if CHORE-003 isn't merged yet, leave a TODO marker — tests should still pass without redaction (raw target stored).
- [ ] Unit tests in `cmd/squad/event_test.go`: happy path (writes row), missing-kind error path, missing-DB silent-exit-0 path, env-fallback for session id.
- [ ] `go test ./cmd/squad/...` passes; trailing `ok` line pasted into close-out chat.

## Notes

- This is the single point that hook scripts call. Do not expose any subcommand other than `record` yet — `squad event list`, `squad event tail`, etc. are out of scope (the server endpoints in TASK-025 are the read surface).
- The "exit 0 on any error" rule is critical: hooks run synchronously in the Claude Code main loop; a non-zero exit here would block the agent. Trust the boundary, fail open.
- Wire-up of the recorder into the actual hook scripts is TASK-023 (PreTool/PostTool) and TASK-024 (Subagent) — this item just builds the verb.
- Redaction (CHORE-003) is a sibling concern that lands in parallel with this item; if it lands first, consume it; if not, leave the integration point clearly marked.

## Resolution

(Filled in when status → done.)
