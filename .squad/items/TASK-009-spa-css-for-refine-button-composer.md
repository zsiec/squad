---
id: TASK-009
title: SPA — CSS for Refine button + composer
type: task
priority: P2
area: web-ui
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-1f3f
captured_at: 1777242008
accepted_by: agent-1f3f
accepted_at: 1777242008
epic: inbox-refinement
references:
  - internal/server/web/style.css
relates-to:
  - TASK-008
blocked-by: []
---

## Problem

The new Refine button and inline composer need styling — warn-tier color (between accept-green and reject-red), composer layout with textarea + right-aligned Send/Cancel.

## Context

Modify `internal/server/web/style.css` near the existing `.inbox-details` block. Add `.action-btn.warn`, `.refine-composer`, `.refine-composer textarea`, `.refine-composer-actions`. If `--warn` CSS variable does not exist in the existing palette, define one (yellow/orange tone — match the project's existing accent palette).

Full reference (with paste-in CSS) in Task 9 of the implementation plan.

## Acceptance criteria

- [ ] `.action-btn.warn` renders with a distinguishable warn color, hover brightens.
- [ ] `.refine-composer` lays the textarea above a right-aligned button row.
- [ ] No regressions in existing inbox-row / detail-panel styling (verify by visual smoke in TASK-010).

## Notes

Pairs with TASK-008 — they can land in the same commit if convenient.

## Resolution

### Fix

`internal/server/web/style.css`:
- Added `.action-btn.warn:hover` — fills with the warn color on hover, matching the existing brightening pattern of `.action-btn.ok`/`.action-btn.danger`. The `.action-btn.warn` base rule already existed.
- Added `.refine-composer` (column flex with gap), `.refine-composer textarea` (multi-line input matching the existing `.compose input` style with focus border in the warn palette), and `.refine-composer-actions` (right-aligned button row).

`--warn` already exists as `#e0a860` in the palette; the warn-button colors (`#d9a650`, border `#8a6f33`) reuse the values from the existing `.action-btn.warn` rule for consistency.

### Visual evidence

Synthesized the markup TASK-008 will emit (Refine button + composer with Cancel/Send) to verify the styles render before TASK-008 lands. Screenshots at:
- `/tmp/browser-test/refine-resting.png` — Refine button distinct from Accept/Reject; composer lays textarea above a right-aligned action row.
- `/tmp/browser-test/refine-hover.png` — Refine button fills with warn color on hover.

### AC verification

- [x] `.action-btn.warn` has a distinguishable warn color (orange) and brightens (fills) on hover.
- [x] `.refine-composer` lays the textarea above a right-aligned button row.
- [x] No regressions in inbox/detail-panel — existing `.action-btn`, `.inbox-details`, etc. unchanged. TASK-010 will run the integration smoke once the wiring is in place.
