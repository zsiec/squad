---
id: BUG-003
title: PreToolUse Bash hook emits hookSpecificOutput without hookEventName field
type: bug
priority: P1
area: hooks
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-401f
captured_at: 1777237660
accepted_by: agent-401f
accepted_at: 1777237660
references: []
relates-to: []
blocked-by: []
---

## Problem

`plugin/hooks/loop_pre_bash_tick.sh:24` emitted
`{"hookSpecificOutput":{"additionalContext":...}}` without the required
`hookEventName` field. Claude Code rejected the envelope on every Bash
tool call with: "Hook JSON output validation failed — hookSpecificOutput
is missing required field 'hookEventName'".

## Context

`pre_compact.sh:74` and the Go-side `cmd/squad/mailbox.go:57-71` both
include `hookEventName` correctly. The shell printf in
`loop_pre_bash_tick.sh` was the only emitter missing it. Existing tests
(`TestLoopPreBashTick_EmitsContextWithJQ`,
`TestLoopPreBashTick_FallsBackWithoutJQ`) checked for `additionalContext`
but not `hookEventName`, so the bug slipped through.

## Acceptance criteria

- [x] Both jq and fallback test paths assert
      `"hookEventName":"PreToolUse"` is present in the emitted JSON.
- [x] Single printf format string in `loop_pre_bash_tick.sh:24` includes
      the field; both paths benefit because they share the line.
- [x] `go test ./plugin/hooks/...` green (incl. `DashLintClean`).

## Resolution

`plugin/hooks/loop_pre_bash_tick.sh:24` now emits
`{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":%s}}`.
Tests in `plugin/hooks/loop_pre_bash_tick_test.go` assert the field on
both jq and fallback paths.
