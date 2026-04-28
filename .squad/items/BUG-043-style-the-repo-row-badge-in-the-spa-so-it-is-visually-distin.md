---
id: BUG-043
title: style the repo row badge in the SPA so it is visually distinct from default chips
type: bug
priority: P2
area: dashboard
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-28"
captured_by: agent-bbf6
captured_at: 1777332039
accepted_by: web
accepted_at: 1777336164
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777336146
auto_refined_by: claude
---
## Problem

The repo-id badge rendered on workspace board rows has no dedicated CSS rule, so it falls through to the default `.row-badge` style (`internal/server/web/style.css:2324`) and is visually indistinguishable from the `epic`, `parallel`, and `evidence` badges next to it on the same row.

## Context

The badge is emitted by `renderRow` in `internal/server/web/board.js:208` as `<span class="row-badge repo">...</span>` whenever `multiRepo(rows)` is true. `style.css` lines 2322–2334 give `epic`, `parallel`, and `evidence` each a distinct color/border, but the `.row-badge.repo` selector is missing — so in a multi-repo workspace view every repo-id chip is the same neutral `--fg-dim` color as a generic chip.

The repo-id is the most context-shifting badge of the four: the others describe the item's metadata, but the repo-id tells you which checkout the row belongs to. It needs to read as a different *kind* of marker, not just another tag.

## Acceptance criteria

- [ ] `internal/server/web/style.css` contains a `.row-badge.repo` rule that sets both `color` and `border-color` to values different from the defaults inherited from `.row-badge`.
- [ ] The chosen `.row-badge.repo` color and border-color are not equal to any of the values used by `.row-badge.epic`, `.row-badge.parallel`, or `.row-badge.evidence`.
- [ ] A Go test in `internal/server` reads `internal/server/web/style.css` and asserts the `.row-badge.repo` selector is present and declares both `color` and `border-color`.
