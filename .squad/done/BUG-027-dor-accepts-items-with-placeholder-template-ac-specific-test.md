---
id: BUG-027
title: DoR accepts items with placeholder template AC ("Specific, testable thing 1/2")
type: bug
priority: P2
area: intake
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777305641
accepted_by: web
accepted_at: 1777305921
references: []
relates-to: []
blocked-by: []
---

## Problem

The Definition of Ready check (`internal/items/dor.go`, rule `acceptance-criterion`) requires "≥1 `- [ ]` checkbox under `## Acceptance criteria`." The squad-new template ships with two placeholder checkboxes — *"- [ ] Specific, testable thing 1"* and *"- [ ] Specific, testable thing 2"*. Those placeholder boxes mechanically satisfy the rule even though semantically the item is unshaped.

Discovered in this session: CHORE-012 was accepted from the web dashboard with the template body untouched. `squad accept` returned success, the item flipped to `open`, and `squad next` happily handed it out for claim. The agent that picked it up had no real contract — only the title and the placeholder checkboxes — and had to flesh out the AC mid-claim before doing the work.

## Context

This is a DoR escape hatch, not a code bug per se. The current rule is "is there at least one `- [ ]` line?" — the simplest possible test. The fix is to tighten the rule so template-default content fails it.

The simplest sharpening: reject AC where every checkbox label exactly matches the squad-new template ("Specific, testable thing 1", "Specific, testable thing 2"). A more thorough sharpening: reject AC where ALL three template sections (`## Problem`, `## Context`, `## Acceptance criteria`) still contain their template prose unchanged — i.e., the body is the unmodified scaffold.

The template sentinels live in `internal/scaffold/templates/` (or wherever `squad new` writes new items). Any DoR rule should pull its sentinel strings from the same source so they stay in sync if the template ever changes.

## Acceptance criteria

- [ ] `internal/items/dor.go` gains a rule (suggested name: `template-not-placeholder`) that fails when the body's checkboxes match the squad-new template defaults verbatim.
- [ ] The rule's sentinel strings ("Specific, testable thing 1", "Specific, testable thing 2") are sourced from the same template constants `squad new` uses, so they don't drift.
- [ ] `squad accept <id>` of a new-from-template item refuses with the rule's violation message; the item stays `captured`.
- [ ] `squad accept <id>` succeeds for items whose checkboxes have been replaced with real content, even if `## Problem` / `## Context` / `## Notes` still contain template prose (those sections are advisory, only AC is the contract).
- [ ] Existing tests in `internal/items/dor_test.go` continue to pass; new tests cover the placeholder-rejection path.
- [ ] `squad ready --check <id>` surfaces the new violation under its rule name.
- [ ] The dashboard's "Accept" button respects the new rule (it calls the same DoR code path, so this should be free).

## Notes

The Problem and Context sections of THIS item are also written from-scratch — meta-evidence that authors who care write real prose, while authors who don't get caught by the AC rule alone. Tightening AC is the highest-signal fix; tightening the prose sections would over-fit and frustrate fast capture.

Related to BUG-026 (also filed this session) only in that both are post-FEAT-028 follow-ups; otherwise unrelated.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
