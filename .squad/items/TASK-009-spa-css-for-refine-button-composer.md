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
