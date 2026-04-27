---
id: BUG-040
title: squad serve daemon should default to workspace mode (enumerate repos via global DB) instead of relative --squad-dir
type: bug
priority: P2
area: cmd/squad
status: open
estimate: 4h
risk: medium
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777330911
accepted_by: agent-bbf6
accepted_at: 1777330950
references: []
relates-to: []
blocked-by: []
---

## Problem

`squad serve` defaults `--squad-dir` to the relative path `.squad`
(`cmd/squad/serve.go:62`). For a daemon spawned by launchd / systemd-user
this default is meaningless — the daemon's cwd is launchd's default
(`/` on darwin), so `.squad/items/` resolves to a directory that does
not exist. Result: after a reboot, `launchctl kickstart`, or any non-
repo-cwd start of the daemon, the dashboard shows zero items / zero
agents / zero done across the board, even when the user has live
state. The installer at
`internal/tui/daemon/install_darwin.go` and the `--reinstall-service`
path bake the same flagless invocation into the plist, so a reinstall
does not fix it. Reproduced today on darwin: `launchctl list
sh.squad.serve` shows `LastExitStatus = 0` (after the unrelated
`--token` flag drift was fixed) and PID running, but the SPA at
`http://127.0.0.1:7777/` shows 0 active / 0 in-flight / 0 open / 0
done.

## Context

The codebase has signals that the daemon was intended to be
workspace-aware, not single-repo:

- `~/.squad/global.db` already holds cross-repo state (`repos`,
  `claims`, `messages`, `agents` tables).
- `internal/workspace/` exists with package doc "multi-repo views".
- `internal/server/server.go` registers
  `GET /api/workspace/status`.
- The installed launchd plist deliberately omits `--squad-dir`,
  implying the daemon should not be tied to one repo.

The mismatch is in the items / inbox / status routes that still read
`cfg.SquadDir` (default `.squad`) via `items.Walk(squadDir)` and
related callers. Those routes need to walk every repo registered in
the global DB and aggregate, with a per-repo selector exposed to the
SPA.

`cmd/squad/serve.go --squad-dir` should remain a single-repo override
for foreground use ("I want to see one repo's view") but the daemon
default should be workspace.

## Acceptance criteria

- [ ] When `squad serve` runs without `--squad-dir`, it operates in workspace mode — enumerates every repo from `~/.squad/global.db`'s `repos` table and walks each repo's `.squad/items/` and `.squad/done/`.
- [ ] `/api/items`, `/api/inbox`, `/api/agents`, `/api/status`, and any other repo-scoped routes return aggregated data across all enumerated repos. The response payload carries the repo identifier per item so the SPA can display it.
- [ ] When `--squad-dir <path>` is passed, the existing single-repo behavior is preserved exactly — used by foreground `squad serve` runs from inside a repo and by tests.
- [ ] The launchd / systemd installer continues to omit `--squad-dir` (relies on the new workspace default).
- [ ] The SPA surfaces a repo selector or per-row repo column so the user can tell which repo an item is from. Items from the current page's "active repo" sort first; cross-repo entries are visually distinguishable.
- [ ] On a fresh checkout with two repos registered in the global DB, `curl http://127.0.0.1:7777/api/items` returns items from both, tagged with their repo. New regression test in `internal/server/` covers this with a fixture global DB seeded with two repos.
- [ ] After this lands, `launchctl kickstart -k gui/$UID/sh.squad.serve` followed by visiting the dashboard surfaces the correct counters without any `--squad-dir` flag in the plist.

## Notes

The current single-repo behavior survives because most users run
`squad serve` from within their one repo, where the relative `.squad`
path happens to resolve correctly. The bug surfaces precisely when
the user adopts the daemon model — `squad tui` auto-installs the
service, then any later restart loses the cwd context.

Related: BUG-031 (welcome flow re-opens) and BUG-022 / BUG-017 — all
share the symptom that the daemon's environment is not the
foreground user's environment. This bug is the workspace-shaped
version of that pattern.

Open design question: how should the SPA pick the "default" repo
view when launched? Options: (a) most-recently-touched repo (use
`agents.last_tick_at` per repo), (b) the repo whose path matches
`$PWD` of the user when they opened the URL — but the daemon does
not know that, (c) all repos at once with explicit per-row repo
column. Pick during implementation; (a) is the cheapest first cut.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
