# squad

[![CI](https://github.com/zsiec/squad/actions/workflows/ci.yml/badge.svg)](https://github.com/zsiec/squad/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Project management for software work done with AI coding agents.

Squad gives Claude Code the durable coordination layer it lacks on its own — atomic claims, typed chat verbs, file-touch tracking, an evidence ledger, and a multi-agent dashboard. All of it is exposed through **36 MCP tools**, so Claude does the squad work for you. You describe what you want; squad's plumbing makes it happen.

> ⚠️ **Status:** under active development, pre-1.0.

## Quick start

**Step 1.** Install the plugin.

```bash
claude install github.com/zsiec/squad
```

That's all the install does — drops the manifest, registers the MCP server, wires the always-on hooks. Squad's tools become available to every Claude Code session in any repo.

**Step 2.** Open Claude Code in your project and tell it what you want.

> *"Claim the top ready item and walk me through it."*

Claude calls `squad_next` to find the priority pick, `squad_claim` to lock it, prints the acceptance criteria, and flushes any pending peer chat into your context. You start working.

**Step 3.** When the work is done, say so.

> *"Mark this done with summary 'shipped retry logic'."*

Claude calls `squad_done`. If the item declared `evidence_required: [test, review]`, Claude first records each verification with `squad_attest` — capturing the command output, the exit code, and a content hash so the proof survives. The item moves to `.squad/done/` and the next one's ready.

That's the whole loop. You never run a squad command yourself.

> If you prefer typing, `/work` is the slash-command equivalent of step 2.

## Beyond the quick start

A claim → work → done loop is the whole shape of squad. The next layer of the surface is what makes it durable past one session.

**Items live in your repo, not in a tracker.** Every item is a markdown file under `.squad/items/<TYPE>-<NN>-<slug>.md` with YAML frontmatter (priority, type, evidence-required, blockers). They're git-tracked, so the queue travels with the repo. Ask Claude to file one — *"file a bug for the retry-on-503 panic"* — and `squad_new` scaffolds it. See [docs/concepts/the-loop.md](docs/concepts/the-loop.md).

**Chat is durable and typed.** Squad's chat verbs (`ask`, `say`, `fyi`, `milestone`, `stuck`) write to a SQLite-backed bus that outlives any session. A teammate's question yesterday is still in your inbox today. The plugin's hooks deliver pending chat at session-start, between tool calls, and before context compaction — no polling. Concepts: [docs/concepts/chat-cadence.md](docs/concepts/chat-cadence.md).

**Multiple agents on one repo, cleanly.** Atomic SQLite `BEGIN IMMEDIATE` claims mean two Claude Code sessions can't both grab the same item — exactly one wins, the other gets a clean error. File-touch tracking warns before peers collide on the same file. The hands-on walkthrough is at [docs/recipes/multi-agent-parallel-claude-sessions.md](docs/recipes/multi-agent-parallel-claude-sessions.md).

**Evidence-gated done.** Items can declare `evidence_required: [test, review]` in frontmatter. `squad_done` refuses to close them without an attestation per kind, and each attestation captures the command, exit code, stdout, and a content hash. The ledger lives at `.squad/attestations/`. The reasoning behind it: [docs/concepts/the-loop.md](docs/concepts/the-loop.md).

**Multi-repo views.** Squad keeps an operational DB at `~/.squad/global.db` covering every repo on the machine. Ask Claude *"what's ready across all my projects?"* and the workspace queries surface a unified ready stack and chat history. Concepts: [docs/concepts/multi-repo.md](docs/concepts/multi-repo.md).

**Live dashboard.** Ask Claude to start `squad serve` (or run it yourself) and visit http://localhost:7777 — live SSE feed of who-has-what, item flow across repos, and an Insights panel charting verification rate, claim p99 latency, and WIP-cap violations over time. The same data is at `GET /api/stats` and `GET /metrics` (Prometheus exposition). Recipe: [docs/recipes/prometheus.md](docs/recipes/prometheus.md).

**When things go wrong.** Ask Claude to run `squad_status` for a quick health check or `squad_doctor` for the full diagnostic — stale claims, ghost agents, orphan touches, broken refs, DB integrity. Common failure modes and recovery paths are at [docs/troubleshooting.md](docs/troubleshooting.md).

**Without Claude Code.** Squad ships a full CLI for scripting, CI, and power-user use. `squad init`, `squad go`, `squad attest`, `squad doctor`, every chat verb. Install the binary on its own:

```bash
go install github.com/zsiec/squad/cmd/squad@latest
```

The complete command reference is at [docs/reference/commands.md](docs/reference/commands.md).

**Comparing to other tools.** Squad's nearest neighbor is Claude Code's own experimental [agent-teams](https://code.claude.com/docs/en/agent-teams), which is great for ephemeral single-session coordination. Squad is for work that outlives the session — multiple days, multiple machines, durable history. The full decision matrix is at [docs/concepts/squad-vs-agent-teams.md](docs/concepts/squad-vs-agent-teams.md).

## Full documentation

[docs/README.md](docs/README.md) is the entry point — concepts, references, recipes, troubleshooting, contributing — all linked from there.

## License

MIT — see [LICENSE](LICENSE).
