# squad documentation

## What squad is

Squad is a project-management framework for software work done with AI coding agents. It encodes an operating loop — atomic claims, typed chat verbs, file-touch tracking, hygiene sweeps, web dashboard, and an optional Claude Code plugin — into a single static binary. One binary works for solo and multi-agent setups; multi-agent is a configuration, not a separate product.

## Why

AI agents meander without structure: half-finished branches, redundant work, opaque "I think it's done." Existing PM tools optimize for humans typing in web forms, which is the wrong shape for sessions that come and go. Squad pushes the doctrine into enforceable CLI patterns — the agent files claims, ticks for chat, posts evidence at done — and stays out of the way of solo work.

## 5-minute quickstart

```bash
# 1. Install
go install github.com/zsiec/squad/cmd/squad@latest    # or `brew install zsiec/tap/squad` once Phase 14 ships

# 2. Initialize a repo
cd ~/dev/your-project
squad init                                            # answers ≤3 questions

# 3. Register and pick up your first item
squad register --as agent-you --name "Your Name"
squad next                                            # see what's ready
squad new feat "your first item"                      # or claim the example item already in items/
squad claim FEAT-001 --intent "first squad item"

# 4. Do the work, then close
# ... edit, test, commit ...
squad done FEAT-001 --summary "shipped"
```

Total time: under five minutes from `go install` to first `done`.

## What to read next

- **New?** Walk through [adopting.md](adopting.md) end-to-end.
- **Pick a recipe** for your starting situation: [recipes/](recipes/).
- **Doctrine** behind the loop: [concepts/](concepts/).
- **Lookup** for commands, config, hooks, skills, slash commands, and the DB schema: [reference/](reference/).
- **Snag?** [troubleshooting.md](troubleshooting.md).
- **Contribute?** [contributing.md](contributing.md).
