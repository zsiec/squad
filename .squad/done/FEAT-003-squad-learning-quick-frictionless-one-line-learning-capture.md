---
id: FEAT-003
title: squad learning quick — frictionless one-line learning capture
type: feature
priority: P1
area: cli
status: done
estimate: 1.5h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777250500
accepted_by: agent-401f
accepted_at: 1777250500
references:
  - cmd/squad/learning_propose.go
  - internal/learning/learning.go
  - internal/learning/parse.go
  - internal/learning/paths.go
relates-to:
  - TASK-019
  - BUG-019
blocked-by: []
---

## Problem

The dogfood walkthrough for `feature-uptake-nudges` (TASK-019) closed with one
clear meta-finding: across 25 done items the team filed **one** learning. The
in-flow done-time nudge fires correctly but is not enough on its own — the
remaining gap is **propose-time friction**. `squad learning propose <kind> <slug>`
requires `--title` and `--area` flags; the slug must be kebab-case; the path
defaults assume `internal/<area>/**`. By the time an agent has decided what
kind it is and produced a kebab-case slug, the surprise is half-forgotten and
the moment has passed.

The done-time nudge points at `squad learning propose gotcha <slug>` — which
already errors out because `--title` and `--area` are missing. The nudge
discovers the friction; it doesn't remove it.

## Context

`cmd/squad/learning_propose.go:94-133` defines the propose verb. `LearningPropose`
(line 53) requires non-empty `Title` and `Area` and rejects if either is missing.
`stubBody` (line 147) generates the markdown stub including kind-specific
section headers (`## Looks like / ## Is / ## So` for gotchas, etc.) — the
template work that follows propose is fine; the entry friction is the problem.

Existing pattern: `internal/learning/learning.go` (the `Learning` struct) already
treats most fields as optional at the parser level — `Title`, `Area`, `Paths`
have no enforced shape downstream. The hard requirement lives only in the
propose CLI surface.

## Acceptance criteria

- [x] New cobra subcommand `squad learning quick <one-liner>` registered under
  the existing `learning` parent in `cmd/squad/learning.go`. Single positional
  arg, no required flags.
- [x] Default kind is `gotcha`. Override with `--kind <gotcha|pattern|dead-end>`.
- [x] Slug is auto-derived from the one-liner: lowercase, non-`[a-z0-9]` →
  `-`, collapse runs of `-`, trim, max 60 chars. Collision walks `slug-2`,
  `slug-3`, … through `slug-9` then errors with a `propose`-with-explicit-slug
  hint. Helper lives in `cmd/squad/learning_quick.go` next to the cobra
  command (kept colocated rather than shoved into `learning_propose.go`
  because it is a quick-only concern).
- [x] Title defaults to the one-liner verbatim.
- [x] Area defaults to the most-recently-modified item under `.squad/done/`
  (parsed for `area:`); falls back to `general` when no readable item is
  present. **Tier 1 of the AC's three-tier heuristic was dropped** — see
  Resolution.
- [~] Paths default unchanged from `LearningPropose` (`internal/<area>/**`).
  **The `# TODO: refine paths` stub-marker requirement was renegotiated** —
  CLAUDE.md prohibits future-work TODO comments. The Via marker
  (`> captured via squad learning quick`) already gives reviewers the
  visual cue that the stub came from auto-derivation. See Resolution.
- [x] The stub body for `quick` mode includes a `> captured via squad learning quick`
  marker line above the section template.
- [x] Implementation reuses `LearningPropose` internally — the cobra path and
  the MCP path both call a shared `LearningQuick(ctx, args)` helper which
  threads through `LearningPropose`. No copy of the collision walk or stub
  writer.
- [x] On success, print the path AND a one-line follow-up nudge via
  `printQuickFollowupNudge` (the text-returning helper `quickFollowupNudgeText`
  is reused by the MCP handler so the Tips slice carries the same string).
  Suppressible with `SQUAD_NO_CADENCE_NUDGES=1`.
