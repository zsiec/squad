---
id: CHORE-012
title: replace deprecated goreleaser archives.format with archives.formats list
type: chore
priority: P2
area: release
status: open
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777304100
accepted_by: web
accepted_at: 1777304366
references: []
relates-to: []
blocked-by: []
---

## Problem

`.goreleaser.yaml`'s `archives:` block uses the deprecated single-value `format:` field. Goreleaser v2 emits `DEPRECATED: archives.format should not be used anymore` on every check / snapshot / release run, and the field is being phased out in favor of `formats:` (list).

## Context

The current archives entry sets `format: tar.gz`. The replacement is `formats: [tar.gz]` — a list, so multiple archive formats can be produced from one entry if ever needed. The list-of-one form preserves today's behavior verbatim.

This is the second of three pre-existing goreleaser deprecation cleanups (sibling to CHORE-011 / `snapshot.name_template`). The third was CHORE-013 (`brews → homebrew_casks`) which was rejected — homebrew_casks is macOS-only and would cut Linux users off.

## Acceptance criteria

- [ ] `.goreleaser.yaml`'s `archives[]` entry uses `formats: [tar.gz]` instead of `format: tar.gz`. The `name_template:` and `files:` fields are unchanged.
- [ ] `goreleaser check` no longer emits a deprecation warning naming `archives.format`.
- [ ] `goreleaser release --snapshot --clean --skip=publish` succeeds and produces the four expected `.tar.gz` archives (linux/{amd64,arm64}, darwin/{amd64,arm64}) per the matrix in `CLAUDE.md`.

## Notes

Trivially mechanical — single field rename + value-to-list conversion. No code path or contents change; only the goreleaser schema shape.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
