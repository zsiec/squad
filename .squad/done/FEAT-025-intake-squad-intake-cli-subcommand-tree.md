---
id: FEAT-025
title: 'intake: squad intake CLI subcommand tree'
type: feature
priority: P2
area: cmd/squad
parent_spec: intake-interview
parent_epic: intake-interview-cli
status: done
estimate: 2h
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
blocked-by: [FEAT-016, FEAT-018, FEAT-022, FEAT-023]
---

## Problem
Humans (and scripts) need a CLI surface for the intake lifecycle: kick off, list, inspect, cancel, manual commit.

## Context
New parent command `cmd/squad/intake.go` plus per-subcommand files. Wired into the root cobra command. Source of truth for the plugin slash command.

Plan ref: Task 12.

## Acceptance criteria
- [ ] `squad intake new <idea...>` opens a green-field session, prints session_id and skill briefing.
- [ ] `squad intake refine <item-id>` opens a refine-mode session.
- [ ] `squad intake list` lists open sessions for current `(repo, agent)`.
- [ ] `squad intake status <id>` pretty-prints transcript + checklist gaps.
- [ ] `squad intake cancel <id>` marks cancelled (irreversible).
- [ ] `squad intake commit <id> --bundle <path>` reads JSON, calls Commit. Emergency form.
- [ ] Integration tests drive new → fake turns via direct DB → commit, asserting filesystem state.

## Notes
Subcommands follow the existing intake-family CLI conventions (`squad refine`, `squad decompose`).

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
