---
id: FEAT-062
title: A captured item should allow operators to comment inline on selected ranges and submit with comments, with context for regeneration (using claude cli lke auto-refine)
type: feature
priority: P1
area: web, server
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: web
captured_at: 1777345464
accepted_by: web
accepted_at: 1777345730
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777345678
auto_refined_by: claude
---

## Problem

The auto-refine button at `internal/server/web/inbox.js:116` sends the
whole body to claude with no operator steering. When the resulting
draft is close-but-wrong (an AC bullet too vague, the Problem section
misstates intent, or the operator wants claude to focus on a specific
paragraph), the only options today are accept-and-hand-edit or rerun
auto-refine on the full body and hope. The "Send for refinement"
button at `inbox.js:238` is a peer-queue handoff (`/api/items/:id/refine`
→ `handleItemsRefine`), not a claude path, so the operator can't
attach reasoning *and* point claude at specific spans in one shot.

## Context

This item is the **server-side foundation** for a comment-driven
auto-refine flow. The full feature decomposes into:

- **This item**: server endpoint accepts a `comments:
  [{quoted_span, comment}]` payload, threads it into the claude
  prompt, broadens the status gate to allow re-refinement of an
  already-refined item.
- **Follow-up: SPA range-selection UI** (separate item) — replaces
  `openRefineComposer` (`inbox.js:324`) with text-range selection,
  multi-disjoint-range visible highlighting, and comment-attach UI;
  wires to the new server endpoint.
- **Follow-up: peer-queue cleanup** (separate item) — once the SPA
  ships the new flow, decide the fate of `/api/items/:id/refine` +
  `handleItemsRefine`. Either delete (if claude flow supersedes
  every use case) or keep with explicit "use this when claude
  isn't appropriate" framing.

Existing surface to extend:

- `internal/server/items_auto_refine.go:124` — `handleItemsAutoRefine`
  reads the request, resolves the item, builds the prompt via
  `autoRefinePromptFor` at line 299, runs claude, parses the result.
- `internal/server/items_auto_refine.go:299` —
  `autoRefinePromptFor(itemID)` returns the prompt string. The
  comment-rendering logic goes here.
- `internal/items/auto_refine.go:21` — `AutoRefineApply` rewrites
  the body and stamps. Untouched by this item; DoR validation it
  already enforces continues to apply.
- `internal/server/items_auto_refine.go:153` — captured-only
  precondition. This item broadens the gate to
  `{captured, needs-refinement, open}` so a draft that auto-refine
  flipped to `open` can be re-refined with comments.

Per direction: comment-driven refinement applies on any
non-in-progress status. Items in `in_progress` (held by an agent)
keep the existing protection (no concurrent body rewrites).

## Acceptance criteria

- [ ] `autoRefinePromptFor` (or its successor) accepts an optional
      slice of `{quoted_span string; comment string}` pairs and
      renders each pair as a `> <quoted span>\n<operator comment>`
      block above the existing instruction text. With an empty
      slice the prompt output is byte-identical to today's prompt.
- [ ] A Go unit test pins the rendering: given a fixture item id
      and two `{quoted_span, comment}` pairs, the constructed
      prompt contains both quoted spans and both comments in input
      order. The empty-comments case matches the current prompt
      exactly (regression guard).
- [ ] `handleItemsAutoRefine` accepts an optional
      `comments: [{quoted_span, comment}]` field on the JSON
      request body. Non-empty comments thread through to the
      prompt. An empty / missing field preserves today's behavior.
- [ ] The handler's status precondition broadens from
      `captured` only to `{captured, needs-refinement, open}`. An
      `in_progress` item still returns 409 (no concurrent body
      rewrites against a held claim). An HTTP test pins each of
      the four cases (200 on captured, 200 on needs-refinement,
      200 on open, 409 on in_progress).
- [ ] An HTTP integration test exercises the comment-driven path
      end-to-end: a captured item + two comments goes in, the
      resulting body is non-empty and differs from the original,
      and the response carries `auto_refined_at` /
      `auto_refined_by` stamps. Mocked subagent runner; no real
      claude invocation.
- [ ] DoR validation enforced by `AutoRefineApply` continues to
      reject template-placeholder bodies returned by the runner.
      An integration test seeds a runner that returns a
      placeholder body (`Specific, testable thing 1`) and asserts
      the apply path rejects it, leaving the original body
      untouched.

## Notes

- This item is server-side only. SPA changes are deferred to the
  range-selection UI follow-up.
- The two follow-ups will be filed as separate FEATs and listed in
  this item's resolution. They depend on this contract.
- Comments are not persisted on the item — they're operator
  context for one prompt invocation. If persistence becomes
  necessary, that's a separate item.
- The "needs-refinement" status today routes to the peer-queue
  flow. Once this lands, an item in needs-refinement can be
  re-refined by claude with comments instead of (or in addition
  to) waiting for a peer. The peer-queue flow stays intact for
  this item; cleanup is the third follow-up.

## Resolution
(Filled in when status → done.)
