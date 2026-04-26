---
id: FEAT-002
title: explicit commit recording at squad done — drop git-log inference
type: feature
priority: P1
area: server
status: done
estimate: 2h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777244479
accepted_by: agent-bbf6
accepted_at: 1777244479
references: []
relates-to: []
blocked-by: []
---

## Problem

FEAT-001 resolved an item's commits by shelling out to `git log` with a window + touched-files filter. That's heuristic — it requires a `pending-prs.json` entry to know the branch, and even then it can include commits the agent didn't make (overlap with peers), or miss commits the agent did make (touches not recorded). The feedback was: don't infer; record explicitly.

## Context

The agent that closes an item knows when it claimed the work and is about to release the claim. At that moment, every commit on `HEAD` since `claimed_at` is part of the agent's contribution. Capture that list at `squad done` time into a new `commits` table; let the `/api/items/{id}/links` endpoint just `SELECT` from it.

## Acceptance criteria

- [ ] New `commits` table: `(repo_id, item_id, sha, subject, ts, agent_id)` with primary key on `(repo_id, sha)` so re-running done is idempotent.
- [ ] `squad done` (and the equivalent MCP/server paths) enumerates `git log <claimed_at..HEAD>` for every commit at-or-after the claim's `claimed_at` and inserts each into `commits`.
- [ ] `handleItemLinks` reads from the `commits` table — no more git-log shelling at request time. Each row gets decorated with `<github-base>/commit/<sha>`.
- [ ] PR resolution path (pending-prs.json → branch → compare URL) stays as-is.
- [ ] `internal/prmark/commits.go` (`ResolveCommits`) and its tests are deleted.
- [ ] Server tests updated: seed the `commits` table directly, drop the fixture-git-repo dependency from the links test.
- [ ] New unit test for the done-side capture: a fixture git repo with 3 commits before+after `claimed_at` returns only the in-window commits as inserted rows.

## Notes

Trade-offs accepted: the time-window approach can include peer commits if multiple agents concurrently commit on the same working-tree branch (rare); it cannot distinguish co-author cases. A future per-commit hook (post-commit recording) would be more precise but is out of scope here.

## Resolution
(Filled in when status → done.)
