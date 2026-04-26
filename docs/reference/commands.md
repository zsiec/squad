# Commands

Every subcommand the `squad` binary ships, by group. Each entry shows the synopsis (auto-generated from `squad <verb> --help`), one-line description, and an example.

To regenerate this page from the binary itself: `squad <verb> --help`.

## Identity

### `squad register`

Register this agent in the squad global database. Zero-arg form derives a session-stable id and display name from your terminal session — no flags needed for the common case. Pass `--as <id> --name "..."` only when you want explicit overrides (scripted setups, log-readable names).

```bash
squad register                                # session-derived id and name
squad register --as agent-blue --name "Alice" # explicit override
```

Idempotent; re-running updates the display name and bumps `last_tick_at`. Most agents reach this command via `squad go`, which calls register internally before claiming.

### `squad whoami`

Print the agent id this session resolves to.

```bash
squad whoami
```

### `squad who`

List registered agents with status, current claim, and last tick.

```bash
squad who
```

## Items

### `squad new`

Create a new item file under `.squad/items/`.

```bash
squad new feat "add the export button"
squad new bug "race in the cache flusher"
squad new feat "kafka migration" --priority P0 --area infra --estimate 4h --risk high
```

Flags (each falls back to `defaults.<key>` in `.squad/config.yaml`, finally to a built-in default):
- `--priority P0|P1|P2|P3` (default: P2)
- `--area <tag>` (default: `<fill-in>`)
- `--estimate 30m|1h|4h|1d` (default: 1h)
- `--risk low|medium|high` (default: low)

### `squad next`

List ready items in priority order. Excludes items already claimed unless `--include-claimed` is passed.

```bash
squad next                    # default top of stack
squad next --limit 10         # show more
```

An item is "ready" when its `blocked-by:` list is empty (or all references are done) and no one currently holds the claim.

### `squad status`

Show claimed / ready / blocked / done counts for this repo.

```bash
squad status
```

### `squad dump-status`

Emit `STATUS.md` from current DB and items state.

```bash
squad dump-status > STATUS.md
```

## Specs and epics

### `squad analyze`

Decompose an epic's items into parallel streams. Reads the epic frontmatter, walks the items that reference it, and writes `.squad/epics/<epic>-analysis.md` containing the stream list, file globs, dependency edges, and a parallelism factor.

```bash
squad analyze auth-rework
```

Prints the absolute path to the analysis file on success. Decomposition is deterministic: same inputs produce the same streams.

### `squad epic-new`

Create an epic scaffold under `.squad/epics/<name>.md` with `spec`, `status`, and `parallelism` frontmatter. The `--spec` flag is required and must reference an existing spec slug; the command fails if the spec file does not exist.

```bash
squad epic-new auth-rework --spec auth
```

Prints the absolute path to the created file on success. Names must be kebab-case.

### `squad spec-new`

Create a spec scaffold under `.squad/specs/<name>.md` with `title`, `motivation`, `acceptance`, `non_goals`, and `integration` frontmatter ready to fill in.

```bash
squad spec-new auth "rebuild authentication around OIDC"
```

Prints the absolute path to the created file on success. Names must be kebab-case; titles are free-form.

## Onboarding

### `squad go`

Onboard or resume in one step: init the workspace if `.squad/` is absent, register the agent if not already registered, find the top ready item, claim it, print its AC, and flush the mailbox. Idempotent — running twice does not claim two items; resumes the existing claim instead.

```bash
squad go
```

No flags — the command takes no arguments. The recommended single entry point for both first-time and resume cases. Equivalent to `init --yes` + `register` + `next` + `claim` + `tail` chained, but with the resume semantics built in.

### `squad claim`

Atomically claim an item.

```bash
squad claim FEAT-001 --intent "wire the export button to the API"
```

Exit non-zero if another agent already holds it. Add `--touches path1,path2` (comma-separated) to declare files you'll be editing so peers see the overlap. Use `--long` to apply the 2h long-running threshold instead of `hygiene.stale_claim_minutes`.

### `squad release`

Release your claim on an item.

```bash
squad release FEAT-001
```

### `squad done`

Run the `verification.pre_commit` gate from `.squad/config.yaml`, then release the claim, rewrite frontmatter to `status: done`, and move the file to `.squad/done/`. Pass `--skip-verify` to override the gate locally.

```bash
squad done FEAT-001 --summary "shipped, all tests green"
squad done FEAT-001 --skip-verify --summary "trivial doc fix"
```

