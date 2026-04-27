---
id: FEAT-032
title: inbox JSON surfaces auto_refined_at / auto_refined_by for the SPA badge
type: feature
priority: P2
area: server
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777308128
accepted_by: web
accepted_at: 1777308350
epic: auto-refine-inbox
references:
  - internal/server/inbox.go
relates-to: []
blocked-by: [TASK-040]
---

## Problem

The inbox API response shape in `internal/server/inbox.go` does not include the new `auto_refined_at` / `auto_refined_by` fields. Without them the SPA cannot render the auto-refined badge on inbox rows — the badge fires off the presence of `auto_refined_at`, which the SPA only sees if the server includes it.

## Context

`internal/server/inbox.go` builds per-item entries from `items.Parse` plus the DoR pass result. It already exposes a fixed JSON shape consumed by `inbox.js`. We extend the entry struct with `AutoRefinedAt int64` and `AutoRefinedBy string` (omitempty) sourced from the parsed item's new fields (TASK-040). No DB read required if the items primitive keeps the audit fields file-only (recommended); if the impl puts them in the DB mirror the read path is also cheap.

## Acceptance criteria

- [ ] Inbox API entry struct in `internal/server/inbox.go` gains `AutoRefinedAt int64 ` + `AutoRefinedBy string` JSON fields with `omitempty` so non-auto-refined items keep a slim payload.
- [ ] The fields are populated from the parsed item (`it.AutoRefinedAt`, `it.AutoRefinedBy`); zero values yield omitted JSON fields.
- [ ] The inbox API handler test covers an entry with both fields set (auto-refined item) and an entry with neither set (untouched captured item) and asserts the JSON shape matches.
- [ ] No regression in existing inbox response fields (`item_id`, `title`, `priority`, `area`, `dor_pass`, etc.); the JSON additions are strictly additive.
- [ ] The existing `inbox_changed` SSE payload is not affected — clients re-fetch the inbox after an event and pick up the new fields naturally.

## Notes

Tiny item by design — extracted from FEAT-031 so the SPA work and the server JSON shape can be developed and reviewed independently. They merge in either order; FEAT-031 references this work as a blocker because the badge cannot render without the data.
