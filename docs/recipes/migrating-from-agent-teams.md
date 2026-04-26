# Migrating from agent-teams to squad

You started with [Claude Code agent-teams](https://code.claude.com/docs/en/agent-teams) for a quick collaboration burst, the work expanded, and now you're hitting agent-teams' edges: state vanishing on session end, no cross-host story, no way to resume tomorrow's session where today left off. That's the signal you've outgrown agent-teams' lifetime model. This recipe walks you through a clean migration.

If you're not sure whether you've actually outgrown agent-teams, read [concepts/squad-vs-agent-teams.md](../concepts/squad-vs-agent-teams.md) first.

## Signals you've outgrown agent-teams

You probably need to migrate when **two or more** of these are true:

- You've started the same agent-teams team three or more days in a row in the same repo.
- You've lost mid-flight coordination state because a session ended and you couldn't resume.
- You've wanted to run the same coordination on a second machine.
- You've wanted a record of which agent verified which finding.
- You've wanted a "stale claim" or "blocked" notion that agent-teams doesn't have.
- You've started copy-pasting tasks from agent-teams notes into a separate file to keep them around.

## Pre-flight: snapshot your current agent-teams state

Before you exit the agent-teams session, capture what's in flight. Agent-teams stores tasks under `~/.claude/tasks/<team>/`. While the session is still live:

```bash
# Identify your active team's directory.
ls ~/.claude/tasks/

# Snapshot the task list (read-only, while the session is up).
TEAM=<your-team-name>
cp -r ~/.claude/tasks/$TEAM ~/agent-teams-snapshot-$(date +%Y%m%d)/
```

Why this matters: when the session ends, agent-teams' coordination state is gone. Squad won't try to revive it; you migrate the work, not the coordination. The snapshot is for your own reference during the migration.

## Step 1 — Initialize squad in the repo

```bash
cd ~/dev/your-project
squad init --yes      # accepts defaults, no prompts
```

This creates `.squad/items/`, `.squad/done/`, and `.squad/config.yaml`. Commit these — squad's per-repo state belongs in git.

## Step 2 — Register yourself

```bash
squad register --as agent-you --name "Your Name"
```

If you're going to drive squad from multiple machines, run `squad register` on each machine. The DB is per-machine; the agent identity is what unifies your work in the durable record.

## Step 3 — Translate in-flight agent-teams tasks into squad items

For each meaningful task in your snapshot, create a squad item. There is no automated importer because agent-teams tasks have no fixed schema beyond what each lead chose to put in them — but the translation is mechanical:

```bash
# For each non-trivial in-flight task in the snapshot:
squad new feat "short title from the agent-teams task"
# Edit .squad/items/FEAT-NNN-*.md to paste in the AC, context, and any
# verification notes that were in the agent-teams task description.
```

Trivial tasks (one-line, already half-done, "look at this file") aren't worth porting. The migration target is **work that will continue across sessions**, not every micro-step.

If you do this often, write yourself a tiny shell function. Squad will not ship one because the agent-teams task shape isn't stable enough to import against without breaking on the next upstream change.

## Step 4 — Re-claim what you were working on

Pick the item you were actively driving when the session ended:

```bash
squad next
squad claim FEAT-001 --intent "resuming after migrating from agent-teams"
```

Now you're in squad's coordination model. The chat verbs (`squad ask`, `squad fyi`, `squad milestone`) and the claim ledger replace the in-session mailbox you had under agent-teams.

## Step 5 — Re-attach helpers (optional)

If you had a primary + reviewer or primary + helper pair under agent-teams, the squad equivalent is two registered agents working on related items, coordinating via squad chat:

```bash
# Helper, on their own machine or terminal:
squad register --as agent-helper --name "Helper"
squad next                                      # they pick a related item
squad claim FEAT-002 --intent "helping FEAT-001"
squad fyi @agent-you "I'm on FEAT-002, will ping when ready for review"
```

You'll feel the difference immediately: agent-teams' SendMessage was synchronous within a session; squad chat is durable, async, and you can hand off the conversation to a fresh session tomorrow.

## Step 6 — Verify with the evidence ledger (R4)

Once R4 has landed, every `squad done` can be backed by `squad attest` evidence. This is the durability dividend you didn't have under agent-teams:

```bash
# Run your tests, capture the result.
squad attest --item FEAT-001 --kind test --command "go test ./..."

# Run your reviewer (or self-review with disprove-before-report skill).
squad attest --item FEAT-001 --kind review --reviewer-agent agent-helper

# Close out.
squad done FEAT-001 --summary "..."
```

If the item's frontmatter has `evidence_required: [test, review]`, `squad done` will refuse to close without both attestations. This is the explicit replacement for "the lead said it's done."

## Step 7 — Tear down the agent-teams snapshot

Once everything material has been ported into squad items and you've verified the next session resumes cleanly:

```bash
rm -rf ~/agent-teams-snapshot-YYYYMMDD/
```

The snapshot was a safety net. Squad is now the source of truth.

## Step 8 — Commit your new squad state

```bash
git add .squad/
git commit -m "chore: adopt squad for ongoing project coordination"
```

The items are in git. Anyone else who clones the repo and runs `squad register` joins the same coordination space.

## What you keep, what you lose

**Kept across the migration:**
- All committed code. Both tools coordinate work *on* the repo; the repo doesn't care.
- Your mental model of "primary + helper / reviewer." Squad expresses it as two agents on two items.
- Per-task verification habits. R4's evidence ledger formalizes them.

**Lost across the migration:**
- The exact mailbox transcript from inside the agent-teams session. If a specific message mattered, hand-port it into a squad chat verb.
- The "lead" role. Squad has no lead; if you need one, run agent-teams *inside* a squad claim for that one piece of work (see the [composition pattern](../concepts/squad-vs-agent-teams.md#composing-the-two)).
- Single-machine simplicity. Squad's per-machine DB is one more thing to think about. The cross-host benefit pays for it.

## Going the other way (squad → agent-teams)

You can. It's just rare. If you have a squad-managed repo and want to spin up an ephemeral agent-teams session inside one squad claim, that's the [composition pattern](../concepts/squad-vs-agent-teams.md#composing-the-two) — not a migration. Don't tear down squad to get to agent-teams. Compose them.

If you genuinely want to abandon squad for agent-teams (for example, you're handing the project off to someone who wants ephemeral coordination), commit your `.squad/items/` and tell them to read the items as a pre-existing backlog. Tear down `~/.squad/global.db` only if you're done with squad on that machine entirely.

## Troubleshooting

**`squad register` says I'm already registered on this machine.**
You ran it before. `squad whoami` shows your identity; pass `--as <new-id> --name <new-name>` if you want a different one.

**Two items got the same FEAT-NNN.**
You initialized squad twice in nested repos. Run `squad doctor` to find which one. Keep the outer one; tear down the inner.

**Migrated items say `status: filed` but I never claimed them.**
That's the default. `squad next` shows them; `squad claim` picks them up.

**I want the agent-teams `TeammateIdle` hook behavior in squad.**
Squad's R1 plugin uses the `Stop` hook for the same effect. After R6, `squad install_plugin` wires it up. See [reference/hooks.md](../reference/hooks.md).

## See also

- [concepts/squad-vs-agent-teams.md](../concepts/squad-vs-agent-teams.md) — the decision matrix.
- [recipes/multi-agent-parallel-claude-sessions.md](multi-agent-parallel-claude-sessions.md) — squad-only multi-agent pattern.
- [recipes/adopting-on-an-existing-project.md](adopting-on-an-existing-project.md) — adopting squad on a repo with no prior coordination tool.
