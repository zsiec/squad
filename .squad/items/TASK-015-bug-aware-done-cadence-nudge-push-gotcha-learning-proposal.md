---
id: TASK-015
title: bug-aware done cadence nudge — push gotcha learning proposal
type: task
priority: P2
area: cli
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-bbf6
captured_at: 1777245993
accepted_by: agent-bbf6
accepted_at: 1777245993
epic: feature-uptake-nudges
evidence_required: [test]
references:
  - cmd/squad/cadence_nudge.go
  - cmd/squad/done.go
relates-to: []
blocked-by: []
---

## Problem

Across the first 25 closed items (16 of them bugs), zero learning artifacts were filed. The Stop hook prompts at session end — too late in multi-item sessions. The current `printCadenceNudge(w, "done")` line generically says "surprised by anything?" — it doesn't differentiate a bug close-out (where the surprise payload is exactly what gotcha learnings exist for) from a chore close-out (which rarely produces durable lessons).

## Context

`cmd/squad/cadence_nudge.go:16-26` defines `printCadenceNudge(w io.Writer, kind string)`. It's invoked in `cmd/squad/done.go:182` and `cmd/squad/claim.go:158`. The pattern is one stderr line, suppressible via `SQUAD_NO_CADENCE_NUDGES`.

## Acceptance criteria

- [ ] New helper `printCadenceNudgeFor(w io.Writer, kind, itemType string)` extends the existing function. The two-arg `printCadenceNudge` becomes a thin wrapper that calls the three-arg form with `itemType=""`.
- [ ] On `kind="done"` and `itemType="bug"`: the output line mentions the word `gotcha` and the literal command `squad learning propose gotcha`.
- [ ] On `kind="done"` and `itemType in {"feature","task"}`: the output line is the existing generic "surprised by anything?" copy, with `squad learning propose` as the command.
- [ ] On `kind="done"` and `itemType in {"chore","tech-debt","bet"}` or unset: no learning-push line is emitted (silent, since chores rarely produce gotchas).
- [ ] `SQUAD_NO_CADENCE_NUDGES=1` suppresses all variants — preserve the existing silence semantics.
- [ ] In `cmd/squad/done.go:182`, replace `printCadenceNudge(cmd.ErrOrStderr(), "done")` with a call to the new helper, reading the item type from the moved item file (the file is in `bc.doneDir` by that point — use `findItemPath(bc.doneDir, itemID)` and `items.Parse`).
- [ ] Unit tests in `cmd/squad/cadence_nudge_test.go` for each of the four type cases above (bug, feature/task, chore, silenced).
- [ ] `go test ./cmd/squad/...` passes; trailing `ok` line pasted.

## Notes

- The feature/task variant deliberately stays generic — features and tasks can produce any kind of learning (pattern, dead-end, gotcha). Don't presume.
- Chore/debt close-outs explicitly emit nothing, not even the old generic line. A blank line on chore done-out is the right signal that learning capture isn't expected.
- Skill-prose updates land in TASK-018 to keep this item code-only.

## Resolution

### Fix

`cmd/squad/cadence_nudge.go`:
- New `printCadenceNudgeFor(w, kind, itemType)`. The 2-arg `printCadenceNudge` is a thin wrapper that calls the 3-arg form with `itemType=""`.
- `done` + `bug` → mentions `gotcha` and the literal `squad learning propose gotcha`.
- `done` + `{feat, feature, task}` → existing generic "surprised by anything? `squad learning propose <kind> <slug>`" copy.
- `done` + `{chore, tech-debt, bet, ""}` → silent.
- `claim` branch unchanged. `SQUAD_NO_CADENCE_NUDGES` silence preserved across all variants.

`cmd/squad/done.go` — after `done` succeeds, look up the moved item via `findItemPath(bc.doneDir, itemID)`, parse it, and pass `it.Type` to `printCadenceNudgeFor`. Best-effort: missing/unparseable file leaves `itemType=""` and the nudge stays silent — `done` itself doesn't fail.

`cmd/squad/cadence_nudge_test.go` — four new tests (bug, feature/task, overhead, env-suppress); the legacy `TestPrintCadenceNudge_DoneEmitsLearningTip` was renamed to `TestPrintCadenceNudge_DoneWithoutTypeIsSilent` to assert the new contract. Only non-test caller of the 2-arg form is `claim.go:158` (`kind="claim"`), unaffected.

### Coordination

3-arg shape is what agent-bbf6 (TASK-016 — second-opinion claim nudge) and agent-1f3f or me (TASK-017 — per-AC milestone target nudge) plug their helpers into without re-touching this file.

### Evidence

```
$ go test ./cmd/squad ./...
ok  github.com/zsiec/squad/cmd/squad ...
(0 FAIL lines, race-enabled)
```

### AC verification

- [x] `printCadenceNudgeFor(w, kind, itemType)` with 2-arg wrapper.
- [x] `done`+`bug` → "gotcha" + `squad learning propose gotcha`.
- [x] `done`+`{feature, task}` → existing generic copy.
- [x] `done`+`{chore, tech-debt, bet, ""}` → silent.
- [x] `SQUAD_NO_CADENCE_NUDGES` suppresses all variants.
- [x] `done.go` reads item type from moved file via `findItemPath` + `items.Parse`.
- [x] Unit tests cover all four type cases.
- [x] `go test ./cmd/squad/...` passes.
