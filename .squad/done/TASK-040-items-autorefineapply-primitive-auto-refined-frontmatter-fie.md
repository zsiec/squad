---
id: TASK-040
title: items.AutoRefineApply primitive + auto_refined frontmatter fields
type: task
priority: P2
area: items
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308127
accepted_by: web
accepted_at: 1777308350
epic: auto-refine-inbox
references:
  - internal/items/persist.go
  - internal/items/rewrite.go
  - internal/items/items.go
relates-to: []
blocked-by: []
---

## Problem

The auto-refine epic needs an atomic "rewrite a captured item's body and stamp `auto_refined_at` / `auto_refined_by`" primitive in `internal/items/`. Today the only body-rewrite primitive is the manual-refine path (`WriteFeedback`, `MoveFeedbackToHistory`); it appends to a Reviewer feedback section and does not touch the body wholesale, and it does not stamp any auto-refine audit fields.

## Context

`internal/items/rewrite.go` already has `rewriteFrontmatter` (used by `Promote` and `Recapture`) and `atomicWrite` for safe file replacement. The Item struct in `internal/items/items.go` carries the existing audit fields (`captured_by`, `captured_at`, `accepted_by`, `accepted_at`); we add two more (`auto_refined_at int64`, `auto_refined_by string`). The persistence layer (`internal/items/persist.go`) writes the SQLite mirror — auto-refine fields can either ride along or stay file-only. Decide at impl time; cheapest is file-only (no migration), but the inbox JSON propagation (FEAT-032) gets simpler if the DB has them.

## Acceptance criteria

- [ ] `Item` struct in `internal/items/items.go` gains `AutoRefinedAt int64` and `AutoRefinedBy string` fields with the matching `auto_refined_at` / `auto_refined_by` YAML tags; `Parse` round-trips both.
- [ ] New exported `items.AutoRefineApply(squadDir, itemID, newBody, refinedBy string) error` rewrites the item file's body section (everything after the frontmatter `---` boundary) with `newBody` and updates the frontmatter `auto_refined_at` (unix now) and `auto_refined_by` fields atomically; the file is held under the items lock for the duration.
- [ ] `AutoRefineApply` refuses (returns an error) if the item's status is not `captured`; this preserves the captured→open human-only contract.
- [ ] `AutoRefineApply` validates that `newBody` is non-empty and that `items.DoRCheck` on the parsed (frontmatter+body) result returns zero violations; if DoR fails the file is not touched and the error names the failing rule(s).
- [ ] Unit tests cover: round-trip of the new fields through `Parse` / persist; happy-path body rewrite stamps both audit fields; rewrite refuses on non-captured status; rewrite refuses when newBody fails DoR; concurrent calls serialize via the items lock.
- [ ] No existing items test regresses; in particular `TestDoRCheck` and the parse round-trip suite stay green.

## Notes

Decision deferred: whether `auto_refined_at` / `auto_refined_by` also persist to the SQLite items mirror. If yes, additive migration v10 + persist.go change. If no (file-only), FEAT-032 reads them by re-parsing the file each request (acceptable — the inbox already does small-file IO). Recommend file-only for v1; revisit if dashboard hot-paths show cost.
