# Recipe: Recovering from orphan worktrees

When `--worktree` is in play (or `agent.default_worktree_per_claim: true`), each claim provisions an isolated checkout under `<repoRoot>/.squad/worktrees/<agent>-<item>/`. `squad done` and `squad handoff` clean these up. An agent crash mid-claim, a partial cleanup failure, or a manual `git worktree add` can leave a directory behind.

## Detect

```bash
squad doctor
```

Stranded directories show up as `worktree_orphan` findings:

```
doctor: 1 finding(s):
  - [worktree_orphan] orphan worktree: /path/to/repo/.squad/worktrees/agent-blue-FEAT-123 has no matching active claim
      fix: cd <repo-root> && git worktree remove --force /path/to/repo/.squad/worktrees/agent-blue-FEAT-123
```

## Recover

```bash
cd <repo-root>
git worktree remove --force .squad/worktrees/<agent>-<item>
```

If the directory is gone but git still tracks it:

```bash
git worktree prune
```

If a `squad/<item>-<agent>` branch was left behind with commits you want to keep, push it as a PR; otherwise:

```bash
git branch -D squad/<item>-<agent>
```

## Avoid

- Don't `rm -rf` a worktree without `git worktree remove` first — the registration in `.git/worktrees/` lingers.
- If `squad done` warns "worktree cleanup failed", read the warning before proceeding to the next claim. The most common cause is uncommitted local edits in the worktree.
- A stale claim that didn't clean up is best handled by `squad force-release <ID> --reason "..."`; the orphan finding will surface afterward.
