# Adopting squad

This is the cold-start walkthrough. If you can finish it without DM-ing anyone for help, the docs did their job.

## TL;DR

```bash
go install github.com/zsiec/squad/cmd/squad@latest
cd ~/dev/your-project
squad go                      # init, register, claim top ready item (the example), print AC, flush chat
# ... follow the steps inside the example item ...
squad done EXAMPLE-001 --summary "loop complete"
```

Total time target: under 5 minutes from `go install` to first `squad done`.

## Day 0 — install

```bash
go install github.com/zsiec/squad/cmd/squad@latest    # or `brew install zsiec/tap/squad` once Phase 14 ships
squad version
```

Expected output: `0.1.0-dev` or similar. If `squad: command not found`, your `$GOPATH/bin` isn't on PATH. Add it:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Persist that in your shell rc.

## Day 0 — initialize a repo

```bash
cd ~/dev/your-project           # must be a git repo (any state — even zero commits)
squad init
```

`init` asks ≤3 questions:

1. **Project name** — defaults to the directory basename. Press enter to accept.
2. **ID prefixes** — comma-separated list. Defaults are fine for most.
3. **Install plugin?** — `Y` if you use Claude Code, `n` if you don't.

What lands on disk:

- `.squad/items/EXAMPLE-001-try-the-loop.md` — a tutorial item that walks through the loop while you do it.
- `.squad/items/`, `.squad/done/` — directories for items you'll file.
- `.squad/config.yaml` — project config; tune later.
- `AGENTS.md` — generic agent doctrine doc.
- `CLAUDE.md` — managed block injected (or file created).
- `.gitignore` — squad lines appended (only the patterns it owns; existing rules are left alone).

Verify and commit:

```bash
git status                      # see what changed
git diff CLAUDE.md              # see the managed block (if CLAUDE.md already existed)
git add .squad/ AGENTS.md CLAUDE.md
git commit -m "chore: adopt squad"
```

## Day 0 — onboard with `squad go`

```bash
squad go
```

One command does init (if not already), registers a session-derived agent id, picks the top ready item (the example item on first run), claims it atomically, prints its acceptance criteria, and flushes any unread chat into your context.

`squad go` is idempotent — re-run it any time to resume the same claim. The session id is derived from your terminal session (`SQUAD_SESSION_ID`, `TERM_SESSION_ID`, etc.), so each terminal tab gets its own agent_id automatically.

If you want an explicit override — different display name, scripted setups — `squad register` accepts `--as <id> --name "..."`; otherwise let `squad go` derive both.

## Day 0 — your first item

`squad go` already opened EXAMPLE-001 for you and printed its AC. Follow the steps inside the item:

1. Read the AC.
2. Check off each box as you do it.
3. Post `squad milestone` when an AC clears.
4. Run any local tests the item names.
5. `squad done EXAMPLE-001 --summary "loop complete"`.

After `done`, the file moves to `.squad/done/`. Commit:

```bash
git add .squad/
git commit -m "chore: complete first squad loop"
```

That's the whole cycle. Everything else is repetition.

## Day 1 — your first real item

```bash
squad new feat "the smallest real thing you can think of in this repo"
```

It writes a frontmatter-only stub at `.squad/items/FEAT-001-...md`. Open it and fill in:

- `## Problem` — what's wrong / what doesn't exist.
- `## Acceptance criteria` — the list of testable things; **be specific**, not "works correctly."
- `## Notes` — anything else.

Then claim, work, test, review, done — same loop.

## Day 1 — install hooks (optional)

```bash
squad install-hooks
```

Interactive: asks about each hook. Recommended for solo:

- `session-start` — Y (default; ensures the session has a derived agent id on Claude Code launch)
- `pre-commit-pm-traces` — Y if you tend to leak ticket IDs into commits
- `pre-edit-touch-check` — n (no peers)

For multi-agent, install `pre-edit-touch-check` too. See [reference/hooks.md](reference/hooks.md).

## Day 2 — multi-agent (if applicable)

Open a second Claude Code session in the same repo and run `squad go` in each. The SessionStart hook (if installed) keys the agent_id off `TERM_SESSION_ID` so each session gets a distinct ID automatically. If you skipped the hook, set a unique session env var per shell first (`SQUAD_SESSION_ID`, `TERM_SESSION_ID`, etc.) — without one, both shells share the same persisted agent-id file and the second registration overwrites the first:

```bash
SQUAD_SESSION_ID=second squad go
```

Now both sessions can `claim` independently. Try claiming the same item from both — one wins, one gets a clear error. That's the lock at work. See [recipes/multi-agent-parallel-claude-sessions.md](recipes/multi-agent-parallel-claude-sessions.md) for the full guide.

## Day 7 — hygiene

```bash
squad doctor                    # should be clean if you've been releasing claims promptly
```

If `doctor` flags stale claims (yours from sessions that crashed or that you forgot to release), follow its suggested commands. See [concepts/hygiene.md](concepts/hygiene.md).

Run `squad doctor` weekly as a habit, even when nothing seems wrong. It also checks the global DB integrity.

## When things go wrong

See [troubleshooting.md](troubleshooting.md). The fastest path to a fix:

1. `squad doctor` — clears 80% of issues.
2. `squad workspace list` — confirms the repo is registered.
3. File a `squad new bug "<symptom>"` against squad itself if the issue is a real bug. Your snag is the next person's snag.

## When you graduate

You'll know you've adopted squad when:

- You don't think about the loop anymore — you just do it.
- You file items reflexively, without deliberating.
- `squad doctor` is silent for a week at a time.
- You can't remember what coordinating with peers was like before atomic claims.

That's the success criterion. The loop is invisible when it's working.
