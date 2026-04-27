---
id: FEAT-004
title: squad handoff --propose-from-surprises — auto-draft learnings from chat history
type: feature
priority: P2
area: cli
status: done
estimate: 2.5h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777250600
accepted_by: agent-401f
accepted_at: 1777250600
references:
  - cmd/squad/handoff.go
  - internal/chat/handoff.go
  - internal/chat/history.go
  - cmd/squad/learning_propose.go
relates-to:
  - FEAT-003
  - TASK-019
blocked-by:
  - FEAT-003
---

## Problem

`squad handoff` already collects `--surprised-by` strings — exactly the raw
material a gotcha-learning is made of — but they live and die in the chat
record. Nothing turns them into a proposed learning. Combined with the chat
verbs `stuck` (which marks "this hit a wall") and `fyi` (which agents use for
discovery posts), the claim's history is a rich source of unfiled learnings
the agent already wrote.

The complementary item FEAT-003 fixes propose-time friction with a `quick`
verb. This item fixes the **end-of-claim cleanup** path — auto-draft a
proposal from what's already in chat, agent confirms or discards. Net new
data captured per claim should rise without any extra typing.

## Context

`cmd/squad/handoff.go:17-69` defines the cobra command; `Handoff` (line 98)
posts the handoff body and releases held claims. `chat.HandoffBody`
(`internal/chat/handoff.go:10`) carries `SurprisedBy []string`.

Chat history retrieval already exists: `internal/chat/history.go:13` exposes
`History(ctx, itemID)` returning ordered entries with `Kind` and `Body`.
`Kind` enum is in `internal/chat/kinds.go`: `KindStuck`, `KindFYI`,
`KindThinking`, `KindMilestone` are the relevant ones for surprise-mining.

Learning stub generation lives in `cmd/squad/learning_propose.go:147-176`
(`stubBody`). `LearningPropose` (line 53) takes the kind, slug, title, area,
session, paths, agent — the inputs we already have at handoff time.

## Acceptance criteria

- [ ] New flag `--propose-from-surprises` on `squad handoff`. Boolean. When
  set, the handoff runs as today AND emits one proposed learning per
  `surprised_by` entry on the held claim's chat history.
- [ ] When `--propose-from-surprises` is set with NO `--surprised-by` flags
  passed explicitly, gather candidate surprises automatically from the
  agent's claim chat history:
  - all `KindStuck` posts on threads owned by claims this agent currently holds
  - the body text of `KindFYI` posts from this session whose body contains
    the words "surprise"/"surprised"/"didn't expect"/"turns out"/"wait"
    (case-insensitive)
  - de-duplicate by simple lowercase substring containment so near-duplicates
    don't produce two proposals.
- [ ] For each surprise:
  - kind = `gotcha` (it's a surprise; that's what gotchas are for)
  - title = first 80 chars of the surprise body
  - slug = derive via the same helper FEAT-003 introduces (strict reuse — do
    NOT re-implement)
  - area = inferred from the held claim's frontmatter `area` field
  - the stub body's `## Looks like` section is pre-filled with the raw
    surprise body verbatim (so the agent isn't staring at a blank template)
- [ ] After writing the proposals, print a numbered list to stdout:
  ```
  drafted 3 learning proposals — review with `squad learning list --proposed`
    1. .squad/learnings/gotchas/proposed/<slug-1>.md
    2. ...
  ```
- [ ] Honor `--dry-run`: print what WOULD be proposed (one per surprise, with
  derived slug + title + area) but write nothing. Useful for the agent to
  preview before committing.
- [ ] Honor `--max N` (default 5): cap auto-proposals at N. A handoff that
  wants to file >N is a signal to switch to `learning quick` per surprise
  manually — print a one-line warning when the cap clips the list.
- [ ] If `--propose-from-surprises` finds zero candidates, the handoff
  proceeds normally and prints `no surprises to propose from` on stderr (not
  an error). Don't fail the handoff over an empty mining run.
- [ ] Unit tests in `cmd/squad/handoff_test.go` (or new `handoff_propose_test.go`):
  - explicit surprises path: `--surprised-by "X" --surprised-by "Y" --propose-from-surprises` writes 2 stubs with X and Y bodies.
  - mined-from-history path: seed chat with one `stuck` and two matching `fyi` entries, no `--surprised-by`; expect 3 stubs (or 2 after dedupe).
  - dry-run path: nothing written, list printed.
  - max-cap path: 7 candidates with `--max 5` writes 5, prints warning.
  - zero candidates path: handoff succeeds, no stubs, message on stderr.
- [ ] Integration test: a fixture run that claims an item, posts two `stuck`
  messages with distinct bodies, runs `squad handoff --propose-from-surprises`,
  asserts both stubs land in `.squad/learnings/gotchas/proposed/` with the
  expected `area` from the claim's item.
- [ ] Update `docs/reference/commands.md` `handoff` section. Update the
  handoff skill prose at `plugin/skills/squad-handoff/SKILL.md` to recommend
  the flag.
- [ ] `go test ./...` passes; trailing summary pasted at close-out.

## Notes

- Risk medium because the mining heuristic (KindFYI body match) is fuzzy and
  could produce noisy proposals. Mitigations: keep the substring set small
  and behind the explicit flag; cap at `--max 5`; the agent reviews the
  numbered list and can `squad learning reject` any miss before the next
  handoff.
- This depends on FEAT-003's slug-derivation helper. **Don't ship before
  FEAT-003** — duplicating the helper would split the contract.
- MCP exposure: the handoff MCP tool gains an optional `propose_from_surprises`
  bool param; the response payload's `Tips` (per BUG-019 shape) gains the
  numbered list of drafted paths.
- The auto-drafted stubs live in the `proposed/` state — they are not yet
  approved. The existing `squad learning approve` flow gates them. No new
  state machine.
- Consider this a stepping stone toward trace-driven auto-proposal once OTel
  emission lands (separate Tier 1 work). The chat-verb mining heuristic is
  cheap and works today.

## Resolution
(Filled in when status → done.)