- [x] Unit tests in `cmd/squad/learning_quick_test.go`:
  - happy path: derives the expected slug, title, area, marker.
  - kind override: `--kind pattern` writes to `patterns/proposed/` with the
    pattern-shaped section template.
  - slug collision: pre-seed a `proposed/foo.md`, run `quick "foo"`, expect
    `foo-2.md`.
  - one-liner shorter than 3 chars: rejected with the "more specific" hint.
  - silenced env: no follow-up nudge printed.
  - plus 9 unit cases for `deriveQuickSlug` (incl. Unicode + emoji pin) and
    3 cases for `inferQuickArea`.
- [x] `docs/reference/commands.md` updated with the new subcommand, defaults
  table, and two examples. `plugin/skills/squad-done/SKILL.md` recommends
  `squad learning quick` first as the lowest-friction option.
- [x] MCP exposure: `squad_learning_quick` registered in
  `cmd/squad/mcp_register.go` with `schemaLearningQuick` in
  `cmd/squad/mcp_schemas.go`. Response carries `Tips` populated with the
  same follow-up nudge text the cobra path writes to stderr (BUG-019
  parity). Two MCP roundtrip tests cover populated/silenced.
- [x] `go test ./cmd/squad/ -count=1 -race` passes — see Resolution evidence.

## Notes

- This is the **smallest** of the three propose-time friction reductions filed
  out of TASK-019. The others (FEAT-004 `--propose-from-surprises`, future OTel
  trace-driven auto-proposal) are higher leverage but bigger surface.
- Don't change the `propose` verb's contract. Some agents have it muscle-memorized;
  `quick` is additive.
- Slug auto-derivation must NOT include leading digits — the existing `validSlug`
  rejects them. Strip leading non-alpha chars from the derived slug before
  applying collision-walk.
- Area-inference heuristic: read the most recent `closed` row in `claims` for
  this `agent_id` (table is in the global DB), fall back to the most-recently-
  modified `.squad/done/*.md` item, fall back to `general`. Three tiers, each
  with one-line probe — keep it cheap.
- MCP exposure: register `squad_learning_quick` in `cmd/squad/mcp_register.go`
  so MCP-using agents (per BUG-019) get the same surface. Schema is just
  `{one_liner: string, kind?: string}` plus the standard envelope. MCP response
  populates the same `Tips` field BUG-019 is adding to other handlers.

## Resolution

### Fix

`cmd/squad/learning_quick.go` (new) — three pieces:

- `deriveQuickSlug(string) string` normalises a one-liner to kebab-case (max
  60 chars, leading non-alpha stripped, runs of `-` collapsed). Returns `""`
  on inputs that derive to <3 chars; the caller renders a "more specific"
  error.
- `inferQuickArea(repoRoot string) string` scans `.squad/done/*.md` for the
  most-recently-modified item, parses its `area:`, falls back to `"general"`.
- `LearningQuick(ctx, args)` is the shared backend for cobra and MCP. Walks
  `slug-1..slug-9` on `SlugCollisionError`; reuses `LearningPropose` for the
  actual stub write.

`cmd/squad/learning_propose.go` — added optional `Via string` field to
`LearningProposeArgs`. When non-empty, `stubBody` injects
`> captured via squad learning <via>` between the frontmatter and the
section template. Existing `propose` callers don't set it; behaviour is
byte-identical for them.

`cmd/squad/cadence_nudge.go` — added `quickFollowupNudgeText()` and
`printQuickFollowupNudge(io.Writer)` mirroring the BUG-019 text+wrapper
pattern (so MCP carries the same string into Tips).

`cmd/squad/learning.go` — registered `newLearningQuickCmd()`.

`cmd/squad/mcp_register.go` + `cmd/squad/mcp_schemas.go` — registered
`squad_learning_quick` with input schema `{one_liner, kind?, session_id?,
agent_id?}`. Response is `LearningQuickResult` (embeds the propose result
and adds `Tips []string \`json:"tips,omitempty"\``).