### `squad blocked`

Mark item blocked: release claim + status: blocked + ensure `## Blocker` section exists.

```bash
squad blocked FEAT-001 --reason "waiting on auth.proto from API team"
```

### `squad reassign`

Transfer your claim by releasing it and @-mentioning the new owner. The new owner still has to run `squad claim FEAT-001` themselves — reassign is consent-based.

```bash
squad reassign FEAT-001 --to agent-bob
```

### `squad force-release`

Admin: forcibly release someone else's stuck claim. Requires `--reason` for the audit trail.

```bash
squad force-release BUG-042 --reason "agent-blue offline >2h, no response"
```

### `squad handoff`

Post a handoff brief and release any held claims. The brief is structured: each `--shipped/--in-flight/--surprised-by/--unblocks` is repeatable.

```bash
squad handoff \
  --shipped FEAT-001 \
  --in-flight FEAT-002 \
  --note "EOD wrap-up; back tomorrow"
```

Or pipe the note from stdin: `git log --oneline | squad handoff --stdin --shipped FEAT-001`.

## Chat

### `squad tick`

Show new messages since last tick and advance the read cursor. Diagnostic-only in normal operation — chat is delivered continuously via the `Stop` listen + post-tool-flush + user-prompt-tick hooks. Reach for `squad tick` when you suspect a hook miss or want to advance the cursor explicitly.

```bash
squad tick
```

### `squad thinking` / `milestone` / `stuck` / `fyi`

Typed verbs. All post to your active claim's thread by default; `--to <thread>` overrides (`--to global` for the team-wide channel, `--to FEAT-001` for a specific item thread).

```bash
squad thinking "leaning toward suspending the producer rather than throttling"
squad milestone "AC 1 green, moving to AC 2"
squad stuck "cannot reproduce locally — seeing fresh patterns?"
squad fyi "touching shared.go in a way that will conflict with mid-pool work"
```

### `squad ask`

Ask a question of a specific agent.

```bash
squad ask @agent-blue "did the deep-copy change merge yet?"
```

### `squad answer`

Answer a previous message by id.

```bash
squad answer 1234 "yes, merged at 09:42"
```

### `squad knock`

High-priority directed message — interrupts the recipient's tick.

```bash
squad knock @agent-blue "main is broken on your last commit, please look"
```

### `squad say`

Plain chat — escape hatch when no verb fits.

```bash
squad say "lunch break, back at 14:00"
```

### `squad review-request`

Request review on an item, optionally mentioning a reviewer.

```bash
squad review-request FEAT-001 --mention agent-bob
```

### `squad progress`

Report progress on a held item. The percentage is positional (0–100); the note is optional. Only the agent currently holding the claim can report progress on it.

```bash
squad progress FEAT-001 60 --note "AC 2 of 4 done"
```

The report is written to both the `progress` table (source of truth for "latest pct") and the `messages` table (so it shows up in `squad tail` and `squad history`).

### `squad tail`

Print recent messages, optionally streaming new ones.

```bash
squad tail                     # messages from the last 30 minutes
squad tail --since 1h          # last hour
squad tail --follow            # stream
squad tail --thread global     # only the global channel
```

### `squad history`

Print all messages for an item, in time order.

```bash
squad history FEAT-001
```

### `squad standup`

Per-agent digest over a lookback window: what shipped, what was lost (reclaimed or force-released), the currently-open claim (with age + intent), stuck signals you posted, asks you sent that nobody answered, and the file touches you still hold. Defaults to a 24-hour window.

```bash
squad standup                   # last 24h, human-readable
squad standup --since 1w        # custom window
squad standup --json            # machine-readable for scripting / dashboards
```

The digest is read-only; it doesn't mutate the DB.

### `squad archive`

Roll old chat messages into a per-month archive DB.

```bash
squad archive --before 30d         # keep last 30 days; older messages roll into ~/.squad/archive/
```

## File touches

### `squad touch`

Declare files you are editing on a specific item so peers see the overlap.

```bash
squad touch FEAT-001 internal/cache/flusher.go internal/cache/flusher_test.go
```

Signature: `squad touch <ITEM-ID> <path>...`. The first argument is the item ID the touches belong to (usually your active claim); the rest are paths.

### `squad untouch`

Release file touches; no paths releases all touches for this agent.

```bash
squad untouch                                      # release all
squad untouch internal/cache/flusher.go            # release one
```

