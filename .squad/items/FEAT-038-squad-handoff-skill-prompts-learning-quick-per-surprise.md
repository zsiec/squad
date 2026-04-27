---
id: FEAT-038
title: squad-handoff skill prompts learning_quick per surprise
type: feature
priority: P2
area: plugin/skills
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309557
references: []
relates-to: []
blocked-by: [BUG-028]
parent_spec: agent-team-management-surface
epic: observation-to-knowledge-pipeline
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

The squad-handoff skill mandates a "surprised by" bullet on every
sign-off, but those bullets are write-only. They land in chat, scroll
out of the visible window, and never become a learning artifact. The
skill produces the observation and then drops it on the floor.

## Context

The handoff cadence is defined in `plugin/skills/squad-handoff/SKILL.md`.
The CLI sink already exists at `cmd/squad/learning_quick.go`. The
single proposed learning currently in `.squad/learning/` is itself a
meta-note proposing exactly this pipeline — agents have noticed the
gap, but nothing converts the noticing into action.

The fix is mechanical: turn each surprise bullet into a prompt to file
a `learning_quick`, with the body pre-filled from the bullet text so
the agent does not have to retype it. If the agent declines, require
an explicit reason rather than silent skip.

## Acceptance criteria

- [ ] `plugin/skills/squad-handoff/SKILL.md` ends with: "for each
      surprise bullet, file via `squad learning quick` or explain why
      not."
- [ ] `squad learning quick` accepts a `--body` argument so the skill
      can pre-fill the learning body without an extra agent turn.
- [ ] Skill exits cleanly when the agent skips a bullet with an
      explicit reason; no skipped bullet without a reason.

## Notes

Pre-filling the body is the load-bearing piece — without it, the
agent has to re-summarize what they just typed two lines up, which is
exactly the kind of friction that has kept the ledger empty.

Blocked on BUG-028 because the bundle write path that backs
`learning_quick --body` needs to be reliable before we route surprise
bullets through it.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
