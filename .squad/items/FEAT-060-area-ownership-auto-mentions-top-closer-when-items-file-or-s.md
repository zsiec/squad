---
id: FEAT-060
title: area ownership auto-mentions top closer when items file or scope splits
type: feature
priority: P3
area: chat
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777345159
accepted_by: web
accepted_at: 1777345483
references: []
relates-to: []
blocked-by: []
epic: team-practices-as-mechanical-visibility
---

## Problem

There is no signal anywhere in the system about who knows the most
about a given subsystem. When two agents in parallel sessions filed
near-identical CHORE-017s last week, neither knew the other was
about to file the same item. Mentions appear in 5% of all chat
messages over the last 2 days; routing to a domain-knowledgeable
peer is essentially never happening. A human team uses CODEOWNERS
or oral tradition; agents need a ledger-derived equivalent.

## Context

Every closed item carries an `area:` and an attribution to whichever
agent ran `squad done`. The chat verb `fyi` already supports
`@<agent>` mentions and routes to the agent's mailbox. So the
plumbing exists — what's missing is the autocompute: "who has
closed the most items in this area in the last 30 days?" plus the
auto-`fyi` at the moments where routing matters.

## Acceptance criteria

- [x] On `squad new`, after the item file is scaffolded, if the
      `--area` value matches an area where some agent has closed
      ≥3 items in the last 30 days, auto-`fyi` that agent on the
      `#global` thread with body
      `new <ID> in area <area> — heads-up, you've been the top
      closer here recently`. Configurable via env
      (`SQUAD_NO_AREA_MENTIONS=1` to suppress).
- [x] On `squad refine` (or any explicit area-change in the
      frontmatter), if the new area has a different top-closer
      than the previous area, auto-`fyi` the new top-closer on
      the item thread.
- [x] When no agent qualifies (no closer with ≥3 closes in the
      area), the auto-`fyi` is suppressed silently — don't post
      "no owner found" noise.
- [x] The "top closer" calculation is shared with the retro
      generator (FEAT-059) so both surface the same routing
      signal — single source of truth in `internal/stats/` or
      similar.
- [x] Test: seed two agents with closes across two areas, file a
      new item in one area, assert the right agent gets the fyi
      and only the right agent.

## Notes

- Tie-breakers when multiple agents have the same close count:
  most-recent-close wins. If still tied, alphabetical by agent
  display name — deterministic, low-stakes.
- Risk: this creates a feedback loop ("agent X gets all the items
  in area Y because they closed three of them"). Acceptable for
  now; if it becomes a problem, the operator can override on the
  item file directly.

## Resolution

`internal/stats/TopCloser` is the shared single source of truth (AC#4); `cmd/squad/area_mentions.go` wraps it with two best-effort fyi helpers — `notifyAreaTopCloser` (squad new path) and `notifyAreaChange` (recapture path). AC#2's "or any explicit area-change in the frontmatter" is currently scoped to the `squad recapture` path: that's the canonical commit-back point after a captured agent edits the frontmatter to address reviewer feedback. Other potential area-write paths (auto-refine apply, direct file edits + accept/reject) are not currently hooked — file a follow-up if the broader contract is desired.
