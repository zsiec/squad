# Commands

Every subcommand the `squad` binary ships, by group. Each entry shows the synopsis (auto-generated from `squad <verb> --help`), one-line description, and an example.

To regenerate this page from the binary itself: `squad <verb> --help`.

## Identity

### `squad register`

Register this agent in the squad global database.

```bash
squad register --as agent-blue --name "Alice"
```

Idempotent; re-running with the same `--as` updates the display name and bumps `last_tick_at`.

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
```

### `squad next`

List ready items in priority order.

```bash
squad next                    # default top of stack
squad next --limit 10         # show more
```

An item is "ready" when its `blocked-by:` list is empty (or all references are done) and no one currently holds the claim.

### `squad status`

Show in-progress / ready / blocked counts for this repo.

```bash
squad status
```

### `squad dump-status`

Emit `STATUS.md` from current DB and items state.

```bash
squad dump-status > STATUS.md
```

## Claims

### `squad claim`

Atomically claim an item.

```bash
squad claim FEAT-001 --intent "wire the export button to the API"
```

Exit non-zero if another agent already holds it. Add `--touches path1 path2` to declare files you'll be editing so peers see the overlap.

### `squad release`

Release your claim on an item.

```bash
squad release FEAT-001
```

### `squad done`

Mark an item done: release claim + rewrite frontmatter to `status: done` + move file to `.squad/done/`.

```bash
squad done FEAT-001 --summary "shipped, all tests green"
```

### `squad blocked`

Mark item blocked: release claim + status: blocked + ensure `## Blocker` section exists.

```bash
squad blocked FEAT-001 --reason "waiting on auth.proto from API team"
```

### `squad reassign`

Transfer your claim by releasing it and @-mentioning the new owner.

```bash
squad reassign FEAT-001 @agent-bob
```

### `squad force-release`

Admin: forcibly release someone else's stuck claim. Requires `--reason` for the audit trail.

```bash
squad force-release BUG-042 --reason "agent-blue offline >2h, no response"
```

### `squad handoff`

Post a handoff brief and release any held claims.

```bash
squad handoff "EOD; FEAT-001 in review, FEAT-002 ready"
```

## Chat

### `squad tick`

Show new messages since last tick and advance the read cursor.

```bash
squad tick
```

### `squad thinking` / `milestone` / `stuck` / `fyi`

Typed verbs. All post to your active claim's thread by default; `--thread <ID>` overrides.

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
squad review-request FEAT-001 --reviewer agent-bob
```

### `squad progress`

Report progress on a held item.

```bash
squad progress FEAT-001 --pct 60 --note "AC 2 of 4 done"
```

### `squad tail`

Print recent messages, optionally streaming new ones.

```bash
squad tail                     # last 50 messages
squad tail --since 1h          # last hour
squad tail --follow            # stream
```

### `squad history`

Print all messages for an item, in time order.

```bash
squad history FEAT-001
```

### `squad archive`

Roll old chat messages into a per-month archive DB.

```bash
squad archive --before 2026-03-01
```

## File touches

### `squad touch`

Declare files you are editing so peers see the overlap.

```bash
squad touch internal/cache/flusher.go internal/cache/flusher_test.go
```

### `squad untouch`

Release file touches; no paths releases all touches for this agent.

```bash
squad untouch                                      # release all
squad untouch internal/cache/flusher.go            # release one
```

## Hygiene

### `squad doctor`

Diagnose stale claims, ghost agents, orphan touches, broken refs, and DB integrity.

```bash
squad doctor
```

Exit 0 when clean; non-zero with a problem list when not.

## Multi-repo

### `squad workspace status`

Per-repo summary table across every repo registered in the global DB.

```bash
squad workspace status
squad workspace status --repo current
squad workspace status --repo id1,id2,id3
```

### `squad workspace next`

Top ready items across every repo by priority.

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
squad install-hooks --yes                         # use defaults (only session-start ON)
squad install-hooks --yes --pre-commit-tick=on    # tune individually
squad install-hooks --status                      # what is installed
squad install-hooks --uninstall                   # remove all squad-managed entries
```

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
