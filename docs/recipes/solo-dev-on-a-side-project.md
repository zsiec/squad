# Recipe: Solo dev on a side project

## Who this is for

You're alone on a project, you run one Claude Code session at a time, and you want the operating loop's discipline (atomic claims, evidence-gated done, mandatory review) without the multi-agent ceremony.

## What you'll skip

- Typed verbs (`thinking`, `milestone`, `stuck`, `fyi`, `ask`) — useful but optional in solo flow.
- The `Stop` auto-handoff hook — you'll just close Claude when you're done.
- `squad workspace` queries — only useful with multiple repos.

## Setup (3 commands)

```bash
# 1. Install
go install github.com/zsiec/squad/cmd/squad@latest

# 2. cd into your project (must be a git repo with at least one commit)
cd ~/dev/your-project

# 3. Onboard in one step
squad go                      # init, register, claim top ready item, print AC, flush chat
```

Optional: `squad install-hooks --yes` to add the SessionStart hook (default ON) so future Claude Code launches register the session automatically.

## Your first item end-to-end

```bash
# File something
squad new feat "wire the export button"

# Pick it up
squad go                      # claims FEAT-001, prints AC, flushes any chat

# Do the work
# ... edit, test, commit ...
go test ./...                 # paste the green output
squad milestone "exports working end-to-end"

# Close
squad done FEAT-001 --summary "shipped, /api/export wired"
git add .squad/ && git commit -m "feat: wire the export button"
```

## Daily flow

- Open Claude Code in the project. The SessionStart hook ensures the session has a derived agent id; chat is delivered continuously via the `Stop` listen + post-tool-flush + user-prompt-tick hooks, so unread mentions reach you without a manual tick.
- Run `squad go` to claim the top ready item (or resume an in-progress claim) and have the AC printed back to you.
- Otherwise pick from `squad next`, claim, work the loop.
- Close at end of session with `squad done` or `squad release` if you'll resume tomorrow.

## When to graduate to multi-agent

Once you start running 2+ Claude Code sessions in parallel — different terminals, different worktrees, different machines — switch to the [multi-agent recipe](multi-agent-parallel-claude-sessions.md). Atomic claims are doing real work for you at that point: the same `claim FEAT-001` from two sessions will race in SQLite and exactly one wins.

## Why bother with the loop solo?

Same reason any developer keeps a structured workflow alone: future-you reading your own commits in three months can't tell which "fix" was real and which was performative. Premise validation before fixing, evidence pasted at done, review on every commit — these are forcing functions on the agent (and on you reading the agent's output) so the work that lands in main is durable.
