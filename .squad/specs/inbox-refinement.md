---
title: Inbox refinement loop — captured items can be sent back for refinement with reviewer comments
motivation: |
  Today the inbox modal forces a binary choice: accept (promote to ready) or reject (terminal kill).
  Items that are nearly-right-but-not-quite — bad area, fuzzy AC, missing context — have no middle path.
  This spec adds a third inbox action that captures reviewer comments and routes the item to a refining
  agent who edits the body, then sends it back to the inbox for human review.
acceptance:
  - A reviewer can click 'Send for refinement' in the inbox modal, type comments, and submit. The item flips to a new status 'needs-refinement' and disappears from the inbox.
  - A peer agent can list needs-refinement items via 'squad refine', claim one with the existing 'squad claim', edit the markdown body to address the feedback, then run 'squad recapture' to bounce it back to the inbox.
  - Multi-round trips preserve history — the working '## Reviewer feedback' section moves into '## Refinement history' on each recapture; the next round starts with a clean feedback section.
  - Existing accept/reject paths are unchanged.
non_goals:
  - Auto-refinement by the squad binary (no LLM call from the binary itself).
  - A 'refiner' agent role — squad stays role-less.
  - Putting needs-refinement items on the regular 'squad next' ready stack.
  - Per-AC inline comments — feedback is a single freeform section.
  - Notification fan-out beyond the existing inbox_changed SSE event.
integration:
  - internal/items/ — body-rewrite parser (WriteFeedback, MoveFeedbackToHistory)
  - internal/server/ — three new HTTP handlers and a list endpoint, plus an SSE event extension
  - internal/store/ — additive status enum value if a CHECK constraint exists
  - cmd/squad/ — three new CLI verbs (refine, refine list, recapture)
  - internal/server/web/ — Refine button + inline composer in the inbox modal
  - plugin/skills/squad-loop/, internal/scaffold/templates/AGENTS.md.tmpl, docs/recipes/ — refinement-loop documentation
---

## Background

Captured items live in `.squad/items/<ID>-*.md` with `status: captured`. The inbox modal in
`internal/server/web/inbox.js` is the human triage surface. BUG-006 just added an inline detail
panel that lets reviewers read the full item body before deciding — this spec extends that surface
with a third action that closes the loop on "this needs work, but isn't dead."

The full design is captured in
`/Users/zsiec/dev/switchframe/docs/plans/2026-04-26-squad-inbox-refinement-design.md` and the
implementation plan in
`/Users/zsiec/dev/switchframe/docs/plans/2026-04-26-squad-inbox-refinement.md` (gitignored, per
project convention).