### `squad touches list-others`

List active file touches held by agents OTHER than the current one. Used by the `pre-edit-touch-check` hook to warn before edits collide.

```bash
squad touches list-others
squad touches list-others --json
```

## Evidence

### `squad attest`

Record a verification artifact (test/lint/build/typecheck/manual/review) into the evidence ledger. Items with `evidence_required: [...]` in their frontmatter need an attestation per kind before `squad done` will close them out (without `--force`).

```bash
# Test, lint, typecheck, build — squad runs the command and stores stdout+exit.
squad attest --item FEAT-001 --kind test     --command "go test ./..."
squad attest --item FEAT-001 --kind lint     --command "golangci-lint run"
squad attest --item FEAT-001 --kind build    --command "go build ./..."

# Review — write findings to a file first; squad reads, hashes, and stores it.
squad attest --item FEAT-001 --kind review \
    --reviewer-agent agent-helper \
    --findings-file /tmp/review.md

# Manual — record an out-of-band verification (compliance sign-off etc).
squad attest --item FEAT-001 --kind manual --command "compliance review e-mailed 2026-04-26"
```

| Flag | Required when | Meaning |
|---|---|---|
| `--item <ID>` | always | Item id; the attestation is scoped to one item. |
| `--kind <kind>` | always | One of `test`, `lint`, `typecheck`, `build`, `review`, `manual`. |
| `--command <cmd>` | always except `kind=review` | Shell command to run; squad captures stdout and exit code into the ledger. |
| `--findings-file <path>` | `kind=review` | File whose body becomes the review record. |
| `--reviewer-agent <id>` | `kind=review` | The agent id of the reviewer. |

The attestation is stored under `.squad/attestations/<hash>.txt` (committed) and indexed in `~/.squad/global.db` (operational). Records are deduplicated on `(repo_id, item_id, kind, output_hash)` — re-running an identical attestation is a no-op.

## Learning

R5 learning artifacts live under `.squad/learnings/{actions,patterns,nots}/{proposed,approved,rejected}/`. `squad learning` is the management surface; the underlying files are plain markdown so peers can review them via PR.

### `squad learning propose`

