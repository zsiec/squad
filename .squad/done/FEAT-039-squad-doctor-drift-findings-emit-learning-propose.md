---
id: FEAT-039
title: squad doctor drift findings emit learning_propose
type: feature
priority: P2
area: internal/hygiene
status: done
estimate: 3h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777308756
accepted_by: web
accepted_at: 1777309558
references: []
relates-to: []
blocked-by: []
parent_spec: agent-team-management-surface
epic: observation-to-knowledge-pipeline
intake_session_id: intake-20260427-44256e4424c4
---

## Problem

`squad doctor` finds the highest-signal patterns in the repo —
schema drift, malformed items, stale claims — and prints them once.
The diagnostic context that future agents need to recognize the same
pattern is exactly what the finding contains, but no learning is
proposed. Each run discards reusable knowledge.

## Context

Doctor sweeps live in `internal/hygiene/`; the CLI shell is
`cmd/squad/doctor.go`. The package enumerates a fixed set of finding
kinds (drift, malformed item, stale claim, etc.) — the work here is
to attach a `learning_propose` template per kind, populated with the
finding's diagnostic body, and emit it as a side effect.

Two guards keep this from becoming noise:

- Debounce per kind per repo per 7 days. The same drift finding on
  every nightly run should not spawn seven proposals.
- `squad doctor --no-learnings` for operators who want the pure
  diagnostic output (CI, scripted audits, etc.).

## Acceptance criteria

- [ ] Each doctor finding kind has a `learning_propose` template; the
      finding's diagnostic context is the body.
- [ ] At most one learning proposed per kind per repo per 7 days
      (debounce, persisted so it survives across `doctor` invocations).
- [ ] `squad doctor --no-learnings` runs the sweep without the
      learning side effect.

## Notes

Enumerate the existing finding kinds in `internal/hygiene` and tag
templates per kind rather than building a generic "doctor finding"
template — the per-kind body is what makes the learning useful to a
future reader.

The debounce key is (repo, kind), not (repo, kind, finding text), so
a recurring class of issue produces one learning, not one per
specific instance.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
