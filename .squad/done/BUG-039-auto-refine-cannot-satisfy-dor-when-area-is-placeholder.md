---
id: BUG-039
title: auto-refine cannot satisfy DoR when area is placeholder
type: bug
priority: P1
area: refine
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777325912
accepted_by: web
accepted_at: 1777325988
references: []
relates-to: []
blocked-by: []
---

## Problem
Auto-refine rewrites only the item body, not the frontmatter. Items captured through the SPA inbox land with `area: <fill-in>` (a placeholder), and the DoR `area-set` rule rejects any apply where area is still the placeholder. As a result, every newly-captured item that goes through the Auto-refine button produces `claude exited without drafting; run again` in a loop — the underlying apply is rejected by DoR, the loop never converges.

## Context
The auto-refine handler lives at `internal/server/items_auto_refine.go`; the apply tool at `cmd/squad/auto_refine_apply.go`; the file-mutation logic at `internal/items/auto_refine.go`. The current contract of `squad_auto_refine_apply` is `(item_id, new_body)` — body-only by design. The DoR `area-set` rule lives in `internal/items/dor.go`. Verified end-to-end against BUG-031 with the daemon timeout fix in place: claude completes in ~111s, calls apply, apply rejects, handler returns 500.

## Acceptance criteria
- [ ] `squad_auto_refine_apply` MCP tool accepts an optional `area` string
- [ ] `items.AutoRefineApply` writes `area` to the frontmatter when supplied
- [ ] `autoRefinePromptFor` instructs claude to choose a free-form area string from item title and context, and pass it to apply
- [ ] DoR `area-set` rule passes for items where claude supplied an area via auto-refine
- [ ] Existing area-not-supplied path still works (back-compat)
- [ ] BUG-031 refines successfully through the SPA Auto-refine button after the fix lands

## Notes
Free-form area chosen for v1; tightening to "prefer sibling areas" requires extra MCP roundtrips and can ride a follow-up. The 500 message ("claude exited without drafting; run again") is misleading for any DoR-rejection path — separate UX bug worth filing.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
