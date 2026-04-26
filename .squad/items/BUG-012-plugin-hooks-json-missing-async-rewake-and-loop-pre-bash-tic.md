---
id: BUG-012
title: plugin/hooks.json missing async_rewake and loop_pre_bash_tick — scripts exist, can't be enabled
type: bug
priority: P2
area: plugin
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241468
accepted_by: web
accepted_at: 1777241680
references: []
relates-to: []
blocked-by: []
---

## Problem

`plugin/hooks/async_rewake.sh` and `plugin/hooks/loop_pre_bash_tick.sh` exist (and have passing test files alongside them), but neither script is registered in `plugin/hooks.json`. `docs/adopting.md:138` advertises both as opt-in via `squad install-hooks`, so a user who follows the advice cannot enable them — there is nothing in the manifest for `install-hooks` to flip on.

## Context

The plugin's hook surface is the contract between the docs and the user. A scripted-but-unregistered hook is dead code from the user's perspective. Either the manifest needs to register them so they can be enabled (preferred — the scripts and tests exist for a reason), or the docs need to stop promising them and the dead scripts deleted. The first option is right: both `async_rewake` and `loop_pre_bash_tick` are referenced in CLAUDE.md / commit history as intentional features.

## Acceptance criteria

- [ ] `plugin/hooks.json` includes registrations for `async_rewake.sh` (likely as a `Stop` or `SubagentStop` handler — confirm against script behavior) and `loop_pre_bash_tick.sh` (likely a `PreToolUse` matcher for `Bash`).
- [ ] `squad install-hooks` flow lists both as opt-in toggles.
- [ ] `docs/adopting.md` "Day 1" section's hook count and named lists are accurate against the manifest.
- [ ] `docs/reference/hooks.md` documents both hooks with what they do.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Verified by listing `plugin/hooks/*.sh` against the `command` lines in `plugin/hooks.json`. Also note: the `adopting.md` "Ten hooks are on by default" claim is fuzzy — it appears to count subagent-event registrations as four separate hooks. Worth a once-over while in the file.

## Resolution

### Reproduction

`TestHooksJSONIncludesShippedOptInHooks` fails on the prior `plugin/hooks.json` — both `async_rewake.sh` and `loop_pre_bash_tick.sh` are absent. RED confirmed; GREEN after the manifest update.

### Fix

`plugin/hooks.json`:
- Added a new top-level `asyncRewake` event with `async_rewake.sh` (matcher `*`).
- Added `loop_pre_bash_tick.sh` as a second entry inside the existing `PreToolUse` Bash matcher, alongside `pre_commit_pm_traces.sh`. Multi-entry-per-matcher precedent already lives at the `Stop` block (`stop_listen.sh` + `stop_learning_prompt.sh`).

The pre-existing `TestEmbedAndHooksJSONStayInSync` and `TestHooksJSONHasNoOrphanEntries` validate the new entries match `embed.All` (event type, matcher, filename) and round-trip cleanly.

### Test

`plugin/hooks/embed_test.go` — added `TestHooksJSONIncludesShippedOptInHooks`. A grep tripwire so the next "wrote a script, forgot the manifest" regression fails fast in CI.

### AC verification

- [x] `plugin/hooks.json` registers both scripts under their correct events.
- [x] `squad install-hooks --help` already lists both as opt-in toggles (verified — embed.All drives the install flow, no code change needed).
- [x] `docs/adopting.md:142` already names both in the opt-in list and `:132` "Ten hooks are on by default" matches embed.go's 10 DefaultOn entries — accurate as-is.
- [x] `docs/reference/hooks.md` table already documents both (lines 15 and 19).

### Evidence

```
ok  	github.com/zsiec/squad/plugin/hooks	0.218s
```
Full `go test ./...` passes.
