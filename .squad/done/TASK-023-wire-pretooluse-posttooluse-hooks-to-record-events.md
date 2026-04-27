---
id: TASK-023
title: wire PreToolUse + PostToolUse hooks to record events
type: task
priority: P1
area: plugin
status: done
estimate: 1h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777251704
accepted_by: agent-bbf6
accepted_at: 1777251704
epic: agent-activity-stream
references:
  - plugin/hooks/post_tool_flush.sh
  - plugin/hooks/pre_commit_pm_traces.sh
  - plugin/hooks/loop_pre_bash_tick.sh
  - plugin/hooks.json
relates-to:
  - TASK-022
blocked-by:
  - TASK-022
---

## Problem

The `squad event record` verb (TASK-022) is in place but no hook script calls it. Without the wire-up, the `agent_events` table stays empty and the SPA drawer has nothing to render. This item connects the two.

## Context

Existing PreToolUse hooks (per `plugin/hooks.json`):
- `pre_commit_pm_traces.sh` — matches `Bash`, blocks commits with PM-trace in messages
- `loop_pre_bash_tick.sh` — matches `Bash`, sends a tick on every Bash call
- `pre_edit_touch_check.sh` — matches `Edit|Write`, warns on cross-agent file conflicts

Existing PostToolUse hook:
- `post_tool_flush.sh` — matches `*`, flushes pending mailbox messages

We have two options for the recorder: (a) extend each existing hook with a one-liner call to `squad event record`, or (b) add a thin new hook script just for recording. Option (b) keeps existing hooks single-purpose; option (a) avoids hook proliferation. **Recommendation: option (b)** — add `pre_tool_event.sh` and `post_tool_event.sh` as the dedicated event recorders, register them in `hooks.json` alongside the existing hooks. Clean separation, easy to disable independently of the existing hooks.

## Acceptance criteria

- [ ] New `plugin/hooks/pre_tool_event.sh` that:
  - Reads the hook envelope (tool name, args) from stdin per Claude Code hook protocol
  - Calls `squad event record --kind pre_tool --tool <name> --target <truncated_first_arg>` with appropriate flags
  - Exits 0 always (silently absorbs all errors)
  - Honors `SQUAD_NO_HOOKS=1` env var (existing convention) for total opt-out
  - Honors `SQUAD_EVENTS_FILTER_READ=1` env var — when set and tool is `Read`, skip the recorder call entirely
- [ ] New `plugin/hooks/post_tool_event.sh` mirroring the same shape but reading exit code + duration if exposed by the hook envelope, and using `--kind post_tool`.
- [ ] Register both hooks in `plugin/hooks.json` under the existing `PreToolUse` / `PostToolUse` matcher blocks. Do NOT modify the existing entries — append the new event recorders to each block's `hooks` list.
- [ ] Test scripts at `plugin/hooks/pre_tool_event_test.go` + `post_tool_event_test.go` (mirroring the existing test pattern in `plugin/hooks/embed_test.go` etc.). Verify the scripts:
  - Are valid shell (shellcheck-clean would be ideal but not strictly required)
  - Honor `SQUAD_NO_HOOKS`
  - Honor `SQUAD_EVENTS_FILTER_READ` when tool is `Read`
  - Don't error when `squad` binary isn't on PATH (silent exit 0)
- [ ] `go test ./plugin/hooks/...` passes; trailing `ok` line pasted into close-out chat.

## Notes

- Existing hooks are not modified — they keep their single-purpose responsibility (PM-trace blocking, cadence ticking, mailbox flushing). The new event recorders are siblings, not amendments.
- The hook protocol envelope shape is documented in the existing scripts — read `pre_commit_pm_traces.sh` for the JSON-on-stdin pattern and `post_tool_flush.sh` for the post-tool envelope shape.
- Do not log tool *output* (stdout/stderr) — only metadata. The hook envelope may expose output; ignore it.
- The pre-edit `pre_edit_touch_check.sh` already runs on `Edit|Write`; the new `pre_tool_event.sh` matches `*` so it covers `Edit|Write|Bash|Read|...` uniformly.

## Resolution

(Filled in when status → done.)
