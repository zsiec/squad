---
id: BUG-042
title: extend workspace mode to remaining read routes (activity, messages, links, search, attestations, stats)
type: bug
priority: P2
area: internal/server
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777332038
accepted_by: agent-401f
accepted_at: 1777332603
references: []
relates-to: []
blocked-by: []
---

## Problem

BUG-040 wired workspace mode (`cfg.RepoID == ""`) into items / agents / inbox / claims, but the remaining read routes still hard-pin `WHERE repo_id = ?` against the empty string or walk `cfg.SquadDir` directly. In workspace mode (launchd-managed daemon, cwd `/`, no repo discovered) every one of these returns empty data even when the user has live state across multiple repos.

Empirically broken in workspace mode today:

- `GET /api/items/{id}/activity` — `messages WHERE repo_id = ?` → empty thread
- `GET /api/messages` — global chat returns []
- `GET /api/items/{id}/links` — `items.Walk(s.cfg.SquadDir)` returns nothing → 404 "no item X"
- `GET /api/search?q=...` — items via `walkAll` work, but `messages WHERE repo_id = ?` returns 0 message hits
- `GET /api/items/{id}/attestations` — `attest.New(db, "")` filters every row out
- `GET /api/stats` — `stats.Compute(..., RepoID: "")` filters every row out
- `GET /api/agents/{id}/events` — `agent_events WHERE repo_id = ?` returns 0
- `GET /api/agents/{id}/timeline` — six `WHERE repo_id = ? AND agent_id = ?` queries (chat / claim / release / commit / attestation / event) all 0
- `GET /api/epics` and `GET /api/epics/{name}` — `epics.Walk(squadDir)` returns nothing
- `GET /api/specs` and `GET /api/specs/{name}` — `specs.Walk(squadDir)` returns nothing

## Context

Pattern set by BUG-040 in `internal/server/items.go`: branch on `cfg.RepoID == ""`. Single-repo mode keeps today's `WHERE repo_id = ?` filter; workspace mode drops the filter and includes `repo_id` in response rows so the SPA can disambiguate. `walkAll()` already does this for items.

Per @agent-bbf6 in the BUG-042 thread: epic and spec slugs are **per-repo unique, not workspace-global**. Two repos can both have a spec named "auth"; aggregating gives a list with potential duplicate names. Detail routes need `?repo_id=` to disambiguate. Items appear to be globally unique only by accident of squad's monotonic numbering — guard against the latent collision now.

Mutation routes (auto-refine, recapture, claim, done, accept, refine, reject, blocked, force-release, items_create, handoff) are deliberately out of scope here — they have a different design question (resolve item's repo from items table vs require client-supplied repo_id) and live in BUG-044.

## Acceptance criteria

- [ ] In workspace mode, `GET /api/items/{id}/activity` returns the message thread for items in any repo; rows include a `repo_id` field
- [ ] In workspace mode, `GET /api/messages` returns chat across all repos; rows include `repo_id`
- [ ] In workspace mode, `GET /api/items/{id}/links` finds the item via `walkAll`-style aggregation, queries the matching repo's `repos.remote_url`, reads that repo's `pending-prs.json`, and lists commits across the right repo
- [ ] In workspace mode, `GET /api/search?q=foo` returns message hits from all repos (item hits already work via `walkAll`); message hits include `repo_id`
- [ ] In workspace mode, `GET /api/items/{id}/attestations` returns attestations for the item from whichever repo it lives in
- [ ] In workspace mode, `GET /api/stats` returns aggregated stats across all repos (or stats per repo with a `repo_id` field — pick whichever the SPA can render without a follow-up item)
- [ ] In workspace mode, `GET /api/agents/{id}/events` and `GET /api/agents/{id}/timeline` return events across all repos for that agent; rows include `repo_id`
- [ ] In workspace mode, `GET /api/epics` and `GET /api/specs` aggregate across all repos and tag each row with `repo_id`
- [ ] `GET /api/epics/{name}` and `GET /api/specs/{name}` accept an optional `?repo_id=` query param; in workspace mode without `?repo_id=` they return the first match deterministically (and include `repo_id` in the response so the SPA can compose the disambiguating link); when two repos collide and `?repo_id=` matches one, return that one
- [ ] Single-repo mode (`cfg.RepoID != ""`) behavior is unchanged — pinned by either preserved existing tests or new single-repo coverage that asserts the old query shape and response shape
- [ ] Each route has a workspace-mode test that seeds a multi-repo fixture in the global DB (mirroring `TestHandleItemsList_WorkspaceModeAggregatesRepos` from BUG-040) and asserts cross-repo aggregation
- [ ] `go test ./...` exit 0 (evidence pasted in close summary)

## Notes

- `attest.New(db, repoID, ...)` and `stats.Compute(..., RepoID, ...)` and `commitlog.ListForItem(db, repoID, id)` currently encode `repoID` as a fixed filter. Two implementation paths:
  1. Extend each package to treat `repoID == ""` as "all repos" (drop the filter). Symmetric with the items.go pattern at the handler layer; one place per package; tests stay close to the data layer.
  2. Keep the packages single-repo and have the handler enumerate repos and merge results.
  Recommend option 1 — fewer moving parts at the handler, and the "" sentinel is already the convention BUG-040 established.
- Stats aggregation has a subtle wrinkle: `claim_duration_seconds` summary buckets are repo-agnostic in the data, so simple aggregation is fine. `wip_violations_attempted_total` is a counter; sum across repos.
- The SPA may need follow-on work to render the new `repo_id` field in chat / activity / agent panels. File a separate follow-up if SPA lacks the affordance.

## Resolution
(Filled in when status → done.)
