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
(Filled in when status → done.)