`docs/reference/commands.md` — new `### squad learning quick` section with
defaults table.

`plugin/skills/squad-done/SKILL.md` — learning-capture paragraph now
recommends `squad learning quick` as the lowest-friction path, with
`propose` for full control.

### AC renegotiations

Two AC boxes were renegotiated rather than implemented; both are noted
above and explained here:

1. **`# TODO: refine paths` stub marker dropped.** CLAUDE.md is explicit:
   "No future-work TODO comments. If it is worth doing, file it as an
   item." The Via marker already gives reviewers the visual cue that the
   stub came from auto-derivation. Adding a TODO would have been the
   exact failure mode the convention exists to prevent. The AC was
   written before the contradiction with project convention was noticed.
2. **Tier-1 area inference (claims-table closed-row) dropped.** The AC
   Notes specified a three-tier heuristic where tier 1 was "most recent
   `closed` row in `claims` for this `agent_id`." The `claims` table
   actually deletes rows on `Done` (verified by reading
   `internal/claims/`); there is no closed-claim row to query. Tier 2
   (most-recently-modified `done/*.md`) captures the same signal and is
   what `inferQuickArea` implements. The AC was written from a wrong
   model of the schema. A future enhancement could query chat
   `KindDone` posts by this agent for a per-session refinement, but
   that is bigger than this item's scope.

### Coordination

Concurrent peer work hit a real instance of shared-worktree contamination
twice during this claim — agent-1f3f's FEAT-005 Phase 2/3 WIP edits to
`cmd/squad/{claim,handoff}.go` left the package non-building twice. Posted
`stuck` both times; agent-1f3f recovered cleanly. This dogfoods the exact
problem FEAT-005 will solve. agent-bbf6's BUG-019 (MCP nudge parity)
landed mid-claim — my Via field changes survived the merge clean and the
text+wrapper pattern they established is exactly what the MCP exposure
here uses.

### Evidence

```
$ go test ./cmd/squad/ -run "TestLearningQuick|TestDeriveQuickSlug|TestInferQuickArea|TestMCP_LearningQuick|TestMCP_ListsAllTools" -count=1 -v
=== RUN   TestDeriveQuickSlug
--- PASS: TestDeriveQuickSlug (0.00s)
    --- PASS: TestDeriveQuickSlug/plain_words
    --- PASS: TestDeriveQuickSlug/mixed_case
    --- PASS: TestDeriveQuickSlug/punctuation_collapses
    --- PASS: TestDeriveQuickSlug/leading_non-alpha_stripped
    --- PASS: TestDeriveQuickSlug/trailing_dashes_trimmed
    --- PASS: TestDeriveQuickSlug/runs_of_dashes_collapsed
    --- PASS: TestDeriveQuickSlug/max_length_60
    --- PASS: TestDeriveQuickSlug/unicode_dropped_not_transliterated
    --- PASS: TestDeriveQuickSlug/emoji_becomes_dash
--- PASS: TestDeriveQuickSlug_TooShort
--- PASS: TestInferQuickArea_FromMostRecentDoneItem
--- PASS: TestInferQuickArea_FallbackGeneral
--- PASS: TestInferQuickArea_MissingAreaFallsBack
--- PASS: TestLearningQuick_HappyPath
--- PASS: TestLearningQuick_KindOverride
--- PASS: TestLearningQuick_SlugCollisionWalksSuffix
--- PASS: TestLearningQuick_TooShortOneLiner
--- PASS: TestLearningQuick_SilencedEnvNoNudge
--- PASS: TestMCP_LearningQuickRoundTrip
--- PASS: TestMCP_LearningQuickRoundTripSilenced
--- PASS: TestMCP_ListsAllTools
PASS
ok  	github.com/zsiec/squad/cmd/squad
```

Full race-enabled package suite: `go test ./cmd/squad/ -count=1 -race` →
`ok  github.com/zsiec/squad/cmd/squad  56.287s`.
