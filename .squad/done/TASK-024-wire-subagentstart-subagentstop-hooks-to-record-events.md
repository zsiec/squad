---
id: TASK-024
title: wire SubagentStart + SubagentStop hooks to record events
type: task
priority: P2
area: plugin
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777251705
accepted_by: agent-bbf6
accepted_at: 1777251705
epic: agent-activity-stream
references:
  - plugin/hooks/subagent_event.sh
  - plugin/hooks.json
relates-to:
  - TASK-022
blocked-by:
  - TASK-022
---

## Problem

The existing `subagent_event.sh` posts subagent lifecycle to chat but doesn't record an `agent_events` row. After this item, every subagent dispatch and completion produces both a chat post (current behavior, unchanged) and an event row.

## Context

`plugin/hooks/subagent_event.sh` is registered in `plugin/hooks.json` for SubagentStart, SubagentStop, TaskCreated, TaskCompleted (one script handles all four via the `$1` event-name argument). Read it first to see the existing shape.

## Acceptance criteria

- [ ] `plugin/hooks/subagent_event.sh` extended with a call to `squad event record --kind subagent_start` (or `subagent_stop`) after the existing chat-post logic. The chat post is preserved — the event recording is additive.
- [ ] The recorder call honors `SQUAD_NO_HOOKS=1` (skips both chat and event when set, same as existing).
- [ ] The recorder call's `--tool` is set to the subagent type / description if available in the hook envelope; `--target` is the subagent description; `--exit` is the subagent exit code on stop.
- [ ] If `squad` binary isn't on PATH, the script exits 0 silently — no parent-session blocking on a missing binary.
- [ ] Test in `plugin/hooks/subagent_event_test.go` (exists; extend) verifying the recorder is invoked when the binary is mocked, and skipped when the env var is set.
- [ ] `go test ./plugin/hooks/...` passes.

## Notes

- TaskCreated / TaskCompleted are also dispatched through `subagent_event.sh` — those produce no `agent_events` row in this item; the verb taxonomy is `pre_tool` / `post_tool` / `subagent_start` / `subagent_stop` only. If a future item wants TaskCreated/Completed events, add them then.
- Single hook script handling four event kinds means careful argument parsing — `$1` is the event name; switch on it.

## Resolution

(Filled in when status → done.)
