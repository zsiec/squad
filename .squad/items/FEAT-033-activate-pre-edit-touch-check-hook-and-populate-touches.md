---
id: FEAT-033
title: activate pre_edit_touch_check hook and populate touches
type: feature
priority: P2
area: plugin/hooks
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309557
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: coordination-defaults-opinionated-opt-out
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

The `pre_edit_touch_check.sh` hook exists and is registered on
PreToolUse for Edit/Write at `plugin/hooks.json:102-110`, but the
`touches` table stays empty in the audited session because the hook
only READS conflicts via `squad touches policy <path>`
(`cmd/squad/touches_policy.go:46` calls `Tracker.Conflicts`) — it never
writes a touch row. As a result `Tracker.ListOthers`
(`internal/touch/touch.go:122`) has nothing to return and the warn /
deny path is dead code in practice.

## Context

The touch ledger is SQL-backed (the `touches` table; `internal/touch/touch.go:75`
is the INSERT) — there is no on-disk `.squad/touches/<agent>/<file>.touch`
artifact, so the audit's "directory is empty" observation is really
"the table has zero rows for this repo." The verbs that write are
`squad touch` (`cmd/squad/touch.go:44`) and `squad claim --touches=...`
(`cmd/squad/claim.go:182`). Neither runs from the Edit/Write hook today.

The fix is to extend the PreToolUse Edit/Write path so that, in addition
to emitting the existing conflict-check JSON, it records a touch row
keyed on the active claim's item id for the file Claude is about to
edit. Release semantics already cascade — `internal/claims/release.go:45`
sets `released_at` on every touch held by the agent when their claim
closes, so claim release and `squad done` already clean up after the
new write path.

## Acceptance criteria

- [ ] `plugin/hooks.json` keeps the Edit|Write PreToolUse registration
      for `pre_edit_touch_check.sh` (already there at
      `plugin/hooks.json:102-110`) — verify with a test that asserts the
      matcher and command path.
- [ ] First Edit/Write under a claim writes a row to the `touches`
      table for (agent, item, file). The hook (or a verb the hook
      calls) does the INSERT; the existing `squad touches policy`
      stdout JSON shape is preserved so Claude Code's PreToolUse
      contract stays intact.
- [ ] When the claim is released or closed, the touch row's
      `released_at` is set — covered by the existing
      `internal/claims/release.go:45` UPDATE; add a regression test
      that exercises the hook→touch→release cycle end-to-end.

## Notes

Implementation likely extends `cmd/squad/touches_policy.go` so the
verb both checks for conflicts AND records the touch in one round-trip,
or adds a sibling verb the hook script calls in sequence. Whichever
shape, keep the JSON-on-stdout contract for the existing call so the
PreToolUse handshake doesn't regress.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
