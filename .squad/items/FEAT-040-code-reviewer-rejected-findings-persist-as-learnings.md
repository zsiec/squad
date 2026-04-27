---
id: FEAT-040
title: code-reviewer rejected findings persist as learnings
type: feature
priority: P2
area: internal/learning
status: open
estimate: 2h
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

When `superpowers:code-reviewer` rejects a change, the rejection
rationale is the highest-signal observation in the system: a peer
agent explaining, with concrete reasoning against concrete code, why
something was wrong. Today that rationale lives in the attestation
ledger but is not mined into a learning. The next agent makes the
same mistake.

## Context

Attestations are written via `internal/attest/` (CLI:
`cmd/squad/attest.go`). The learning artifacts package is
`internal/learning/`. Review attestations exist today but their text
content is opaque to the learning pipeline.

The conversion is straightforward: when an attestation of kind
`review` exits non-zero, capture the rejection text and emit a
`learning_propose` with `kind=gotcha`, body = the rejection rationale.
Repeat rejections on the same item are themselves a signal — tag the
second and subsequent learnings as `second-round` so a reviewer
triaging the proposal queue can prioritize patterns that are biting
twice.

## Acceptance criteria

- [ ] `squad attest --kind review` with a non-zero exit captures the
      rejection text into the attestation row.
- [ ] That rejection text becomes a `learning_propose` with
      `kind=gotcha` and the rationale as the body.
- [ ] A subsequent `squad attest --kind review` non-zero on the same
      item tags the new learning as `second-round`.

## Notes

The "rejection text" is whatever the reviewer agent emits to stdout
or the attestation note field — pick whichever the attest CLI
already plumbs through. Do not invent a new transport.

`second-round` is a tag on the learning, not a new kind. Triage
queries can filter on it without schema changes.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
