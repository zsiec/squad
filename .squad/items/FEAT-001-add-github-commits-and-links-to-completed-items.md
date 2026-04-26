---
id: FEAT-001
title: Add github commits and links to completed items
type: feature
priority: P0
area: web
status: open
estimate: 3d
risk: medium
created: 2026-04-26
updated: "2026-04-26"
captured_by: web
captured_at: 1777242713
accepted_by: web
accepted_at: 1777242755
references: []
relates-to: []
blocked-by: []
---

## Problem

The drawer view for `done` items in the SPA (`internal/server/web/drawer.js`)
shows AC, body sections, and a state timeline, but no link to the code that
actually landed. A reader looking at a closed item cannot jump to its PR or
commits without leaving the dashboard and grepping git by hand.

## Context

Most of the plumbing already exists:

- **Origin URL** — `internal/repo/discover_register.go:12` runs
  `git config --get remote.origin.url` at register-time and stores it on the
  repo row in `~/.squad/global.db`. A `https://github.com/<owner>/<repo>` base
  URL can be derived server-side without any new git invocation per request.
- **PR ↔ item mapping** — `internal/prmark/` records
  `{item_id, branch, created_at}` in `.squad/pending-prs.json` when
  `squad pr-link` runs (`cmd/squad/pr.go:104`). The merge-archive workflow
  resolves merged PRs back to items via the `<!-- squad-item: ID -->` marker
  embedded in PR bodies.
- **Claim window** — `accepted_at` (frontmatter) and the `done` activity
  event bound the window in which the resolving commits were authored on the
  PR's branch.
- **File touches** — `internal/touch` records which files an item's claim
  touched, used here to disambiguate when several agents commit on the same
  branch in the same window.

Two things are *not* in place: (1) no field on the item or in any table maps
a closed item to a PR URL or commit list, and (2) the SPA drawer has no
"Code" section at all.

URL parsing has to handle both `git@github.com:owner/repo.git` and
`https://github.com/owner/repo(.git)?` forms; non-github origins should
short-circuit to "no links" rather than render broken URLs.

## Acceptance criteria

### Backend

- [ ] New endpoint `GET /api/items/{id}/links` returns
      `{pr: {url, number, branch} | null, commits: [{sha, subject, url}]}`.
- [ ] Origin URL → GitHub base URL helper lives in `internal/prmark/` (next
      to the existing PR-URL parsing) and handles SSH (`git@github.com:o/r.git`),
      HTTPS, and trailing-`.git` forms; non-GitHub origins return empty.
- [ ] PR resolution: when `pending-prs.json` has an entry for the item, the
      response includes a constructed
      `https://github.com/<o>/<r>/pull/<n>` URL. PR number is *not* extracted
      from `pending-prs.json` (it doesn't carry one) — derive it by parsing
      the PR URL written by `squad pr-link` if present, otherwise return
      `pr: { url: null, branch }` so the SPA can still link the branch
      compare view.
- [ ] Commit resolution: shells out to
      `git log <branch>..<base> --format=<sha,subject>` (or the symmetric
      form scoped to the claim window via `--since`/`--until`), filtered to
      the item's touched files, capped at 20 commits.
- [ ] git log invocation is sandboxed: refuse if branch name contains shell
      metacharacters; pass arguments via `exec.Command` arg slice, never via
      a shell string.
- [ ] Endpoint returns 404 for unknown item IDs, 200 with
      `{pr: null, commits: []}` for known-but-unlinked items.
- [ ] Server test against a fixture git repo with a deterministic commit
      history asserts the endpoint returns the expected URLs and shas given
      a fixed `accepted_at`/`done_ts` and a known `pending-prs.json`.
- [ ] Server test for the URL helper covers SSH, HTTPS, `.git` suffix,
      non-github (e.g. gitlab), and malformed-origin inputs.

### SPA

- [ ] `drawer.js` renders a new "Code" section, after the state timeline and
      before the "Detail" section, when `it.status === 'done'`.
- [ ] Section shows PR row first (if present): `PR #<n> <branch>` linking to
      the PR URL, opens in a new tab (`target="_blank" rel="noopener"`).
- [ ] Each commit row shows `<short-sha> <subject>` linking to
      `<base>/commit/<sha>`. Subject is HTML-escaped.
- [ ] Empty state: if both `pr` is null and `commits` is empty, the entire
      section is omitted (no empty card, no "no commits found" placeholder).
- [ ] Section is fetched lazily when the drawer opens for a done item, not
      eagerly for every list response. Loading state is a single skeleton
      row, not a spinner.
- [ ] Manual-test note in the resolution: opens for a real done item with a
      PR, opens for a done item without a PR, opens for a non-github repo.

## Notes

- **Why a server endpoint instead of denormalising onto the item file** — PR
  and commit data lives outside `.squad/items/`, and the item file is
  already the contract. Stuffing PR URLs into the frontmatter on `squad
  done` would couple the item format to GitHub and create a drift surface
  if the URL ever changes.
- **Author filter is unreliable** — multiple agents commit under one human's
  `git config user.email`, so the time-window + touched-files intersection
  is the disambiguator. Don't filter by author.
- **No commit-message ID lookup** — squad explicitly forbids item IDs in
  commit messages (CLAUDE.md, "No PM traces anywhere in code"), so we
  can't grep messages.
- **Estimate** — 1h was the captured estimate; a realistic split is ~1d
  backend (endpoint + URL helper + tests) + ~1d SPA (section, lazy fetch,
  empty/loading states) + ~1d for fixture-repo plumbing and edge cases
  (non-github origin, missing PR, branch deleted post-merge). Bumped to 3d.
- **Out of scope** — querying the GitHub API for merge state, author, or
  CI status. Branch + PR URL is enough; anything richer becomes a separate
  feature.

## Resolution
(Filled in when status → done.)
