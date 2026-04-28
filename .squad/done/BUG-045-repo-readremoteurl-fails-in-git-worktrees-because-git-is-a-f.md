---
id: BUG-045
title: repo.ReadRemoteURL fails in git worktrees because .git is a file, not a directory
type: bug
priority: P2
area: internal/repo
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777332506
accepted_by: agent-afcd
accepted_at: 1777333467
references: []
relates-to: []
blocked-by: []
---

## Problem

`repo.ReadRemoteURL` (`internal/repo/repo.go:54`) does
`os.ReadFile(filepath.Join(rootPath, ".git", "config"))` to extract
the origin URL. In a git worktree, `<rootPath>/.git` is not a
directory but a regular file containing a single `gitdir: <path>`
pointer to the real git dir. The `os.ReadFile` call therefore
returns an error like `open .../.git/config: not a directory`.

`Discover` wraps this and returns a non-nil error with that text;
the loopback Stop hook (`~/.squad/hooks/stop_listen.sh`, which
invokes `squad listen`) prints the message every Stop cycle when
the agent's working directory is inside a `.squad/worktrees/<...>`
checkout. The agent keeps functioning (claims, chat, etc. still
work because most paths fall back to other state) but the noise
clutters every transcript.

## Context

Reproduce on this repo:

```
$ cd /Users/zsiec/dev/squad
$ squad claim CHORE-XXX --intent "any captured item with worktree"
$ cd /Users/zsiec/dev/squad/.squad/worktrees/agent-XXX-CHORE-XXX
$ ls -la .git
.git: ASCII text          # not a directory
$ cat .git
gitdir: /Users/zsiec/dev/squad/.git/worktrees/agent-XXX-CHORE-XXX
$ # Stop hook fires →
read git config: open .../.squad/worktrees/agent-XXX-CHORE-XXX/.git/config: not a directory
```

Surfaced after CHORE-XXX (config-default landing) flipped
`agent.default_worktree_per_claim: true` for this repo, so every
claim now produces a worktree and any agent that `cd`s into it
trips the bug on the next Stop hook.

## Acceptance criteria

- [ ] `repo.ReadRemoteURL` (or whatever helper Discover uses to
      resolve the origin URL) handles the worktree case: when
      `<rootPath>/.git` is a regular file containing
      `gitdir: <path>`, follow the pointer to read the actual
      `config` file under the real git dir.
- [ ] When a session's CWD is inside `.squad/worktrees/<...>`,
      `squad listen` (and any other Discover caller) does not log
      `read git config: ... not a directory`.
- [ ] A unit test in `internal/repo/` covers both shapes —
      regular `.git` directory and `.git`-as-file gitdir pointer
      — and asserts the same origin URL comes back from both.
- [ ] (Optional, if the cleanup is small) prefer
      `git config --get remote.origin.url` shelled to `git`, which
      handles all worktree shapes natively without a custom parser.

## Notes

The error text comes from
`internal/repo/repo.go:59`:
```go
return "", fmt.Errorf("read git config: %w", err)
```

Two viable fixes:

1. **Read the gitdir pointer.** When `os.Stat(rootPath/.git)`
   reports a regular file, parse `gitdir: <path>` from its
   contents and read `<gitdir>/config` (or, more correctly,
   `<gitdir>/../../config` since worktree config typically lives
   in the main git dir, not the worktree's).
2. **Shell to git.** Replace the manual `os.ReadFile` with
   `exec.Command("git", "-C", rootPath, "config", "--get",
   "remote.origin.url")`. Git already understands worktrees and
   bare clones and submodules; we don't need a parallel
   implementation.

Option 2 is the smaller change and removes the parseOriginURL
helper as a side benefit. Option 1 keeps squad shell-out-free for
this specific path (matches the rest of `internal/repo`).
