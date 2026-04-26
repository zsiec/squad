# Multi-repo

## Why one DB

The global DB at `~/.squad/global.db` holds operational state for **every** squad-managed repo on this machine. That makes cross-repo views one query away: one ready stack across all your projects, one chat history, one claim list. From any repo's working directory:

```bash
squad workspace status            # per-repo summary table
squad workspace next --limit 10   # top P0/P1 across every repo
squad workspace who               # every agent in every repo
squad workspace list              # every known repo with last activity
```

If each repo had its own DB, those queries would be N file opens and a manual merge. With the shared DB, they're one SQL `JOIN`.

## How repo identity works

`repo_id` is computed from the git remote when one is set:

```
repo_id = sha256("<origin remote URL>")[:16]
```

If there's no remote (fresh `git init` with no `git remote add`), squad falls back to a path-derived ID with a discriminator so cloning the same repo twice doesn't collide:

```
repo_id = sha256("<absolute path>")[:16]   # path fallback
```

The path-fallback case is best-effort. If you later add a remote, the ID changes — `squad init` will warn and ask before continuing.

## What stays per-repo

- `.squad/items/<ID>-<slug>.md` — item content, AC, body. Committed to git, travels with the codebase.
- `.squad/done/` — closed items.
- `.squad/config.yaml` — project-level config: ID prefixes, risk paths, verification commands. Committed.
- `.squad/pending-prs.json` — PR ↔ item mappings recorded by `pr-link` (Phase 12). Committed; the GitHub Actions workflow reads it.

## What stays global

- `~/.squad/global.db` — claims, chat messages, file touches, agent registrations, item index. Machine-local, never committed.
- `~/.squad/hooks/` — materialized hook scripts (Phase 11). Machine-local.
- `~/.squad/hygiene.lock` — sweep debounce file. Machine-local.

## Workspace queries

| Command | What it shows |
|---|---|
| `squad workspace status` | One row per known repo: in-progress / ready / blocked counts, last activity timestamp. |
| `squad workspace next --limit N` | Top N P0/P1 ready items across every repo (lower priorities aren't surfaced — drill into a single repo with `squad next` for those). |
| `squad workspace who` | Every registered agent across every repo with current claim and last tick. |
| `squad workspace list` | Every known repo with origin URL and last-seen-at. |
| `squad workspace forget <repo_id>` | Remove a repo from the global DB (e.g. after deleting it locally). Items and config in the repo itself are untouched. |

Filter scope on commands that support `--repo`:

```bash
squad workspace status --repo current      # just this repo
squad workspace status --repo other        # everything except this repo
squad workspace status --repo id1,id2,id3  # explicit list
```

## Gotchas

- **Cloning the same repo twice** on one machine. The path-fallback ID will differ between clones (paths differ), so they're treated as distinct repos in the DB. If both clones have the same git remote, the remote-derived ID is identical and they merge — usually what you want.
- **Same repo on two machines.** Items sync via git like normal. Claims do not — the global DB is per-machine, so registering on machine B does not see machine A's claim. This is by design (cross-machine claim sync is v2).
- **Multiple users on a shared machine.** Each user has their own `~/.squad/global.db` (it's under `$HOME`), so claims and chat are private per OS user. Item files in the repo are shared via git in the usual way.
- **`squad init` in a non-git directory.** Squad warns and asks for a `git init` first. The repo_id needs *something* stable; a directory that may be moved or renamed is not it.

## See also

- [squad-vs-agent-teams.md](squad-vs-agent-teams.md) — when to use squad vs. Claude Code's agent-teams.
