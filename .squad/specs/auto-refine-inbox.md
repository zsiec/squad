---
title: Auto-refine inbox items via the local claude CLI
motivation: |
  Today the dashboard inbox shows each captured item with a manual-comments Refine
  button (`internal/server/web/inbox.js:114`) — useful when a reviewer has specific
  notes, but the wrong tool for the dominant case: an item captured with just a
  title (or a body still bearing the squad-new placeholders) that just needs a
  real Problem / Context / Acceptance criteria block before it can be accepted.
  With the new `template-not-placeholder` DoR rule blocking accept on placeholder
  AC, this papercut sharpened: the reviewer cannot click Accept until somebody
  writes real AC. Auto-refine pushes the drafting onto the user's local claude
  CLI, leaving the human in an approve/reject role.
acceptance:
  - The inbox per-item action row exposes an "Auto-refine" button on every captured item; clicking it triggers a server-side subprocess that drafts a fresh body and atomically rewrites the item file.
  - The drafted body satisfies items.DoRCheck (including template-not-placeholder); the human Accept click remains the only path from captured → open.
  - Auto-refined items carry frontmatter audit fields (`auto_refined_at` unix, `auto_refined_by: claude`); the inbox renders an "auto-refined" badge so reviewers know they are approving a draft.
  - A reviewer who dislikes the draft can re-click Auto-refine to redraft (overwrites the previous body, no confirmation modal); the existing Reject flow also still works.
  - The existing manual-comments Refine flow (`POST /api/items/{id}/refine`, `openRefineComposer`) keeps working from the item-detail panel; auto-refine is additive on the inbox row, not a replacement of the entire refine epic.
  - Subprocess failure (CLI not on PATH, auth not set, timeout, model error) leaves the item body unchanged and surfaces a toast.
non_goals:
  - Server-side Anthropic SDK integration. We shell out to the user's local `claude` CLI; squad's go.mod stays free of the Anthropic SDK and `ANTHROPIC_API_KEY` is never read by squad serve.
  - Auto-promoting refined items to `open`. Acceptance stays a human click.
  - Streaming progress / partial draft rendering. The button is a sync 90s-bounded request; UI shows a spinner until done.
  - Persisting the prompt/response transcript. We record only that auto-refine happened and when; the body itself is the artifact.
  - Per-AC inline edits or diff views. The drafted body replaces the existing body wholesale.
  - Cross-host coordination. Squad serve is loopback / single-user; we do not handle two browsers double-clicking on different machines.
  - Decomposition into sub-items by claude. The CLI emits one item body, not a tree.
integration:
  - internal/items/ — new AutoRefineApply primitive that atomically rewrites a body and stamps `auto_refined_at`/`auto_refined_by`; new fields on Item; persistence + parse round-trip.
  - internal/store/ — additive columns / migration if the items row carries auto_refined_at server-side (decide at impl time; can also live solely in the markdown frontmatter).
  - internal/mcp/ — new tool `squad_auto_refine_apply(item_id, new_body)` that calls the items primitive; intentionally not part of the default toolset surfaced to interactive sessions.
  - internal/server/ — new route `POST /api/items/{id}/auto-refine`; per-item in-flight set; subprocess invocation via `os/exec` with 90s ctx deadline + process-group kill on timeout; narrow MCP config materialized to a temp file per call (or stable per-server).
  - internal/server/web/ — `inbox.js` button replacement, drafting spinner, error toast, "auto-refined" badge in the inbox row; `inbox.css` for the badge.
  - internal/server/inbox.go — surface `auto_refined_at` / `auto_refined_by` in the inbox JSON so the SPA can render the badge.
  - cmd/squad/ — no new top-level CLI verb in v1; the auto-refine path is dashboard-only.
  - docs/recipes/ — short note on operator requirements (claude CLI on PATH, prior `claude /login`).
---

## Background

The dashboard inbox modal in `internal/server/web/inbox.js` is the human triage
surface for `status: captured` items. The previous `inbox-refinement` epic added
a third action ("Send for refinement") that captures reviewer comments and
flips an item to `needs-refinement`; that flow is still the right tool when a
reviewer has *specific notes* for a peer agent. Auto-refine targets the
opposite case: the reviewer has *no specific notes*, just wants the body
fleshed out enough to accept.

The mechanism is a server-side subprocess invocation of the user's local
`claude` CLI in print mode (`claude -p`), with a tightly-scoped MCP config
that exposes only read tools plus one new write tool
(`squad_auto_refine_apply`). The CLI session reads the item via MCP, drafts a
body, and calls `squad_auto_refine_apply` exactly once to persist. The squad
server enforces:

- A 90-second deadline on the subprocess (`exec.CommandContext` + process-group kill).
- Per-item in-flight dedupe (a second click while drafting returns 409 Conflict).
- Verification that `auto_refined_at` advanced after the subprocess exits, otherwise it returns 500 with "claude did not produce a draft."
- Atomic body rewrite — the file is never half-written; the `auto_refine_apply` tool either fully succeeds or leaves the file untouched.

The trust boundary: the CLI subprocess can only mutate items via
`squad_auto_refine_apply`. The squad server runs the MCP server in-process and
exports only the narrow surface to the spawned CLI; the broader squad
toolset is not available, so the model cannot accidentally claim items, post
chat, or accept items.

The full architectural design (sequence diagram, error matrix, prompt shape)
is captured in
`/Users/zsiec/dev/switchframe/docs/plans/2026-04-27-squad-auto-refine-inbox-design.md`
and the implementation phasing in
`/Users/zsiec/dev/switchframe/docs/plans/2026-04-27-squad-auto-refine-inbox.md`
(both gitignored, per project convention; not all design docs exist yet — the
implementer creates them at refinement time if the squad-loop calls for it).
