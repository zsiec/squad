---
id: FEAT-043
title: stale chat cadence nudge on long-held claims
type: feature
priority: P2
area: cmd/squad
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309558
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: cadence-and-time-boxing-as-pacing
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

Chat verb usage in recent traffic is bottom-heavy — `claim`, `done`,
`release`, and `say` dominate, while `thinking` and `milestone` are
absent. Peers cannot route attention if non-obvious state is never
posted. Agents need an external prompt when they go silent on a held
claim.

## Context

The wiring is already in place:

- `cmd/squad/cadence_nudge.go` houses the existing nudge logic and is
  the natural home for a stale-chat variant.
- `plugin/skills/squad-chat-cadence/SKILL.md` documents the verb
  taxonomy the nudge should reference.
- `internal/chat/` defines the verb types and exposes the
  last-message-by-agent query the wakeup needs to check whether
  anything was posted since claim time.

The check is: on a held claim, has this agent posted a `thinking` or
`milestone` since `claim_at`? If not, fire a short nudge.

## Acceptance criteria

- [ ] A cadence wake fires every 30m on a held claim with no `thinking`
      or `milestone` since claim time.
- [ ] The nudge text is short and references the `squad-chat-cadence`
      skill by name.
- [ ] Suppressible via `SQUAD_NO_CADENCE_NUDGES=1`, consistent with the
      existing nudges.
- [ ] Nudges stop firing as soon as the agent posts a `thinking` or
      `milestone`, and resume the 30m window from the latest post.
- [ ] Test coverage exercises both the silent-agent and recently-posted
      paths.

## Notes

Treat `stuck` as satisfying the cadence too — an agent who has flagged
a blocker should not be nudged for silence. The verb whitelist lives in
`internal/chat/`; reuse it rather than hardcoding strings here.

## Resolution

(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
