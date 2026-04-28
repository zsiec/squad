---
id: FEAT-064
title: spa range-selection ui for comment-driven auto-refine
type: feature
priority: P1
area: spa
status: done
estimate: 4h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777346119
accepted_by: web
accepted_at: 1777346371
references: []
relates-to: []
blocked-by: [FEAT-062]
---

## Problem

The "Send for refinement" button at `internal/server/web/inbox.js:238`
currently triggers `openRefineComposer` (line 324), which opens a
single-textarea composer and posts to the peer-queue endpoint
`/api/items/:id/refine`. With the server-side comment-driven
auto-refine landing under FEAT-062, the SPA needs a range-selection
+ inline-comment UI that wires to the new claude-driven contract.

## Context

The body is rendered at `inbox.js:228` via
`<div class="md">${renderMarkdown(it.body_markdown)}</div>` inside
the inbox details panel. Range selection on rendered HTML requires
the DOM Range/Selection APIs and a way to map selected text back to
the source markdown so the server receives clean `quoted_span`
strings.

The new server endpoint accepts
`{comments: [{quoted_span, comment}]}` per FEAT-062. The empty-list
case is byte-identical to today's auto-refine, so the existing
auto-refine button at `inbox.js:116` can stay unchanged — this item
only replaces the line 238 manual-refine flow.

## Acceptance criteria

- [ ] The line 238 "Send for refinement" button (still labelled
      that) opens an inline composer that lets the operator
      select text inside the rendered body (`<div class="md">`)
      and click "Add comment" on the selection. Multiple
      disjoint selections per item are supported.
- [ ] Each commented range is visibly highlighted in the body
      panel (CSS class on a wrapping `<span>`). Hovering or
      clicking a highlight reveals the attached comment with
      delete/edit affordances.
- [ ] A `Send` button submits to the FEAT-062 endpoint
      (`POST /api/items/:id/auto-refine`) with payload
      `{comments: [{quoted_span, comment}, ...]}`. With zero
      comments attached, the request omits the field — same
      shape as today's auto-refine.
- [ ] On a 200 response, the inbox row re-renders via
      `replaceRow` with the `auto-refined` badge, the highlights
      and comment overlays clear, and a toast confirms
      `Refined <ID>`.
- [ ] On non-200 (502/500/etc.), the existing
      `autoRefineToastForStatus` paths apply — comments stay
      attached so the operator can retry.
- [ ] Structural Go test (same `webFS.ReadFile` pattern as
      `repo_badge_css_test.go`'s
      `TestRepoBadgeCssIsDistinctlyStyled`) pins that
      `inbox.js` references the FEAT-062 contract:
      `comments` field name + `quoted_span` field name + the
      `/api/items/:id/auto-refine` POST URL. The legacy
      `/api/items/:id/refine` POST should NOT appear in the
      replaced flow's code path.

## Notes

- Mapping rendered HTML selections back to source markdown is the
  hardest part. A pragmatic option: store the selected text
  verbatim (what the operator sees) as `quoted_span` rather than
  trying to round-trip through markdown. Claude reads it as
  context, not as a textual replace target, so byte-perfect source
  alignment isn't required.
- `openRefineComposer` (`inbox.js:324`) is replaced wholesale.
  The legacy peer-queue endpoint stays alive until FEAT-065
  decides its fate.
- Blocked by FEAT-062 — the server contract this UI hits doesn't
  exist yet.

## Resolution
(Filled in when status → done.)
