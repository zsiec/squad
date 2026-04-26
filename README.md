# squad

[![CI](https://github.com/zsiec/squad/actions/workflows/ci.yml/badge.svg)](https://github.com/zsiec/squad/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Project-management framework for software work done with AI coding agents.

Atomic claims, typed chat verbs, file-touch tracking, hygiene sweeps, web dashboard, and an optional Claude Code plugin — shipped as a single static binary that works for solo and multi-agent setups.

> ⚠️ **Status:** under active development, pre-1.0.

## Install

The fastest path is via Claude Code's plugin system:

```bash
claude install github.com/zsiec/squad
```

This drops the manifest, registers the MCP server, wires the always-on hooks, and exposes squad's verbs as MCP tools. No further setup needed.

If you prefer to install the binary first (e.g. for CLI-only use, scripting, or CI):

```bash
go install github.com/zsiec/squad/cmd/squad@latest
squad install-plugin   # registers MCP + hooks in ~/.claude/settings.json
# brew install zsiec/tap/squad  # planned, post-v1.0.0
```

Both paths converge on the same final state.

## Agent quickstart

One command takes a fresh Claude Code session from zero to working:

```bash
squad go
```

This is idempotent. On first run it inits `.squad/`, registers a session-derived agent id, claims the top ready item, prints its acceptance criteria, and flushes any unread chat into your context. On re-run it resumes the same claim and re-flushes the mailbox.

If you're using the Claude Code plugin, the `/work` slash command does the same thing.

```bash
# What just happened?
squad whoami       # who am I in this repo?
squad next         # what else is ready?
squad tick         # any new chat since I last looked?
```

When you finish: `squad done <ID> --summary "one-line outcome"`.

## Human quickstart

For developers driving squad by hand (no agent, exploring the loop):

```bash
# 1. Initialize a repo
cd ~/dev/your-project
squad init                        # answers ≤3 questions

# 2. Register and pick up your first item
squad register                    # auto-derives a session-stable id
squad next                        # see what's ready
squad new feat "your first item"
squad claim FEAT-001 --intent "first squad item"

# 3. Do the work, then close
# ... edit, test, commit ...
squad done FEAT-001 --summary "shipped"
```

Total time: under five minutes from `go install` to first `done`.

## What you get

- **Atomic claims** backed by SQLite `BEGIN IMMEDIATE` — two agents racing for one item, exactly one wins.
- **Typed chat verbs** (`thinking`, `milestone`, `stuck`, `fyi`, `ask @agent`) routing automatically to the right thread.
- **File-touch tracking** so peers see your overlap before they edit the same file.
- **Hygiene sweep** (`squad doctor`) for stale claims, orphan touches, broken refs, DB integrity.
- **Web dashboard** (`squad serve`) with SSE — live who-has-what across every repo on your machine.
- **Optional Claude Code plugin** with skills, slash commands, hook scripts (default-on + opt-in), and an MCP server (`squad mcp`) exposing the full verb surface — claim, done, attest, learning_propose, etc. — so agents call squad without spawning a shell.
- **Multi-repo workspace** queries — one ready stack, one chat history across all your projects.
- **GitHub Actions auto-archive** — merge a PR with a hidden item marker, the workflow moves it to `.squad/done/` automatically.

## Cross-repo views

Once `squad init` has run in two or more repos, the global DB knows about all of them. From any repo:

```bash
squad workspace status            # per-repo summary table
squad workspace next --limit 10   # top P0/P1 across every repo
squad workspace who               # every agent in every repo, last activity
squad workspace list              # all known repos
```

## Stats

`squad stats --json` prints a Snapshot with verification rate, claim p50/p90/p99,
WIP-cap violations, reviewer disagreement, and (when available) repeat-mistake
rate. `squad stats --tail` streams NDJSON for external aggregation.

The dashboard exposes the same Snapshot at `GET /api/stats` and a Prometheus
text exposition at `GET /metrics`. The Insights panel (S key, or the **STATS**
button) renders verification rate over time, claim p99 over time, and WIP-cap
violations.

## Documentation

- [docs/README.md](docs/README.md) — top-level entry point
- [docs/adopting.md](docs/adopting.md) — full onboarding walkthrough
- [docs/concepts/](docs/concepts/) — the loop, claims, chat, hygiene, multi-repo
- [docs/reference/](docs/reference/) — commands, config, hooks, skills, slash commands, db schema
- [docs/recipes/](docs/recipes/) — solo, multi-agent, GitHub Actions, adopting on existing project
- [docs/troubleshooting.md](docs/troubleshooting.md)
- [docs/contributing.md](docs/contributing.md)

## License

MIT — see [LICENSE](LICENSE).
