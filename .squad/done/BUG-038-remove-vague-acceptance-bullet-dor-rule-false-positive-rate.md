---
id: BUG-038
title: remove vague-acceptance-bullet DoR rule — false-positive rate too high to justify the friction
type: bug
priority: P2
area: internal/items
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777324559
accepted_by: ""
accepted_at: 0
references: []
relates-to: [FEAT-036, BUG-033]
blocked-by: []
---

## Problem

The `vague-acceptance-bullet` DoR rule introduced by FEAT-036 produces
false positives at a rate that fails its own design bar. FEAT-036's
Notes warned: *"False positives are the failure mode here; the
template-placeholder rule worked because it had zero false positives,
and this extension has to maintain that bar or agents will start adding
noise verbs to clear the check."* The differential corpus sweep run
during BUG-033 confirms the bar is not met: even after expanding the
verb allow-list with 30+ additions (motion / state-transition /
regression cluster), 130 of 141 historical items still trip the rule —
a 92% false-positive rate against bullets that were judged ready when
written. Authors hit the rule on plainly-testable propositions
("the reproducer test goes from FAIL to PASS"), and the rule pushes
them toward verb-laundering, the exact noise the design tried to
prevent.

## Context

`internal/items/dor.go` defines `vagueACBulletViolations` (line 112)
backed by a closed allow-list `vagueACBulletAllowedVerbs` (line 90).
Three sub-rules fire under one rule name: bullet shorter than 6 words,
no recognized verb, equal to title.

The verb sub-rule is the load-bearing source of false positives. English
has thousands of common verbs; any closed allow-list will reject real
propositions. BUG-033's expansion (commit 06f749b) cut violations by
~10% but the rate remains untenable.

The other two DoR rules (`template-not-placeholder`, `area-set`,
`title-or-problem`) were not implicated and continue to earn their
keep. The `acceptance-criterion` rule (no checkbox) is also fine.

This bug removes only the `vague-acceptance-bullet` rule. The
title-equality sub-check and word-count sub-check are bundled with the
verb check under one rule name and one violation type; we remove the
whole rule rather than carving up the bundled implementation, since
the title-equality pattern is rare in practice (the
`template-not-placeholder` rule already catches the common case where
the template is left untouched).

## Acceptance criteria

- [ ] `vagueACBulletViolations`, `vagueACBulletReason`, `vagueACBulletStripPunctRe`, and `vagueACBulletAllowedVerbs` no longer exist in `internal/items/dor.go`.
- [ ] `DoRCheck` no longer calls the removed function; no other rule names or messages change.
- [ ] All test cases in `internal/items/dor_test.go` that asserted vague-acceptance-bullet behavior get removed; remaining cases continue to pass unchanged.
- [ ] `go test ./... -race` passes.
- [ ] `go vet ./...` passes.
- [ ] BUG-032 accepts cleanly without any rephrasing.

## Notes

If a future need for soft AC-quality nudges resurfaces, the right shape
is an advisory print on `squad accept` (not a blocking violation),
modeled on the existing cadence-nudge pattern in
`cmd/squad/cadence_nudge.go`. That keeps the signal without taxing
authors.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
