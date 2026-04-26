---
id: TASK-017
title: per-AC milestone-target claim nudge for items with >=2 AC
type: task
priority: P2
area: cli
status: open
estimate: 45m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-bbf6
captured_at: 1777245995
accepted_by: agent-bbf6
accepted_at: 1777245995
epic: feature-uptake-nudges
evidence_required: [test]
references:
  - cmd/squad/cadence_nudge.go
  - cmd/squad/claim.go
  - internal/items/counts.go
relates-to: []
blocked-by: []
---

## Problem

The chat-cadence skill says "milestone each AC" but only 4 milestone posts were filed across 25 done items — almost half of which had multiple acceptance criteria. Agents post one milestone (usually at the end) or none. Claim-time is the right moment to set the target, because once work is in flight the agent has no concrete number to compare against.

## Context

`internal/items/counts.go` already has helpers for counting acceptance-criteria boxes (verify the exact function name — likely `CountAC` or similar; if absent, the helper is trivial: walk the body looking for `- [ ]`/`- [x]` lines under `## Acceptance criteria`). `cmd/squad/cadence_nudge.go` is the home for the new helper. `cmd/squad/claim.go` is where it's wired after the existing nudges.

## Acceptance criteria

- [ ] New helper `printMilestoneTargetNudge(w io.Writer, acTotal int)` in `cmd/squad/cadence_nudge.go`.
- [ ] Emits a one-line stderr message of shape `"  tip: %d AC items — expect ~%d 'squad milestone' posts as you green each one · silence with SQUAD_NO_CADENCE_NUDGES=1"` when `acTotal >= 2`.
- [ ] Silent for `acTotal == 0` (no AC defined) and `acTotal == 1` (single-AC items don't need a per-AC nudge).
- [ ] Respects `SQUAD_NO_CADENCE_NUDGES`.
- [ ] Wired into `cmd/squad/claim.go` success branch, after the second-opinion nudge from TASK-016 (or independently — order is not load-bearing).
- [ ] If counting AC fails (no `## Acceptance criteria` section, parse error, etc.), the nudge is silent.
- [ ] Unit tests: total=4 fires with both numbers in the output; total=1 silent; total=0 silent; silenced env silent.
- [ ] `go test ./cmd/squad/...` passes; trailing `ok` line pasted.

## Notes

- Independent of TASK-016 even though both wire into `claim.go` — they're separate stderr lines with no overlapping symbols. Either order is fine.
- If `internal/items/counts.go` doesn't expose a usable counter, add one under TDD as part of this item — keep the helper small and testable.

## Resolution

(Filled in when status → done.)
