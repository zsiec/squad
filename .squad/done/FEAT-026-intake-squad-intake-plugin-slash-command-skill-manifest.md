---
id: FEAT-026
title: 'intake: squad-intake plugin slash command, skill, manifest'
type: feature
priority: P2
area: plugin/squad
parent_spec: intake-interview
parent_epic: intake-interview-plugin
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777290819
accepted_by: web
accepted_at: 1777291153
references: []
relates-to: []
blocked-by: [FEAT-024, FEAT-025]
---

## Problem
Claude Code users invoke the interview via `/squad:squad-intake`. That requires a slash command, a skill that drives the interview prose, and a manifest registering the four MCP tools.

## Context
- `plugin/squad/commands/squad-intake.md` — thin slash command, matches the format of `plugin/squad/commands/squad-work.md`.
- `plugin/squad/skills/squad-intake/SKILL.md` — operating manual for the interview agent: one question at a time, multiple-choice when possible, the literal `Ship it? (y/n/edit)` sign-off prompt, hard rules against inventing acceptance criteria or silently passing `--ready`.
- `plugin/manifest.json` — register the four MCP tools, semver-minor bump.

Plan ref: Task 13.

## Acceptance criteria
- [ ] Slash command file exists and matches existing format.
- [ ] Skill file describes the loop: open → ask → record turn → check still_required → draft → "Ship it? (y/n/edit)" → commit.
- [ ] Manifest registers `squad_intake_open`, `squad_intake_turn`, `squad_intake_status`, `squad_intake_commit`. Plugin version bumped.
- [ ] Manual smoke test in a fresh tmp repo: green-field interview produces `item_only` bundle; refine-mode interview supersedes a stub item.

## Notes
No automated test for plugin markdown — manual smoke is the gate. Paste smoke output into the done summary.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
