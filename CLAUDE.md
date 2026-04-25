# CLAUDE.md — squad repo

## For agents starting a session

1. Read this file end-to-end.
2. Read the operating manual: `/Users/zsiec/dev/switchframe/docs/plans/2026-04-24-squad-master-plan.md`. This is the stand-in for squad's own `AGENTS.md` until Phase 6 produces the real one.
3. Read the current phase plan in that same directory (e.g. `2026-04-24-squad-phase-01-store-and-identity.md`). Do not read phases you are not working on.
4. Once Phase 6 lands, run `squad init` in this repo and let the scaffolded `CLAUDE.md` replace this file. This hand-written version is a bootstrap stopgap — delete it without nostalgia.

## Codebase orientation

As of Phase 0 the tree is intentionally sparse. It grows as phases land.

```
cmd/squad/       # binary entry; subcommands are added one per file as phases need them
internal/        # (Phase 1+) store, items, chat, touch, hygiene, repo, identity, server, scaffold, importers
templates/       # (Phase 6) files copied into user repos by `squad init`
plugin/          # (Phase 10) Claude Code plugin (skills, commands, hooks)
web/             # (Phase 8) static SPA for the dashboard
docs/            # (Phase 13) README, concepts, reference, recipes, adopting guide
.github/         # CI workflow (Phase 0), release workflow (Phase 14)
```

`go.mod` pins Go 1.22. CI matrix: ubuntu-latest + macos-latest × Go 1.22. Pure Go, no CGO.

## Conventions (the load-bearing section)

### TDD is the default, not a suggestion

Failing test → minimal implementation → passing test → commit. One TDD micro-loop = one commit. Skip TDD only for: pure refactors with full existing coverage, exploratory spikes that won't merge, one-off scripts. If you skip it, justify the skip in the commit message.

### Pure Go, no CGO

Must build with `CGO_ENABLED=0` for every supported os/arch. Use `modernc.org/sqlite`, never `mattn/go-sqlite3`. The release matrix is `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` — nothing ships if any of them break.

### Commits

Prefixes: `feat:`, `fix:`, `test:`, `docs:`, `perf:`, `refactor:`, `chore:`. **No `Co-Authored-By` lines.** Subject ≤72 chars, lowercase after the prefix, imperative mood. Body explains WHY, not WHAT — the diff shows what changed. Frequent small commits beat one big one.

### No PM traces anywhere in code

No backlog IDs, no ticket tags, no PM vocabulary in identifiers, comments, or commit messages. Squad enforces this against its users — squad has to follow it on itself, doubly. If you need a stable reference, use a file:line, a test name, or a function name, not a ticket.

### Minimum comments, default to none

Delete anything that restates code. Only keep a comment when the WHY is non-obvious — a hidden constraint, a workaround for a specific bug, a subtle invariant that would trip a future reader. Comments describing the current task or callers rot immediately and belong in commit messages or PR descriptions, not in the source.

### No future-work TODO comments

If it is worth doing, file it as an item (once Phase 6 ships). If it is not worth filing, it is not worth a comment. TODOs in the tree are a failure mode — they accumulate forever and nobody reads them.

### No defensive checks for impossible states

Trust internal invariants. Validate only at system boundaries: user input, external APIs, disk, network. A nil-check on something that cannot be nil is noise.

### No premature abstraction

Three similar lines is fine. Wait until you have three real callers before extracting. A wrong abstraction costs more than some duplication.

### Evidence requirement before claiming done

Every "tests pass" / "build green" / "CI green" claim must be backed by the actual output line pasted into the conversation. Bare assertions are worth zero. The author and the reviewer both need to see the green line. This is non-negotiable — a silent pass is indistinguishable from a fabrication.

### Code review before every commit

Every meaningful change goes through `superpowers:code-reviewer` before commit. Even one-line fixes. The cost is about thirty seconds; the cost of a bug reaching a user is hours of their time and yours. Review catches what tests do not.

### Use the right skill before coding

- `superpowers:brainstorming` for unclear features, ambiguous requests, or anything creative.
- `superpowers:systematic-debugging` for bugs, test failures, unexpected behavior.
- `superpowers:test-driven-development` for implementing any feature or bugfix.
- `superpowers:dispatching-parallel-agents` when you have three or more independent sub-problems.
- `superpowers:executing-plans` when working through a phase plan.

Do not rationalize past the skill — if it might apply, invoke it.

### TaskCreate threshold

Use `TaskCreate` when a task is estimated at ≥2h or has ≥3 sub-steps. Skip it on small linear work — the overhead of maintaining the list outweighs the visibility.

### Amend policy

Never `git commit --amend` without explicit approval. One narrow exception: removing PM-trace mistakes from a local unpushed commit before anyone sees it. Otherwise always create a new commit — amending published history is destructive.

### Switchframe-ism scrubbing

The reference implementation at `/Users/zsiec/dev/switchframe/server/cmd/sf-team/` is read-only inspiration. Lift code patterns, then scrub every Switchframe-ism per the master plan checklist: `sf-team` → `squad`, `~/.switchframe-team` → `~/.squad`, `SF_TEAM_SESSION_ID` → `SQUAD_SESSION_ID`, all broadcast vocabulary (on-air, MXL, SCTE, captions, mosaic, server/output/, etc.) removed or replaced with generic placeholders. A grep for `switchframe`, `sf-team`, `sf_team`, or `on-air` against this repo must return zero matches outside of documentation that references the migration explicitly.

## Reading order for new agents

In priority order, stopping when you have enough context:

1. **This CLAUDE.md** — conventions and bootstrap stance.
2. **Master plan** — `/Users/zsiec/dev/switchframe/docs/plans/2026-04-24-squad-master-plan.md`. Cross-cutting conventions, phase index, execution model.
3. **Current phase plan** — `/Users/zsiec/dev/switchframe/docs/plans/2026-04-24-squad-phase-NN-<topic>.md`. Task-by-task instructions with paste-in code.
4. **Design doc** — `/Users/zsiec/dev/switchframe/docs/plans/2026-04-24-squad-extraction-design.md`. Every architectural decision with rationale. Read when you need the WHY behind a phase instruction.

All four documents live in `/Users/zsiec/dev/switchframe/docs/plans/`. They are gitignored in that repo and must not be committed from this repo either — `.gitignore` already excludes `docs/plans/`.

## Updating this file

Update this file only when a phase lands that changes the codebase structure or the conventions above. Otherwise leave it alone. After Phase 6 ships, replace it entirely with the output of `squad init` run in this directory.
