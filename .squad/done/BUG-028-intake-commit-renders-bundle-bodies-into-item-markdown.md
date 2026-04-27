---
id: BUG-028
title: intake commit renders bundle bodies into item markdown
type: bug
priority: P1
area: internal/intake
status: done
estimate: 3h
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
epic: refinement-and-contract-hardening
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

The intake commit path for item-only and refine bundles drops every body
field the agent typed during the interview. `internal/intake/commit_run.go`
lines 92 and 109 build `items.Options{Ready: ready, CapturedBy: agentID}`
and pass it to `items.NewWithOptions`, which writes the unmodified
`stubTemplate` from `internal/items/new.go`. The bundle's `Intent`,
`Acceptance`, `Area`, and `Notes` are marshaled into the
`intake_sessions.bundle_json` blob and then thrown away as far as the item
file is concerned.

## Context

The interview's whole value proposition is "answer questions once, get a
filled-in item." Today the agent answers the questions and then opens the
generated file to find `Specific, testable thing 1` — exactly the placeholder
they would have gotten from `squad new` with no interview at all. Two
recently-completed items (the intake interview itself, FEAT-024 / FEAT-026)
shipped this gap because the spec/epic path was the focus; the item-only
path was assumed to "work the same" and never verified end-to-end.

The companion path at `internal/intake/commit_spec_epic.go` already does
the right thing for spec and epic files via `writeSpecFile` and
`writeEpicFile` (lines 193-219), formatting the bundle content into the
file body. Item bodies are the missing piece. `internal/items/new.go`
exposes an `Options` struct (lines 76-91) that today carries `Priority`,
`Estimate`, `Risk`, `Area`, `Ready`, `CapturedBy`, plus the hierarchy
linkage; it needs three new fields for the body content. `commit.go`
defines `ItemDraft` (lines 46-56) with `Intent`, `Acceptance`, `Area`,
`Kind`, `Effort` — those are the source.

This is the foundational fix for the rest of the epic: `FEAT-036`'s DoR
heuristic and `FEAT-037`'s decompose nudge both inspect item body content
that doesn't exist until this lands.

## Acceptance criteria

- [ ] `items.Options` (in `internal/items/new.go`) gains fields for `Area`,
      `Intent` body text, `Acceptance` bullets ([]string), and a `Kind` /
      priority knob sufficient to render the per-type defaults.
- [ ] `commitImpl` in `internal/intake/commit_run.go` populates those fields
      from each `ItemDraft` before calling the item writer, and the writer
      renders the `Intent` into the `## Problem` and `## Context` sections,
      the `Acceptance` bullets into the `## Acceptance criteria` block, and
      any `Notes` into the `## Notes` block.
- [ ] Refine mode preserves the original item's body in a `## Refinement
      history` section on the new item when refinement happens via
      interview, so the interview-driven supersede doesn't lose prior
      context.
- [ ] New regression test in `internal/intake/commit_run_test.go`: a bundle
      with `Intent: "Test intent"` and `Acceptance: ["Specific bullet"]`
      produces an item file whose body contains those exact strings, and
      does not contain the literal template placeholder
      `Specific, testable thing 1`.

## Notes

The simplest implementation extends `stubTemplate` in `internal/items/new.go`
to accept body sections via the existing `Options` struct and falls back to
the placeholder text when the option is empty (so `squad new` with no flags
keeps emitting today's stub). Resist the urge to introduce a separate
"interviewed item" template — same output shape, conditional content.

Refine mode currently runs `supersedeOriginal` which archives the old item
file. Capturing that body into `## Refinement history` on the new item must
happen before the old file is archived; the easiest seam is to read the
parsed body during `supersedeOriginal` and thread it back to the writer
call site, or read the original file before the commit transaction begins.

## Resolution
- `internal/items/new.go`: split the single `stubTemplate` into a frontmatter
  template + a `renderBody(opts)` builder. Added `Intent`, `Acceptance`,
  `Notes`, and `RefinementHistory` to `Options`; empty fields fall back to the
  prior placeholder text so `squad new` with no flags is byte-identical.
- `internal/intake/commit_run.go`: `commitImpl` now builds a per-item
  `Options` from each `ItemDraft` (Intent, Acceptance, Area, Estimate). Added
  `captureRefineHistory` which reads the original item body before the tx and
  hands it to the writer wrapped as `### Round 1 — YYYY-MM-DD`, with `##`/`###`
  headings demoted by one level so the document hierarchy stays clean.
- `internal/intake/commit_spec_epic.go`: same wiring on the spec/epic path.
- Regression tests in `commit_run_test.go`: `TestCommit_ItemOnlyRendersBundleIntoBody`
  (asserts Intent/Acceptance text appears and the literal placeholder is gone)
  and `TestCommit_RefinePreservesOriginalBodyInRefinementHistory` (asserts
  the prior body, attribution, and demoted heading are present).

Note: `ItemDraft` has no `Notes` field today, so the `Options.Notes` plumbing
is dormant on the intake side. The field is wired through to the renderer so
the moment the interview starts capturing per-item notes the data lands in
the file with no further changes here. Intent fills both `## Problem` and
`## Context` per literal AC; the next agent claiming the item can split or
trim as they investigate.
