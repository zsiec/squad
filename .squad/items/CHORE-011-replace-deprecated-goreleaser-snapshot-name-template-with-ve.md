---
id: CHORE-011
title: replace deprecated goreleaser snapshot.name_template with version_template
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
captured_at: 1777304098
accepted_by: web
accepted_at: 1777304366
references: []
relates-to: []
blocked-by: []
---

## Problem
`.goreleaser.yaml:32` uses `snapshot.name_template`, which goreleaser v2 deprecated in favour of `snapshot.version_template`. The release pipeline still works today but emits a deprecation warning every snapshot build, and a future major bump will remove the old key entirely.

## Context
The deprecated key is the `name_template:` line nested under the `snapshot:` block (`.goreleaser.yaml:31-32`). Other `name_template:` keys in the same file are NOT affected — `archives[].name_template` (`:22`) and `checksum.name_template` (`:29`) are still the supported keys for those blocks. Only `snapshot.name_template` → `snapshot.version_template` was renamed in goreleaser v2.

The current value `"{{ incpatch .Version }}-snapshot"` is a `version_template`-shaped expression already, so the fix is a one-line key rename with no template-content change.

## Acceptance criteria
- [ ] `.goreleaser.yaml` uses `snapshot.version_template` instead of `snapshot.name_template`. The template value is preserved verbatim.
- [ ] `archives[].name_template` and `checksum.name_template` are unchanged — only the `snapshot:` block's key is renamed.
- [ ] `goreleaser check` (or the equivalent v2 validation command) succeeds and emits no deprecation warning naming `snapshot.name_template`.
- [ ] `goreleaser build --snapshot --clean` (local snapshot build) succeeds and produces the four expected archives (linux/{amd64,arm64}, darwin/{amd64,arm64}) per the matrix in `CLAUDE.md`.

## Notes
- 30m estimate stands; this is one line in `.goreleaser.yaml` plus a local goreleaser invocation to verify.
- Coordinate with the in-flight FEAT-028 (Homebrew tap) work, which also edits `.goreleaser.yaml` — the two changes are in different blocks (`snapshot:` vs new `brews:`) but should land in commit order to avoid a noisy merge.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