Stub a new learning artifact under `proposed/`. Asks for the body interactively (or pass `--body-file`). Learning kinds are `actions` (do this), `patterns` (this works), `nots` (don't do this).

```bash
squad learning propose actions retry-on-503 --area "http retries"
squad learning propose nots dont-mock-the-db --body-file ./learning.md
```

### `squad learning list`

List learning artifacts. Filter by area, state, or kind.

```bash
squad learning list                          # all states, all kinds
squad learning list --state proposed
squad learning list --kind nots --area auth
squad learning list --json
```

### `squad learning approve`

Promote a proposed learning to `approved/`. The slug is the directory name under `proposed/`.

```bash
squad learning approve retry-on-503
```

### `squad learning reject`

Archive a proposed learning under `rejected/` (preserved for audit; the proposal directory and body are kept).

```bash
squad learning reject not-a-real-pattern --reason "duplicates retry-on-503"
```

### `squad learning agents-md-suggest`

Propose a unified-diff change to `AGENTS.md` based on an approved learning. The diff goes into `.squad/learnings/agents-md/proposed/` for human review.

```bash
squad learning agents-md-suggest --learning retry-on-503
```

### `squad learning agents-md-approve`

Apply a proposed `AGENTS.md` diff via `git apply`; on success archive the proposal.

```bash
squad learning agents-md-approve <id>
```

### `squad learning agents-md-reject`

Archive a proposed `AGENTS.md` change under `rejected/` without applying.

```bash
squad learning agents-md-reject <id> --reason "policy already covers this"
```

### `squad learning triviality-check`

Internal helper consumed by the `stop-learning-prompt` hook: reads `git diff --numstat` from stdin and prints `trivial` or `non-trivial`. Not normally invoked by hand.

## Hygiene

### `squad doctor`

Diagnose stale claims, ghost agents, orphan touches, broken refs, and DB integrity.

```bash
squad doctor                  # report findings; exit 0 either way
squad doctor --strict         # exit non-zero when findings exist (CI use case)
```

The default exit code is 0 even with findings — the output is diagnostic, meant for the user to read and act on. Pass `--strict` only in CI where you want a failing build on any finding.

## Statistics

### `squad stats`

Operational statistics for the current repo: verification rate, claim p99 latency, WIP-violation count, reviewer disagreement rate, and a daily series for each. Pure read-side aggregation over the existing tables; no new state.

```bash
squad stats                              # human summary
squad stats --json                       # full pretty-printed snapshot
squad stats --tail --interval 5s         # NDJSON stream until SIGINT
squad stats --window 1h                  # last hour only (default 24h)
squad stats --window 0                   # unbounded
```

The same data is exposed at `/api/stats` (when `squad serve` is running) and `/metrics` for Prometheus scrape — see [recipes/prometheus.md](../recipes/prometheus.md).

## Multi-repo

### `squad workspace status`

Per-repo summary table across every repo registered in the global DB.

```bash
squad workspace status
squad workspace status --repo current
squad workspace status --repo id1,id2,id3
```

### `squad workspace next`

Top P0/P1 ready items across every repo (lower-priority items aren't shown — drill into a single repo with `squad next` for those).

```bash
squad workspace next --limit 10
```

### `squad workspace who`

Every registered agent across every repo, with current claim and last tick.

```bash
squad workspace who
```

### `squad workspace list`

Every known repo with origin URL and last-seen-at.

```bash
squad workspace list
```

### `squad workspace forget`

Remove a repo from the global DB (after deleting it locally).

```bash
squad workspace forget <repo_id>
```

## GitHub integration

`squad pr` is the parent for the GitHub-pull-request integration; the leaf commands below are the verbs you'll usually invoke.

### `squad pr-link`

Record a pending PR ↔ item mapping (run before `gh pr create`). Prints the hidden HTML marker to embed in the PR body.

```bash
squad pr-link FEAT-001
squad pr-link --write-to-clipboard FEAT-001
squad pr-link --pr 42 FEAT-001                     # append to existing PR via gh pr edit
```

### `squad pr-close`

Archive the squad item linked to a merged PR. CI-only — invoked by the auto-archive workflow.

```bash
squad pr-close 42
squad pr-close 42 --repo-id "owner/repo"
```

## Plugin and hooks

### `squad install-plugin`

Install the squad Claude Code plugin to `~/.claude/plugins/squad/`.

```bash
squad install-plugin
squad install-plugin --uninstall
```

### `squad install-hooks`

Install or update squad's Claude Code hooks in `~/.claude/settings.json`.

```bash
squad install-hooks                               # interactive
squad install-hooks --yes                         # accept defaults (six hooks ON)
squad install-hooks --yes --pre-commit-pm-traces=on    # tune individually
squad install-hooks --status                      # what is installed
squad install-hooks --uninstall                   # remove all squad-managed entries
```

Default-ON hooks (`--yes`): `session-start`, `user-prompt-tick`, `pre-compact`, `stop-listen`, `post-tool-flush`, `session-end-cleanup`. Default-off (opt-in): `async-rewake`, `pre-commit-pm-traces`, `pre-edit-touch-check`, `stop-learning-prompt`, `loop-pre-bash-tick`. See [hooks.md](hooks.md) for the full list and what each does.

## Real-time transport

Three commands together implement squad's chat real-time delivery. Normal users don't invoke them directly — the hooks installed by `squad install-hooks` do — but understanding what they do helps when something looks wrong.

### `squad listen`

Block until a peer message wakes this session; emit a Claude Code decision-block JSON envelope on wake. Bound by the `stop-listen` hook to a loopback TCP listener.

```bash
squad listen --instance my-session --bind 127.0.0.1:0 --max 24h
```

| Flag | Default | Meaning |
|---|---|---|
| `--instance <id>` | derived from env | Stable session identifier; the row in `notify_endpoints` is keyed on this. |
| `--bind <addr>` | `127.0.0.1:0` | Bind address for the loopback listener. Must be loopback. |
| `--fallback <dur>` | `30s` | Fallback re-check interval when no wake arrives. |
| `--max <dur>` | `24h` | Hard exit after this duration with no wake. |

### `squad mailbox`

Print pending mailbox as a Claude Code hook envelope; exit 0 if empty. Used by the `user-prompt-tick`, `post-tool-flush`, and `pre-compact` hooks to inject pending peer chat as `additionalContext`.

```bash
squad mailbox                         # default: additional-context format, UserPromptSubmit event
squad mailbox --event PreCompact
squad mailbox --format text           # plain text instead of JSON envelope
```

### `squad notify-cleanup`

Drop `notify_endpoints` rows for a given instance. Called by the `session-end-cleanup` hook so peer senders stop dialing a dead listener port.

```bash
squad notify-cleanup --instance my-session
```

## MCP

### `squad mcp`

Run an MCP server over stdio (Claude Code transport). Registered in `~/.claude/plugins/squad/.mcp.json` by `squad install-plugin`; not normally invoked by hand.

```bash
squad mcp
```

Exposes 23 tools to Claude Code: lifecycle (claim, done, release, blocked), chat verbs (say, ask, fyi, milestone, thinking, stuck), inspection (next, status, get_item, list_items), evidence (attest, attestations), learning (propose, list, approve, reject), and progress/touch helpers. See `~/.claude/plugins/squad/.mcp.json` for the registered command line.

## Server

### `squad serve`

Start the squad dashboard (HTTP + SSE).

```bash
squad serve --port 7777 --bind 127.0.0.1
```

## Scaffold

### `squad init`

Scaffold a squad workspace in the current repository: writes `.squad/`, `AGENTS.md`, and a CLAUDE.md managed block. Asks ≤3 questions.

```bash
squad init
squad init --yes                  # accept all defaults
```

## Diagnostics

### `squad version`

Print the squad version.

```bash
squad version
```

## Interop

### `squad bridge agent-teams`

**Status:** Specified, implementation deferred until Claude Code's agent-teams API exits experimental. Reflects squad-managed items into an active agent-teams session as native tasks.

```text
squad bridge agent-teams [--team <name>] [--items <filter>] [--once]
```

#### What it does

Mirrors the current repo's pending-and-claimed squad items into the active agent-teams team's task directory at `~/.claude/tasks/<team>/` so an agent-teams lead and teammates can see them in their native task list. The mirror is one-way (squad → agent-teams) and session-scoped (torn down on `SIGINT`, `SessionEnd`, or `--once` exit). Status changes from the agent-teams side update squad's claim `last_touch` only; they cannot transition an item to `done`. There is no flag to enable two-way sync — the bridge is read-mostly by design.

#### Flags

| Flag | Default | Meaning |
|---|---|---|
| `--team <name>` | `default` | The agent-teams team directory under `~/.claude/tasks/`. Created if absent (with the team's own consent — the bridge will not create a directory in another team's namespace). |
| `--items <filter>` | `pending,claimed` | Comma-separated squad statuses to mirror. `done` is intentionally not mirrorable. |
| `--once` | off | Mirror once and exit instead of watching. Useful for scripting; default is to watch. |

#### Naming convention

Bridged tasks are prefixed `squad:` in the agent-teams task list. Example: `squad:FEAT-007 — wire payment retries`. The prefix makes provenance unambiguous so a teammate doesn't mistake a bridged item for a native agent-teams task.

#### Lifecycle

```text
agent-teams session start
  └─► squad bridge agent-teams --team my-team
       │
       ├─► initial mirror: writes pending-and-claimed squad items
       ├─► watch loop: polls squad DB every 2s for changes
       │     ├─► claim → updates task entry's "claimed by" field
       │     ├─► release → unclaims the task entry
       │     ├─► touch → bumps task entry's last-update timestamp
       │     └─► done → removes the task entry from agent-teams
       │
       └─► on SIGINT / SessionEnd / --once exit:
            └─► tears down the mirror, leaves squad untouched
```

#### What it does NOT do

- Mark items `done` from inside agent-teams. Closing requires `squad done` with evidence.
- Sync chat between the two systems.
- Persist between agent-teams sessions. Each new session re-mirrors fresh.
- Mirror items from repos other than the cwd-rooted one.
- Reach across hosts. Both squad and agent-teams must be on the local machine.

#### Why deferred

The on-disk format under `~/.claude/tasks/<team>/` is flagged as subject to change while agent-teams is experimental. Squad will not ship a stable bridge against an unstable upstream. The implementation re-enters the queue when **either** of the following ships upstream:

1. Agent-teams exits experimental and the on-disk format is documented as stable, OR
2. Anthropic publishes a `claude tasks` (or equivalent) command surface that gives a stable shell-level API for reading and writing the same data.

When that happens, the implementation can match the behavior specified above.

#### See also

- [concepts/squad-vs-agent-teams.md](../concepts/squad-vs-agent-teams.md) — when this command is the right tool.
- [recipes/migrating-from-agent-teams.md](../recipes/migrating-from-agent-teams.md) — for the case where you want to leave agent-teams entirely instead of bridging.
