---
id: CHORE-001
title: guide agents toward behavior-anchored recognition (not generic cheer) in chat verbs
type: chore
priority: P3
area: chat
status: done
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-401f
captured_at: 1777239527
accepted_by: web
accepted_at: 1777239588
references: []
relates-to: [BUG-005]
blocked-by: []
---

## Problem

squad's chat verbs (`say`, `fyi`, `ask`, `milestone`, etc.) are also the
audit log and a token sink. Today's guidance (`squad-chat-cadence`
SKILL) tells agents *what* to post (typed-verb, non-obvious state) but
not *how to be warm without being noisy*. Agents tend toward two failure
modes: cold task-only chat that misses real coordination signals, or
generic cheer / small talk that dilutes the audit log and primes
sycophancy in code review. Neither is what we want.

## Context

We want a documented sweet spot: **behavior-anchored recognition**.
Examples that pass: "good catch on the PK collision", "thanks for the
second look on the migration", "the dash-lint fallback test was the
right call". Examples that fail: "great work team!", "you're awesome",
generic small talk while idle. The first kind doubles as review signal
(it tells the recipient *what specifically* worked); the second is
sycophancy and shows up as a hallucinated-agreement risk in reviewer
agents (see `squad-reviewing-as-disprove`).

The natural home for this guidance is the same area BUG-005 is
restructuring — `plugin/skills/squad-chat-cadence/SKILL.md` and the
AGENTS.md template. Wait for BUG-005 to land before editing those, or
fold this in as a small follow-on commit.

## Acceptance criteria

- [ ] `plugin/skills/squad-chat-cadence/SKILL.md` adds a "Recognition"
      section: prescribe behavior-anchored phrasing (cite the specific
      finding/action), call out generic-cheer / small-talk-while-idle as
      anti-patterns, and explicitly forbid generic compliments in
      reviewer-agent roles.
- [ ] At least one good/bad example pair in the SKILL, mirroring the
      existing post-often-post-honestly examples.
- [ ] AGENTS.md.tmpl picks up a one-line pointer to the new section
      (or the section moves directly into AGENTS.md if BUG-005 has
      collapsed the fast/deep split by then).
- [ ] No new lint/CI rule — this is guidance, not enforcement. Don't
      try to detect generic compliments in code.

## Notes

Anti-pattern to avoid in implementation: do not introduce a new chat
verb like `praise` or `compliment`. Recognition belongs in `say` /
`fyi` / `milestone`, anchored to a specific finding. The verb taxonomy
is already complete.

Related: BUG-005 is mid-restructure of the chat-cadence guidance and
the AGENTS.md template at the time of capture. This item should land
*after* BUG-005 to avoid merge conflict.

## Resolution

(Filled in when status → done.)
