---
id: FEAT-065
title: decide fate of legacy peer-queue refine flow once claude flow ships
type: feature
priority: P3
area: server
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777346119
accepted_by: web
accepted_at: 1777346390
references: []
relates-to: []
blocked-by: [FEAT-062, FEAT-064]
---

## Problem

`/api/items/:id/refine` (handler `handleItemsRefine` in
`internal/server/items_refine.go:12`) routes a captured item to a
peer-refinement queue: another agent later runs `squad refine`,
edits markdown directly, and recaptures. Once the comment-driven
claude-CLI flow ships (FEAT-062 server, FEAT-064 SPA), this
peer-queue path is no longer the only option. Its remaining use
case — and whether it's worth keeping — needs an explicit
decision before the dual-flow ambiguity ossifies.

## Context

After FEAT-064 lands, the SPA's "Send for refinement" button drives
the claude flow. Nothing in the SPA still hits
`/api/items/:id/refine`. The peer queue continues to function
indirectly via `squad refine` / the items state machine, but no
operator surface invokes it. Three plausible outcomes:

1. **Delete.** Remove `handleItemsRefine`, the `/api/items/:id/refine`
   route, the `squad refine` CLI command if it has no other
   non-route caller, and the `needs-refinement` status entirely if
   it becomes vestigial. Risk: loses the "send to a human/peer
   reviewer" affordance. Reward: simpler model.
2. **Keep, document.** Keep the endpoint and CLI but add a
   one-line comment near the SPA's removed-flow site explaining
   when to use it (e.g. "claude flow not appropriate for this
   item — use `squad refine <ID>` from CLI"). No code removal.
3. **Repurpose.** Surface the peer-queue flow under a different SPA
   button (e.g. "Send to peer reviewer") that's distinct from
   "Send for refinement". Forks the UI surface; doubles cognitive
   load for operators.

This item picks one and ships the corresponding cleanup.

## Acceptance criteria

- [ ] An explicit decision is made and recorded in the resolution:
      delete / keep-document / repurpose. The decision references
      whether `squad refine` (CLI) has other callers beyond the
      now-removed SPA wiring.
- [ ] If delete: `/api/items/:id/refine` route is removed,
      `handleItemsRefine` and its tests are removed, and any
      orphaned status-machine paths around `needs-refinement` are
      either removed or documented as preserved for a different
      reason.
- [ ] If keep-document: a comment in the SPA at the former
      `openRefineComposer` site explains the alternative path. No
      backend changes.
- [ ] If repurpose: a separate SPA button surfaces the peer-queue
      flow with distinct labelling, and the AC for "Send for
      refinement" stays bound to the claude flow.
- [ ] Whatever is decided, the item ledger reflects the new state
      — no stale references to the removed path in skills,
      AGENTS.md, or CLAUDE.md.

## Notes

- Blocked by FEAT-062 and FEAT-064. Decision can't ship until the
  claude-driven flow is live and observable; otherwise the
  comparison is theoretical.
- The peer-queue flow has a `squad refine` CLI command. If that
  command has independent users (CLI scripts, hooks), delete is
  not safe — keep-document is the floor.
- This item is small (1h estimate) but high-leverage: dual-flow
  ambiguity is a known tax on operator decision-making. Pick a
  shape and commit.

## Resolution
(Filled in when status → done.)
