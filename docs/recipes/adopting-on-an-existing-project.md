# Recipe: Adopting squad on an existing project

## Greenfield vs brownfield

A greenfield repo is easy: `squad init` writes the whole scaffold and you go. A brownfield repo (existing CLAUDE.md, existing PM tracker, existing conventions) is the diff this recipe covers.

## Pre-flight checklist

```bash
# Backup branch — gives you an undo
git checkout -b pre-squad-backup
git checkout main

# Skim what's already there
ls -la                               # any existing .squad/ or AGENTS.md?
cat CLAUDE.md 2>/dev/null            # existing managed blocks?
```

If `.squad/` or `AGENTS.md` already exists, stop and inspect — squad's `init` won't overwrite either, but you want to know what state you're starting from.

## Decide three things before running init

1. **ID prefixes.** Defaults are `BUG, FEAT, TASK, CHORE`. Add anything else your team uses (e.g. `INFRA`, `SEC`).
2. **Plugin yes/no.** If your team uses Claude Code, install. If not, skip the prompt and reuse only the binary.
3. **Existing CLAUDE.md.** If you already have one, squad will inject only its own managed block (delimited with comment markers) and leave the rest alone. Open the file post-init and skim — squad will not touch anything outside its block.

## Run init

Open Claude Code in the project and ask:

> *"Initialize squad here."*

Claude runs `squad init --yes` for you (default ID prefixes, plugin install). If you want to customize anything (project name, prefix list), drop to a terminal and run interactively:

```bash
cd ~/dev/your-existing-project
squad init                          # answers the 3 questions
```

What gets written:

- `.squad/items/` — empty directory plus `.squad/items/EXAMPLE-001-try-the-loop.md` (a tutorial item).
- `.squad/done/` — empty.
- `.squad/config.yaml` — the project-level config you'll tune.
- `AGENTS.md` — generic agent doctrine doc; safe to read and edit.
- `CLAUDE.md` — adds a managed block at the top (or creates the file if absent).

Inspect:

```bash
git status                          # see exactly what changed
git diff CLAUDE.md                  # see the managed block injected
cat .squad/config.yaml              # tune defaults if needed
```

Commit the scaffold:

```bash
git add .squad/ AGENTS.md CLAUDE.md
git commit -m "chore: adopt squad"
```

## Reconciling with an existing tracker

You have three reasonable paths:

1. **Squad authoritative, tracker gone.** File a few items by hand (`squad new feat "..."`) and pivot all new work into squad. Old tracker becomes archive.
2. **Tracker authoritative, squad for active work only.** File items in squad as you start them, mark done when shipped, leave the tracker as the durable backlog. The translation layer is in your head.
3. **Both, dual-recorded.** Heaviest path; only worth it during a transition.

There's no "import from tracker X" command for the OSS trackers. If you genuinely need a bulk import, file an issue — until then, hand-file the 5–10 most active items and let the rest live in the old tracker.

## Reconciling CLAUDE.md

Squad's managed block is delimited:

```markdown
<!-- squad-managed:start -->
... (squad's content; do not edit) ...
<!-- squad-managed:end -->
```

Anything outside those markers is yours. Re-running `squad init` re-renders the managed block; your prose outside is untouched. If `init` ever sees the markers in an unexpected state, it refuses and asks you to manually resolve — better to interrupt than to silently corrupt.

## Day-1 hygiene

In Claude Code: *"Run a squad health check, then claim the top ready item."* Claude calls `squad_status` (or you can run `squad doctor` in a terminal for the full diagnostic) and `squad_next` + `squad_claim`.

The terminal-only equivalent:

```bash
squad doctor                        # should be clean on a fresh install
squad install-hooks                 # opt-in to the optional hooks (six are already on)
squad go                            # register a session-derived agent id and pick up the top ready item
```

For multi-agent work, a teammate does the same on their machine. Then either of you can ask Claude *"who's working on what across our repos?"* (or run `squad workspace who` in a terminal) and you'll see each other.

## When things look weird

`squad doctor` is the first stop. After that:

- `squad workspace list` shows every repo squad has registered. If yours is missing, `cd` into it and run any squad command (init/register/next) — registration is lazy.
- `~/.squad/global.db` is just SQLite; `sqlite3 ~/.squad/global.db ".tables"` and `.schema <table>` will show everything.

## Rollback

If you decide squad isn't for you:

```bash
# Remove the squad scaffold
rm -rf .squad/

# Remove the CLAUDE.md managed block (manually, between the markers)
$EDITOR CLAUDE.md

# If no other repo on this machine uses squad
rm -rf ~/.squad/

# Uninstall the plugin and hooks
squad install-plugin --uninstall
squad install-hooks --uninstall

# Uninstall the binary (go install)
rm $(go env GOPATH)/bin/squad
```

The backup branch from the pre-flight is your safety net — `git checkout pre-squad-backup` gets you to the pre-squad state.
