---
id: FEAT-058
title: risk-tiered code review requires two reviewers for p0 and high risk items
type: feature
priority: P2
area: review
status: open
estimate: 4h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777345159
accepted_by: web
accepted_at: 1777345479
references: []
relates-to: []
blocked-by: []
epic: team-practices-as-mechanical-visibility
---

## Problem

Every item — a one-line typo fix or a P0 incident response —
currently goes through the same single code-quality review pass
(`superpowers:code-reviewer` per `squad-code-review-mandatory`).
High-stakes work deserves a second, adversarial pass framed as
"what fails in production" — but it doesn't get one because there
is no tier in the review contract.

## Context

The pre-`squad done` verification gate runs the configured
verification commands plus calls into the code-reviewer subagent.
That dispatch lives near the top of `cmd/squad/done.go` and the
attestation/skill plumbing is already in place
(`internal/attest/`, `plugin/skills/squad-code-review-mandatory/`).
Adding a second reviewer for a tagged subset of items is additive,
not a contract break.

The "two pairs of eyes on incidents" practice in human teams runs
on social pressure: the on-call wants their fix witnessed.
Translate to agents by gating `squad done` instead — if the item
is `priority: P0` or `risk: high`, require two distinct reviewer
attestations (different reviewer roles, both passing) before
close.

## Acceptance criteria

- [ ] An item with `priority: P0` cannot reach `done` until two
      reviewer attestations exist on the item and both report
      no blocking findings.
- [ ] Same for `risk: high`. The gate fires regardless of priority.
- [ ] The two reviewers are distinct roles — one
      `superpowers:code-reviewer` (existing code-quality pass) and
      one new "production-failure" pass (a separate prompt
      adversarially asking what fails under load, edge cases, or
      partial failure modes). Both attestations are recorded under
      the existing attestation ledger.
- [ ] Lower-tier items (`priority: P1`/`P2`/`P3` AND `risk:
      low`/`medium`) keep today's single-reviewer behavior — no
      new friction.
- [ ] `squad done` error message on missing second attestation
      tells the caller exactly what's missing
      (`run 'squad attest <ID> --kind review --command "..."'`
      or equivalent).
- [ ] Test coverage: a P0 item without two reviews fails the gate;
      with both, passes; a P2 item passes with one. End-to-end
      shape; mocked subagent runners.

## Notes

- The "production-failure" reviewer is a prompt change, not a new
  agent type. Reuse the subagent dispatch mechanism — only the
  prompt and the recorded `kind` differ.
- Risk: doubling review on hot incidents can slow them down.
  Mitigation: the two reviews can run in parallel via the existing
  `dispatching-parallel-agents` pattern; total wall-clock cost is
  ~the same as one review since they share no state.
- Lands cleanly after FEAT-057. Standup surfaces who's holding the
  P0; the gate ensures it can't ship without the second look.

## Resolution
(Filled in when status → done.)
