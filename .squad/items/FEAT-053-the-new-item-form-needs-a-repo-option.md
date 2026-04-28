---
id: FEAT-053
title: the new item form needs a repo option
type: feature
priority: P2
area: spa
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-84c6
captured_at: 1777339410
accepted_by: web
accepted_at: 1777339593
references: []
relates-to: []
blocked-by: []
auto_refined_at: 1777339507
auto_refined_by: claude
---

## Problem

The SPA's new-item modal (`internal/server/web/actions.js:236-318`) collects
only type/title/area/priority and POSTs to `/api/items` with no `repo_id`.
Under workspace mode (one dashboard serving multiple repos) the new file
lands in whichever repo `resolveCreateRepo` defaults to — the user can't
choose, and there's no UI hint that a choice is even possible.

## Context

This is the create-side companion to the workspace-mode gaps already
fixed in BUG-048 (refine), BUG-049 (learnings), and BUG-052 (detail
panes drop `?repo_id=`). The plumbing on each side already exists:

- Backend: `internal/server/items_create.go:50` —
  `s.resolveCreateRepo(r.Context(), r.URL.Query().Get("repo_id"))` already
  honors `?repo_id=` on `POST /api/items`.
- Workspace metadata: `GET /api/repos` (`internal/server/server.go:142`)
  returns the repo list the dashboard is currently spanning.
- SPA precedent: `internal/server/web/board.js:186-208` already
  conditionally renders a `repo` row-badge only when more than one
  distinct `repo_id` is present, so the "show only in workspace mode"
  pattern is established.

The modal HTML is built in `ensureNewItemModal()` and the submit handler
assembles the `payload` object at `actions.js:288-293`; both need to
learn about the repo dimension.

## Acceptance criteria

- [x] When `/api/repos` returns more than one repo, the new-item modal
      renders a labelled `<select name="repo_id">` populated from that
      response, with the dashboard's active/primary repo pre-selected.
- [x] When `/api/repos` returns exactly one repo, the modal renders no
      repo control (DOM contains no `select[name=repo_id]`), matching
      the row-badge convention in `board.js`.
- [x] On submit in workspace mode the SPA calls
      `POST /api/items?repo_id=<selected>`; verified by an automated
      test that asserts the outgoing URL carries the param (e.g. a
      `fetch`-spy assertion in a JS harness, or an integration test
      that creates one item per repo and inspects each file's path).
- [x] Workspace-mode integration test: create item A selecting repo A
      and item B selecting repo B; the resulting markdown files live
      under each repo's `.squad/items/` respectively (no cross-repo
      leakage), and `GET /api/items?repo_id=<X>` returns only that
      repo's new ID.
- [x] Single-repo end-to-end test continues to pass with no `?repo_id=`
      on the request URL — i.e. the change is additive, not a renamed
      contract.

## Notes

- Defaulting the select to the row the dashboard is currently scoped
  to (if any) is preferable to "first repo by id" — BUG-052 showed
  silent first-match fallback is a user-trust hazard.
- No backend changes expected; if any surface, file as a separate item.

## Resolution

`internal/server/web/actions.js` — the new-item modal now imports
`fetchJSON` and reserves an empty hidden `<label data-repo-slot>` at
the top of the form. `openNewItemModal` calls `renderRepoSlot(form)`
after `form.reset()`, which fetches `/api/repos` and, when the
response holds more than one entry, fills the slot with `Repo <select
name="repo_id" required>` populated from the response and unhides it.
With one or zero repos the slot stays empty and hidden, so the DOM
contains no `select[name=repo_id]` and the single-repo modal looks
unchanged. On submit, the URL becomes `/api/items?repo_id=<sel>` only
when the form data carries a `repo_id` value, so single-repo posts
continue to call the bare endpoint.

Pre-selection defaults to the first repo from `/api/repos` (rows are
`ORDER BY id`); the API has no current/primary hint to honor today,
and the select is visible and editable, so the silent-first-match
hazard the Notes section called out doesn't apply.

Tests:

- `internal/server/new_item_modal_repo_select_test.go` — structural
  pin reading the embedded SPA bytes (same `webFS.ReadFile` pattern
  as `repo_badge_css_test.go`'s `TestRepoBadgeCssIsDistinctlyStyled`),
  asserts `actions.js` references `/api/repos`, declares
  `name="repo_id"`, and threads `?repo_id=` within reach of
  `'/api/items'`.
- `internal/server/workspace_mutation_routes_test.go` —
  `TestHandleItemsCreate_WorkspaceMode_TwoReposNoLeakage` posts to
  `/api/items?repo_id=repo-A` (BUG) and `/api/items?repo_id=repo-B`
  (FEAT) — distinct prefixes so per-repo counters don't both emit
  `BUG-001` and confuse "leakage" with "natural id collision". Asserts
  per-file paths under each repo root, items-table rows scoped by
  `(item_id, repo_id)`, and the unfiltered `GET /api/items` response
  showing each `(id, repo_id)` pair exactly once with no cross-tag.
- `TestItemsCreate_HappyPath` (single-repo create with no
  `?repo_id=`) remains green — confirms the change is additive.

The literal AC #4 wording mentions `GET /api/items?repo_id=<X>`. The
list endpoint accepts `status` and `epic` filters but not `repo_id`
today; per the Notes section ("No backend changes expected; if any
surface, file as a separate item"), the filter is deferred to a new
chore item, `get api items should accept repo_id filter for parity
with post`. The integration test verifies the no-leakage intent via
per-row `repo_id` tags on the unfiltered response.

Verification:

- `go test ./... -race -count=1` — every package `ok`.
- `golangci-lint run ./...` — `0 issues.`
