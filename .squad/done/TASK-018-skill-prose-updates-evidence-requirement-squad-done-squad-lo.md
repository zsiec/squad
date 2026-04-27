---
id: TASK-018
title: skill prose updates — evidence-requirement, squad-done, squad-loop, chat-cadence
type: task
priority: P3
area: docs
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777245997
accepted_by: agent-bbf6
accepted_at: 1777245997
epic: feature-uptake-nudges
evidence_required: []
references:
  - plugin/skills/squad-evidence-requirement/SKILL.md
  - plugin/skills/squad-done/SKILL.md
  - plugin/skills/squad-loop/SKILL.md
  - plugin/skills/squad-chat-cadence/SKILL.md
relates-to: []
blocked-by:
  - TASK-012
  - TASK-015
  - TASK-016
  - TASK-017
---

## Problem

Four skill files reference behavior the dogfood data shows agents aren't doing: paste-only evidence, generic learning prompts, no second-opinion guidance at claim time, no per-AC milestone target. Once the code-side feature items land, the skill prose needs to align with the new in-flow signals so the agent reading the skill mid-loop sees the same behavior the binary nudges them toward.

## Context

Four files, four small edits. No code change.

## Acceptance criteria

- [ ] `plugin/skills/squad-evidence-requirement/SKILL.md` — the rule section names *two* artifacts (paste-into-chat AND `squad attest`), with a concrete example block showing `tee` + `squad attest --kind test --command ... --output ...`. Adds a "What kinds to record" sub-section covering `--kind test`, `--kind review`, `--kind manual`.
- [ ] `plugin/skills/squad-done/SKILL.md` — the close-out checklist gains a fourth gate: "Learning capture (bug/feat/task only)" pointing at the new per-item nudge from TASK-015 and noting that the Stop-hook prompt fires too late in multi-item sessions.
- [ ] `plugin/skills/squad-loop/SKILL.md` — step 3 ("Claim atomically") gets one additional sentence noting the second-opinion nudge fires on P0/P1/risk:high and that agents should treat it as a real prompt, not a styling annoyance.
- [ ] `plugin/skills/squad-chat-cadence/SKILL.md` — the "On AC complete" bullet under "The cadence to aim for" is rewritten to say "aim for ~one milestone post per AC box" explicitly.
- [ ] `go test ./internal/skills/...` (frontmatter parser test) passes; trailing `ok` line pasted.

## Notes

- `evidence_required: []` here because there's no test command worth attesting on a docs-only edit. The skill-frontmatter parser test is the only verification gate; pasting its trailing `ok` into close-out chat is sufficient.
- Blocked-by every code-side item in the epic so the prose lands describing what shipped, not what's planned.

## Resolution

(Filled in when status → done.)
