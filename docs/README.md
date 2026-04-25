# squad documentation

## What squad is

Squad is a project-management framework for software work done with AI coding agents. It encodes an operating loop — atomic claims, typed chat verbs, file-touch tracking, hygiene sweeps, web dashboard, and an optional Claude Code plugin — into a single static binary. One binary works for solo and multi-agent setups; multi-agent is a configuration, not a separate product.

## Why

AI agents meander without structure: half-finished branches, redundant work, opaque "I think it's done." Existing PM tools optimize for humans typing in web forms, which is the wrong shape for sessions that come and go. Squad pushes the doctrine into enforceable CLI patterns — the agent files claims, ticks for chat, posts evidence at done — and stays out of the way of solo work.

## 5-minute quickstart

```bash
# 1. Install
go install github.com/zsiec/squad/cmd/squad@latest    # or `brew install zsiec/tap/squad` once Phase 14 ships

# 2. Onboard or resume in one step
cd ~/dev/your-project
squad go                                              # init, register, claim top ready item, print AC, flush chat

# 3. Do the work, then close
# ... edit, test, commit ...
squad done FEAT-001 --summary "shipped"
```

`squad go` is idempotent — first run inits `.squad/` and registers a session-derived agent id; re-runs resume the same claim and re-flush the mailbox. Total time: under five minutes from `go install` to first `done`.

## What to read next

- **New?** Walk through [adopting.md](adopting.md) end-to-end.
- **Pick a recipe** for your starting situation: [recipes/](recipes/).
- **Doctrine** behind the loop: [concepts/](concepts/).
- **Lookup** for commands, config, hooks, skills, slash commands, and the DB schema: [reference/](reference/).
- **Snag?** [troubleshooting.md](troubleshooting.md).
- **Contribute?** [contributing.md](contributing.md).
