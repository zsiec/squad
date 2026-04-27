---
id: FEAT-036
title: squad accept rejects vague AC bullets
type: feature
priority: P2
area: internal/items
status: done
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
blocked-by: [BUG-028]
parent_spec: agent-team-management-surface
epic: refinement-and-contract-hardening
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

`squad accept`'s Definition of Ready check (BUG-027,
`internal/items/dor.go`'s `template-not-placeholder` rule) only flags AC
bullets that match the literal template strings `Specific, testable thing 1`
/ `Specific, testable thing 2`. A bullet like `works correctly` or `make it
better` clears the gate but is no more refutable than the placeholder it
replaced. The result is items that pass `accept` and then trigger the
refinement loop downstream — which logs show is reached for a tiny fraction
of items, because agents route around it once an item is "ready."

## Context

The DoR rules already in `internal/items/dor.go` are the right home: the
file defines `DoRViolation`, the regexes for walking the AC section
(`acSection`, `dorCheckboxLabelRe`), and the existing
`template-not-placeholder` rule. Extending the same check to fire on vague
bullets keeps the heuristic in one place and reuses the parsing.

Goal: AC bullets must be propositions a test can refute. The three cheapest
proxies for that are bullet length (very short bullets cannot describe a
falsifiable condition), absence of a verb (a bullet that's just a noun
phrase like `the table` is descriptive, not a proposition), and equality
with the item title (restating the title without adding falsifiable detail).

The check should NOT fire on `chore` or `docs` items — those genuinely have
soft criteria like `update README to mention X`, and forcing a verb count on
them would generate noise. `Item` already carries `Type`, so gating is a
single field check at the top of the new rule.

## Acceptance criteria

- [ ] New DoR rule in `internal/items/dor.go` rejects any AC bullet shorter
      than 6 words.
- [ ] Same rule rejects bullets that contain no verb. A small allow-list of
      common verbs (`is`, `are`, `should`, `must`, `returns`, `prints`,
      `rejects`, `accepts`, `creates`, `writes`, `reads`, `parses`,
      `validates`, `fails`, `succeeds`, etc.) is sufficient — we are not
      doing real NLP.
- [ ] Same rule rejects bullets equal (case-insensitive, whitespace-trimmed)
      to the item title.
- [ ] Rule is gated on `it.Type == "feature" || it.Type == "bug"`; `chore`
      and `docs` items skip the check entirely.
- [ ] Reject `DoRViolation.Message` names the offending bullet text and the
      specific sub-rule it violated (e.g. `bullet "works correctly" too
      short — at least 6 words required`).
- [ ] Tests in `internal/items/dor_test.go` cover each sub-rule with a
      passing and failing fixture, and confirm the chore/docs gate.

## Notes

Keep the verb list closed and short. The temptation is to grow it into a
WordNet-lite — don't. False positives are the failure mode here; the
template-placeholder rule worked because it had zero false positives, and
this extension has to maintain that bar or agents will start adding noise
verbs to clear the check.

A reasonable rule name: `vague-acceptance-bullet`, with three distinct
`Message` strings keyed by which sub-rule fired. Keep `Field` as `body`
to match the existing convention.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
