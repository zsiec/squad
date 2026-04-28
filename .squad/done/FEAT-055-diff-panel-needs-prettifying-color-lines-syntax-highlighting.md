---
id: FEAT-055
title: diff panel needs prettifying — color +/- lines, syntax highlighting, wider layout, clearer file separators
type: feature
priority: P1
area: web
status: done
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-bbf6
captured_at: 1777344028
accepted_by: agent-bbf6
accepted_at: 1777344058
references: [FEAT-054]
relates-to: []
blocked-by: []
---

## Problem

The worktree-diff panel from FEAT-054 renders a wall of unstyled
monospace text inside a `<pre>` block. It is technically correct but
visually unreadable — the watcher cannot tell at a glance which lines
were added vs removed, where one file ends and the next begins, and
the drawer width clips long lines hard.

## Context

`internal/server/web/agent_detail.js` builds the diff section in
`renderDiffFile`. Today the entire `f.hunks` string is shoved into a
single `<pre class="agent-detail-diff-hunks">`. The CSS in
`internal/server/web/style.css` styles the wrapper but not the lines
inside.

The standard diff convention is straightforward: lines starting with
`+` get a green background, `-` a red one, `@@` a blue accent (hunk
header), `diff --git` and `---`/`+++` are file metadata. Per-line
coloring on the JS side is a 10-line iterator; CSS classes do the
heavy lifting.

Wider drawer is a CSS knob — the existing
`.agent-detail-drawer .action-modal-panel` width can be increased
either generally or specifically when a `data-mode="diff"` is set.

Syntax highlighting *of the code content within hunks* is a meatier
ask. Two paths:
- v1 (this item): just diff-line coloring + structure; no
  language-aware highlighting.
- v2 (follow-up): bring in a highlighter (highlight.js or prism)
  keyed off file extension. Filed separately if the v1 result still
  feels under-cooked.

## Acceptance criteria

- [ ] Each diff line is rendered as its own DOM element with a class derived from its leading character (`+` → `addition`, `-` → `deletion`, `@@` → `hunk-header`, `diff --git` / `---` / `+++` / `index` → `file-header`).
- [ ] Addition lines have a green-tinted background, deletion lines a red-tinted background, hunk headers a blue accent, and file-header lines a dimmed monospace style. Colors must reuse existing theme variables where possible (no new color tokens unless the contrast bar requires them).
- [ ] The drawer is wider by default when the diff section has any files (≥ 720px or 70vw, whichever is smaller); the timeline-only view stays at the current width.
- [ ] Each file block has a visible separator: a stronger top border, a clickable file-path header that toggles collapse, and consistent padding.
- [ ] No new third-party dependencies. Pure CSS + tiny JS line splitter.
- [ ] No backend changes: the response shape from `/api/agents/{id}/diff` is unchanged.
- [ ] Existing FEAT-054 tests continue to pass.

## Notes

Skip language-aware syntax highlighting in this item. If after the v1
prettification the user still wants per-language colors, file a
follow-up feature with a chosen library (highlight.js is small;
prism is smaller; chroma renders server-side in pure Go and would
let us avoid shipping a JS lib at all). Decide then, not now.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
