---
id: TASK-016
title: second-opinion claim nudge for P0/P1 and risk:high items
type: task
priority: P2
area: cli
status: done
estimate: 45m
risk: low
created: 2026-04-26
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777245994
accepted_by: agent-bbf6
accepted_at: 1777245994
epic: feature-uptake-nudges
evidence_required: [test]
references:
  - cmd/squad/cadence_nudge.go
  - cmd/squad/claim.go
  - internal/items/items.go
relates-to: []
blocked-by: []
---

## Problem

Across 25 dogfood items, `squad ask` peer-to-peer was used twice. Twenty-three items were single-agent solo runs, including P1 bug fixes and architectural changes that would reasonably benefit from a second pair of eyes before the work starts. Skills tell agents *how* to ask; nothing tells them *when*. Claim-time is the right moment because the cost of redirecting at the start of work is much lower than at code-review.

## Context

`cmd/squad/cadence_nudge.go` already has `printCadenceNudge` modeled as a one-line stderr nudge with env-var suppression. `cmd/squad/claim.go:144-159` does the success-path printing. Item priority and risk are on the parsed item struct (`internal/items/items.go` — `Priority`, `Risk` fields).

## Acceptance criteria

- [ ] New helper `printSecondOpinionNudge(w io.Writer, priority, risk string)` in `cmd/squad/cadence_nudge.go`.
- [ ] The helper emits a one-line stderr message containing the literal command `squad ask @` when `priority in {"P0","P1"}` OR `risk == "high"`. Otherwise it emits nothing.
- [ ] The helper respects `SQUAD_NO_CADENCE_NUDGES` (returns silent if set).
- [ ] The helper is invoked from `cmd/squad/claim.go` after `printCadenceNudge(..., "claim")` on the success path. Read the item from `bc.itemsDir` via `findItemPath` + `items.Parse` to get priority/risk.
- [ ] Unit tests: P0+low fires; P2+high fires; P3+low silent; silenced env always silent.
- [ ] If `items.Parse` fails (item file unreadable), the nudge is silent — never block claim success on a nudge.
- [ ] `go test ./cmd/squad/...` passes; trailing `ok` line pasted.

## Notes

- The exact copy is "high-stakes claim — consider `squad ask @<peer> \"sanity-check my approach?\"` before starting" + the silence-env reminder. This is bikesheddable; the contract is that the line contains `squad ask @`.
- Independent of all other items in this epic — touches `cadence_nudge.go` (additive helper, no overlap with TASK-015's `printCadenceNudgeFor`) and `claim.go` (one-line wiring after the existing success branch).
- Skill-prose updates land in TASK-018.

## Resolution

(Filled in when status → done.)
