# CLAUDE.md — squad repo

## For agents starting a session

1. Read this file end-to-end.
2. Read `docs/README.md` and `docs/adopting.md` to understand what the binary does and how a user adopts it.
3. If you're working from a phase plan, read it under `/Users/zsiec/dev/switchframe/docs/plans/` (gitignored locally; not part of this repo). Otherwise the user will provide context inline.

## Codebase orientation

```
cmd/squad/                       # binary entry; one file per cobra subcommand (135+ files, incl. tui.go)
internal/store/                  # SQLite layer; schema.sql + additive migrations
internal/items/                  # item file format, lock, walk, rewrite
internal/claims/                 # atomic claim ledger
internal/chat/                   # typed chat verbs + bus
internal/touch/                  # file-touch tracking
internal/hygiene/                # stale-claim sweeps, doctor checks
internal/identity/               # agent id derivation
internal/repo/                   # repo discovery + global DB path
internal/workspace/              # multi-repo views
internal/scaffold/               # `squad init` templates
internal/attest/                 # evidence ledger (R4)
internal/learning/               # learning artifacts (R5)
internal/notify/                 # notification endpoints
internal/prmark/                 # PR linkage
internal/epics/, internal/specs/ # spec/epic hierarchy (R3)
internal/stats/                  # operational statistics (R7)
internal/listener/               # real-time chat transport (R1)
internal/server/                 # dashboard HTTP + SSE; web SPA at internal/server/web/
internal/mcp/                    # MCP server for Claude Code (R6)
internal/installer/              # plugin install state machine
internal/skills/                 # skill frontmatter parser (test scaffold)
internal/tui/                    # bubbletea TUI (`squad tui`)
internal/tui/client/             # HTTP+SSE client — sole storage-layer access for TUI
internal/tui/views/              # per-view tea.Model implementations (12 views)
internal/tui/components/         # shared table, palette, statusbar
internal/tui/daemon/             # OS-specific service installers (launchd / systemd-user)
internal/tui/theme/              # lipgloss styles
plugin/                          # Claude Code plugin (manifest, hooks, skills, commands) (R6)
templates/github-actions/        # CI templates emitted by `squad init`
docs/                            # README, adopting, concepts, recipes, reference, troubleshooting
.github/workflows/               # CI workflow
```

`go.mod` pins Go 1.25. CI matrix: ubuntu-latest + macos-latest × Go 1.25. Pure Go, no CGO. Release artifacts target `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.

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

If it is worth doing, file it as an item (`squad new bug "..."` or similar). If it is not worth filing, it is not worth a comment. TODOs in the tree are a failure mode — they accumulate forever and nobody reads them.

### No defensive checks for impossible states

Trust internal invariants. Validate only at system boundaries: user input, external APIs, disk, network. A nil-check on something that cannot be nil is noise.

### No premature abstraction

Three similar lines is fine. Wait until you have three real callers before extracting. A wrong abstraction costs more than some duplication.

### TUI import boundary

The TUI's view modules (`internal/tui/views/**`) must NOT import storage-layer packages (`internal/store`, `internal/items`, etc.) — only `internal/tui/client` may speak to the wire. Enforced by `TestImportBoundary` in `internal/tui/architecture_test.go`.

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

Update this file when codebase structure or conventions change materially. The conventions section is load-bearing — every contributor reads it. Keep the codebase orientation in sync with the actual tree.
