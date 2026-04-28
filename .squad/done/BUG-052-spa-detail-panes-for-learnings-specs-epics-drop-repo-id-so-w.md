---
id: BUG-052
title: SPA detail panes for learnings/specs/epics drop ?repo_id= so workspace-mode collisions silently load the wrong repo's row
type: bug
priority: P3
area: internal/server/web
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777337428
accepted_by: web
accepted_at: 1777338068
references: []
relates-to: []
blocked-by: []
---

## Problem

The dashboard SPA's detail panes for learnings, specs, and epics call
`GET /api/<resource>/<slug-or-name>` with no `?repo_id=` query param.
When the same slug/name exists in two repos under workspace mode, the
backend handler returns the FIRST match (deterministic by `repos.id`
sort order). The SPA silently displays the wrong repo's row — there's
no badge, no picker, no error.

The list-side rows already include `repo_id`, so the SPA *has* the data
to thread it through; it just doesn't.

## Context

Backend handlers already accept `?repo_id=` per BUG-042 (specs/epics)
and the just-landed companion for learnings:

- `internal/server/specs.go:75-114` — `handleSpecDetail` reads `?repo_id=`
- `internal/server/epics.go:84-105` — `handleEpicDetail` reads `?repo_id=`
- `internal/server/learnings.go` — `handleLearningDetail` reads `?repo_id=`

SPA call sites that drop the param:

- `internal/server/web/learnings.js:132` — `fetchJSON('/api/learnings/' + encodeURIComponent(slug))`
- (find equivalent for specs / epics — likely same shape in `specs.js` / `epics.js`)

Repro under a workspace-mode dashboard with two repos that share a
slug ("auth", say): click the slug in the list — the detail pane
loads whichever repo `repos.id` sorted to first, silently. The list
row has `repo_id: "repo-B"` but the detail pane shows `repo-A`'s body.

## Acceptance criteria

- [ ] `internal/server/web/learnings.js` thread the row's `repo_id`
      into the detail-fetch URL when present (`?repo_id=<row.repo_id>`).
- [ ] Same for `specs.js` and `epics.js` detail-load paths.
- [ ] An end-to-end test (or at minimum a fetched-URL assertion via
      the JS harness) pins that the detail request carries `?repo_id=`
      when the row originated from a multi-repo response.
- [ ] When the same slug exists in two repos, clicking each list row
      opens the correct repo's detail without silent fallback.

## Notes

- Companion to BUG-042 (read-route workspace mode) and the just-landed
  learnings workspace-mode wiring. Backend is correct; this is purely
  an SPA wiring gap surfaced at code-review time.
- Spec/epic detail panes have the same bug class — flagged together
  here since the fix shape is identical and they share the same SPA
  conventions. Splitting into per-resource items would just multiply
  the small follow-up.
- No hooks needed on the list-side; rows already carry `repo_id` from
  the backend.

## Resolution
(Filled in when status → done.)
