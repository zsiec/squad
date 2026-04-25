# Recipe: Multi-agent parallel Claude sessions

## Why this works

Atomic claims live in `~/.squad/global.db`, a SQLite file with `BEGIN IMMEDIATE` semantics. When two agents race for the same item, exactly one wins; the other gets a clean "already claimed" error and picks something else. No corruption, no torn state.

The dashboard at `squad serve` shows live which agent holds what, what they last said, and where the file-touch overlaps are.

## Setup: three terminal panes, one repo

In each pane:

```bash
cd ~/dev/your-project
# pane 1
squad register --as agent-blue --name "Blue"
# pane 2
squad register --as agent-green --name "Green"
# pane 3
squad register --as agent-violet --name "Violet"
```

Each pane is now a distinct agent. Open three Claude Code sessions, one per pane. Squad keys the *current* agent identity off the session — first one of `SQUAD_SESSION_ID`, `TERM_SESSION_ID`, `ITERM_SESSION_ID`, `TMUX_PANE`, `STY`, `WT_SESSION` that's set. Terminal apps usually set one of these automatically per pane, so registration is per-pane out of the box.

If you're scripting against multiple agents from a single shell (no terminal session), set `SQUAD_SESSION_ID` explicitly per process:

```bash
SQUAD_SESSION_ID=blue squad register --as agent-blue --name "Blue"
SQUAD_SESSION_ID=blue squad claim FEAT-001 --intent "blue starts"
SQUAD_SESSION_ID=green squad register --as agent-green --name "Green"
SQUAD_SESSION_ID=green squad claim FEAT-002 --intent "green starts"
```

Without a session key, every shell shares one persisted agent-id file in `~/.squad/agent-id.txt`, and the most recent `register` wins.

## Coordination commands

```bash
squad who                                    # who is registered, current claim, last tick
squad tick                                   # surface mentions and conflicts since last tick
squad ask @agent-green "should I rebase?"    # directed question
squad workspace status                       # cross-repo when you have multiple
```

## File-touch warnings

Declare files you're editing on your active claim so peers see the overlap:

```bash
squad touch FEAT-001 internal/cache/flusher.go
# ... edit ...
squad untouch internal/cache/flusher.go      # release when done; or `squad untouch` for all
```

The first arg is the item ID the touches belong to; the rest are paths.

If you install the `pre-edit-touch-check` hook (Phase 11; opt-in), the warning fires automatically when you Edit/Write a file another agent is touching. The hook only warns; it does not block. The right move when warned:

1. Stop and read what the peer is doing (`squad tail --thread <ITEM>` to see their thread).
2. If the work overlaps, post `squad ask @agent-NAME "I'm about to touch X — conflict?"` and wait.
3. If the work doesn't overlap (different functions in the same file), proceed.

## Stale claims

If a peer crashes or walks away mid-claim, the heartbeat goes stale. After `hygiene.stale_claim_minutes` (default 60), `squad doctor` flags it:

```
$ squad doctor
stale claims: 1
  BUG-042  agent-blue   last_tick 2h17m ago    → squad force-release BUG-042 --reason "agent-blue gone"
```

Take over with:

```bash
squad ask @agent-blue "stealing BUG-042, ok?"   # courtesy ping (skip if peer is unreachable)
squad force-release BUG-042 --reason "agent-blue offline >2h"
squad claim BUG-042 --intent "..."
```

The `claim_history` table records the takeover with the reason, so future audits can see what happened.

## Worktree variant

```bash
git worktree add ../your-project-feat-a feat-a
git worktree add ../your-project-feat-b feat-b
```

Open Claude Code in each worktree. Each worktree's CWD has its own `.squad/items/` (because the items are per-branch), but they all share `~/.squad/global.db`. Claims, chat, and touches still coordinate across worktrees correctly.

If the same item ID exists in two branches with different bodies, claiming locks the ID — but the agent in the other worktree will see different acceptance criteria. Avoid this; either keep items on `main` and merge updates promptly, or rename when the AC genuinely diverges.

## Two machines, one repo

Items follow git like normal — both machines pull and push the same `.squad/items/` files. **Claims do not cross machines.** Each machine has its own `~/.squad/global.db` and its own claim namespace. Coordinate at the agent level: "I'm working on FEAT-001 from my laptop today" gets posted to chat, not enforced by the lock.

This is by design for v1. Cross-machine claim sync is a v2 design problem (the design doc has a section on it).

## Anti-patterns

- **Claiming and walking away** for >30 min without ticking. Heartbeat handles brief absence; long absence stales the claim.
- **Editing a file without `squad touch`** in a multi-agent session. Peers can't see your overlap and the touch-check hook can't help.
- **Force-releasing without checking.** The peer might be 30 seconds from posting a milestone. Try `squad ask @agent-NAME` first.
- **Treating `claim` as a property.** It's a CLI command and a DB row, not a frontmatter field. The item file's `status:` updates only at `squad done`.
