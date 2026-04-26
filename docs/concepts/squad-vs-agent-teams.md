# squad vs. Claude Code agent-teams

Squad and [Claude Code's experimental agent-teams](https://code.claude.com/docs/en/agent-teams) overlap on the surface — both coordinate multiple Claude sessions working on the same codebase. They're aimed at different problem shapes, and choosing the wrong one will hurt. This page is the decision matrix.

## TL;DR

- **Reach for agent-teams** when you want a few agents to crank on a single feature in one session, on one machine, and you're going to walk away when the session ends.
- **Reach for squad** when the work outlives the session: ongoing project, multiple machines, agents that come and go, history you want to look at next week.
- **They compose.** You can run an agent-teams session inside a squad-managed repo. The bridge command (see below) makes squad-claimed items visible to the agent-teams lead.

## Decision matrix

| Dimension | agent-teams | squad |
|---|---|---|
| **Lifetime** | Single session. Tasks vanish when the lead exits. | Durable. Items survive sessions, restarts, machine swaps. |
| **Storage** | `~/.claude/tasks/<team>/tasks.json` (file-locked JSON). | `.squad/items/*.md` (git-committed) + `~/.squad/global.db` (operational). |
| **Hosts** | Single host. No network sync built in. | Cross-host. Items travel with the repo; operational state is per-machine. |
| **Resumption** | None. New session = new team. | First-class. `squad work` reattaches in one command. |
| **Lead role** | Fixed for the life of the team. | No lead role. Agents are peers; coordination is via claim ledger. |
| **Membership** | Up to 4 teammates per docs at writing; lead-spawned. | Unbounded (subject to WIP cap per agent identity). |
| **Repo awareness** | None. Tasks are session-scoped, not repo-scoped. | Items, claims, hygiene all repo-scoped. |
| **Maturity** | Experimental. `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`. API may change. | GA. Pure-Go static binary. SemVer. |
| **Persistence of decisions** | Mailbox messages flushed at session end. | Chat verbs (`squad ask`, `squad milestone`, `squad fyi`) durably stored. |
| **Verification primitive** | None native. | Evidence ledger (`squad attest`) with DoD-gated `squad done`. |
| **Hygiene / stale claims** | None. | `squad doctor`, stale-claim sweeps, force-release. |
| **Claim semantics** | File lock per task. | Atomic SQLite `BEGIN IMMEDIATE` per item, with takeover audit trail. |
| **Onboarding cost** | One env var + lead spawn. | One `squad init` + `squad work`. |
| **Cost when wrong** | Lose the session, lose the work coordination. | Operational overhead for short bursts. |

## When agent-teams is the right tool

Pick agent-teams when **all** of the following hold:

1. You're going to drive the work to completion in one sitting (single session, no walk-away).
2. Everyone's on the same machine.
3. The artifacts (commits, PRs) are the only things that need to outlive the session — coordination state can vanish.
4. You don't need a lead-handoff midway through.
5. You want zero install friction beyond what Claude Code already provides.

Examples that fit:
- "Refactor this 200-line file with a helper agent verifying my edits."
- "Build this small feature with a primary + reviewer pair, ship the PR, done."
- "One-shot research: dispatch three sub-agents to look at three subdirectories."

## When squad is the right tool

Pick squad when **any** of the following hold:

1. The work is going to span multiple sessions, restarts, or days.
2. Multiple machines are involved (your laptop + a dev box; you + a teammate).
3. You want a durable record: who claimed what, when, and how it ended.
4. You need verification gates (evidence ledger, DoD-gated done).
5. You want WIP caps, hygiene sweeps, or force-release semantics.
6. You're operating on a shared repo with multiple agents over time.

Examples that fit:
- "Ongoing project where I or my agents come back to it across many days."
- "Two laptops, sometimes a remote box; I want one place to see all in-flight items."
- "I want CI to flag stale claims and a doctor command that tells me what's broken."
- "I want a record of which agent verified what, with hashes."

## Where they overlap

Both have:
- Claims (file lock vs. atomic DB row).
- Inter-agent messaging (SendMessage vs. squad chat verbs).
- Hooks at session boundaries (`TeammateIdle`/`TaskCreated`/`TaskCompleted` vs. squad's `Stop`/`PostToolUse`/`UserPromptSubmit`).
- Shared task surface within the team / repo.

The overlap is real. Squad does **not** try to replace agent-teams for the cases where agent-teams is better. The question is which lifetime fits the work.

## Composing the two

The two compose cleanly because they live at different layers:

- **agent-teams** is session-scoped coordination.
- **squad** is durable repo-scoped coordination.

A typical compose pattern: open a squad-managed repo, claim a multi-step item with `squad claim`, spawn an agent-teams session to drive it, the team's lead and teammates collaborate inside that one squad claim, on close-out the lead runs `squad done` with evidence. Squad sees one claim by one identity; agent-teams sees one team driving one task. No conflict.

The optional `squad bridge agent-teams` command (see [bridge spec](#optional-the-bridge-command)) reflects squad's pending-and-claimed items into the agent-teams task list so the lead can see them as native tasks. It is one-way (squad → agent-teams), session-scoped, and tears down when the team session ends.

## Optional: the bridge command

`squad bridge agent-teams [--team <name>] [--items <filter>]`

**Status:** Specified in `docs/reference/commands.md`. Implementation is gated on the agent-teams on-disk format exiting experimental. As of writing, the format is documented but flagged as subject to change; squad will not ship a stable bridge against an unstable upstream.

**Behavior (when implemented):**
1. Reads squad's pending-and-claimed items in the current repo.
2. Writes them as agent-teams tasks into `~/.claude/tasks/<team>/tasks.json`, prefixed `squad:` to make their origin obvious.
3. Watches for status changes from the agent-teams side and reflects them back into squad's claim store as touch updates only — never as `done`. Marking complete is still a deliberate `squad done` with evidence.
4. Tears down the mirror on `SIGINT`, `SessionEnd`, or `--once` exit.

The bridge does NOT:
- Sync chat between the two systems.
- Take squad items to `done` from inside agent-teams.
- Persist between agent-teams sessions.

See [recipes/migrating-from-agent-teams.md](../recipes/migrating-from-agent-teams.md) for the migration path when ephemeral stops being enough.

## Anti-patterns

- **Running agent-teams against an unmanaged repo when you've been doing this for weeks.** If you're back in the same codebase repeatedly, you've outgrown agent-teams' lifetime model. Init squad.
- **Initializing squad for a one-off two-hour session.** The init cost is small but real; the chat verbs and hygiene rules don't earn their keep in a single afternoon. Use agent-teams.
- **Trying to force agent-teams' lead role onto squad's peer model.** Squad has no lead. If you need a lead role, run agent-teams *inside* a squad claim.
- **Bidirectional bridge.** Don't write one. The conflict-resolution cost is higher than the convenience. Squad is the source of truth; the bridge is read-mostly.

## See also

- `docs/recipes/migrating-from-agent-teams.md` — step-by-step graduation path.
- `docs/recipes/multi-agent-parallel-claude-sessions.md` — squad-only multi-agent pattern.
- `docs/reference/commands.md#squad-bridge-agent-teams` — bridge command reference.
- Upstream: https://code.claude.com/docs/en/agent-teams.
