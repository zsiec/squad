# Global DB schema

`~/.squad/global.db` is SQLite in WAL mode. It holds **operational state only** — claims, chat, file touches, agent registrations, and a denormalized item index. Item content lives in `.squad/items/` and is git-committed; the DB is machine-local and never committed.

The schema is loaded at startup of every DB-opening command. There are no migration files yet — the schema is small enough that the single `internal/store/schema.sql` is replayed idempotently with `CREATE ... IF NOT EXISTS`.

To inspect: `sqlite3 ~/.squad/global.db ".schema"`.

## Tables

### `repos`

Every repo squad has seen on this machine.

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT PRIMARY KEY | repo_id from sha256(remote URL) or path fallback |
| `root_path` | TEXT | absolute path to the repo's root |
| `remote_url` | TEXT | git origin URL when present |
| `name` | TEXT | display name (project_name from config.yaml) |
| `created_at` | INTEGER | unix seconds when first registered |

### `agents`

Registered agents per repo.

| Column | Type | Notes |
|---|---|---|
| `id` | TEXT PRIMARY KEY | agent_id (`agent-XXXX`) |
| `repo_id` | TEXT NOT NULL | FK → repos.id |
| `display_name` | TEXT NOT NULL | human-readable name |
| `worktree` | TEXT | absolute path of the worktree the agent registered from |
| `pid` | INTEGER | OS process id at register time (informational) |
| `started_at` | INTEGER NOT NULL | unix seconds |
| `last_tick_at` | INTEGER NOT NULL | unix seconds; updated by `tick`, chat verbs, claim activity |
| `status` | TEXT NOT NULL | `active` \| `stale` \| `gone` (computed by hygiene sweep) |

Index: `idx_agents_repo ON agents(repo_id)`.

### `claims`

Active claims. One row per item that's currently held by some agent. Released claims move to `claim_history`.

| Column | Type | Notes |
|---|---|---|
| `item_id` | TEXT PRIMARY KEY | the item's ID; PK ensures one active claim per item per repo |
| `repo_id` | TEXT NOT NULL | FK → repos.id |
| `agent_id` | TEXT NOT NULL | FK → agents.id |
| `claimed_at` | INTEGER NOT NULL | unix seconds |
| `last_touch` | INTEGER NOT NULL | unix seconds; advanced by tick / verbs / progress |
| `intent` | TEXT | one-sentence plan from `--intent` |
| `long` | INTEGER NOT NULL DEFAULT 0 | 1 = long-running claim (longer stale threshold applies) |

Index: `idx_claims_repo ON claims(repo_id)`.

### `claim_history`

Append-only log of closed claims.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | |
| `repo_id` | TEXT NOT NULL | |
| `item_id` | TEXT NOT NULL | |
| `agent_id` | TEXT NOT NULL | |
| `claimed_at` | INTEGER NOT NULL | |
| `released_at` | INTEGER NOT NULL | |
| `outcome` | TEXT | `done` \| `released` \| `blocked` \| `force-released` \| `reassigned` |

Index: `idx_claim_history_repo ON claim_history(repo_id)`.

### `messages`

Chat messages: typed verbs, plain say, system notifications.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | |
| `repo_id` | TEXT NOT NULL | |
| `ts` | INTEGER NOT NULL | unix seconds |
| `agent_id` | TEXT NOT NULL | who posted (or `system` for system messages) |
| `thread` | TEXT NOT NULL | item ID, `global`, or other thread label |
| `kind` | TEXT NOT NULL | `say` \| `thinking` \| `milestone` \| `stuck` \| `fyi` \| `ask` \| `answer` \| `knock` \| `done` \| `handoff` \| ... |
| `body` | TEXT | message body |
| `mentions` | TEXT | comma-joined list of `@agent` mentions (for fast lookup) |
| `priority` | TEXT NOT NULL DEFAULT 'normal' | `normal` \| `high` (for knocks) |

Indexes:
- `idx_messages_thread_ts ON messages(thread, ts)` — for tail / history
- `idx_messages_ts ON messages(ts)` — for cross-thread tail
- `idx_messages_repo_ts ON messages(repo_id, ts)` — for workspace queries

### `touches`

Active and historical file-touch declarations.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | |
| `repo_id` | TEXT NOT NULL | |
| `agent_id` | TEXT NOT NULL | |
| `item_id` | TEXT | nullable — touches can exist without a claim |
| `path` | TEXT NOT NULL | repo-relative file path |
| `started_at` | INTEGER NOT NULL | unix seconds |
| `released_at` | INTEGER | NULL while active; set on `untouch` |

Partial indexes (active touches only):
- `idx_touches_path_active ON touches(path) WHERE released_at IS NULL`
- `idx_touches_agent_active ON touches(agent_id) WHERE released_at IS NULL`
- `idx_touches_repo_active ON touches(repo_id) WHERE released_at IS NULL`

### `reads`

Per-agent read cursor for tick. Tracks which messages each agent has already seen on each thread.

| Column | Type | Notes |
|---|---|---|
| `agent_id` | TEXT NOT NULL | |
| `thread` | TEXT NOT NULL | |
| `last_msg_id` | INTEGER NOT NULL | highest message id read on this thread |

PK: `(agent_id, thread)`.

### `progress`

Append-only log of explicit progress reports via `squad progress <ID> <pct 0..100> [--note "..."]`.

| Column | Type | Notes |
|---|---|---|
| `item_id` | TEXT NOT NULL | |
| `pct` | INTEGER NOT NULL | 0–100 |
| `reported_at` | INTEGER NOT NULL | |
| `reported_by` | TEXT NOT NULL | agent_id |
| `note` | TEXT | optional note |

Index: `idx_progress_item_ts ON progress(item_id, reported_at)`.

### `items`

Denormalized index of items present on disk. Refreshed by the hygiene sweep and by `squad next` / `squad status` queries that walk the filesystem and reconcile.

| Column | Type | Notes |
|---|---|---|
| `repo_id` | TEXT NOT NULL | |
| `item_id` | TEXT NOT NULL | |
| `title` | TEXT NOT NULL | |
| `type` | TEXT | `bug` \| `feat` \| `task` \| `chore` \| etc. |
| `priority` | TEXT | `P0`–`P3` |
| `area` | TEXT | freeform area tag |
| `status` | TEXT | `ready` \| `claimed` \| `in-progress` \| `review` \| `blocked` \| `done` |
| `estimate` | TEXT | `30m`, `1h`, etc. |
| `risk` | TEXT | `low` \| `medium` \| `high` |
| `not_before` | TEXT | ISO timestamp; item is gated until then |
| `ac_total` | INTEGER NOT NULL DEFAULT 0 | total AC checkboxes parsed from body |
| `ac_checked` | INTEGER NOT NULL DEFAULT 0 | how many are checked |
| `archived` | INTEGER NOT NULL DEFAULT 0 | 1 = file moved to `.squad/done/` |
| `path` | TEXT NOT NULL | absolute path to the .md file |
| `updated_at` | INTEGER NOT NULL | last sync from disk |
| `epic_id` | TEXT | nullable; name of the owning epic when the item was scaffolded under one |
| `parallel` | INTEGER NOT NULL DEFAULT 0 | 1 = item is safe to run in parallel with its siblings |
| `conflicts_with` | TEXT NOT NULL DEFAULT '[]' | JSON array of item IDs that must not be claimed concurrently |

PK: `(repo_id, item_id)`. Strict mode (`STRICT`).

Indexes:
- `idx_items_repo_status ON items(repo_id, status)`
- `idx_items_epic ON items(repo_id, epic_id)` — for `squad analyze` and epic rollups

### `specs`

One row per spec markdown file under `.squad/specs/`. A spec captures the why-and-what for a body of work: title, motivation, acceptance criteria, non-goals, and integration surface area.

| Column | Type | Notes |
|---|---|---|
| `repo_id` | TEXT NOT NULL | |
| `name` | TEXT NOT NULL | kebab-case slug; matches the filename without extension |
| `title` | TEXT NOT NULL | human-readable title |
| `motivation` | TEXT NOT NULL DEFAULT '' | why this spec exists |
| `acceptance` | TEXT NOT NULL DEFAULT '' | acceptance criteria (rendered list) |
| `non_goals` | TEXT NOT NULL DEFAULT '' | explicit non-goals |
| `integration` | TEXT NOT NULL DEFAULT '' | areas of the codebase this spec touches |
| `path` | TEXT NOT NULL | absolute path to the .md file |
| `updated_at` | INTEGER NOT NULL | last sync from disk |

PK: `(repo_id, name)`. Strict mode (`STRICT`).

### `epics`

One row per epic markdown file under `.squad/epics/`. An epic groups items that deliver a slice of a spec; `squad analyze` reads this table together with `items` to produce a stream decomposition.

| Column | Type | Notes |
|---|---|---|
| `repo_id` | TEXT NOT NULL | |
| `name` | TEXT NOT NULL | kebab-case slug; matches the filename without extension |
| `spec` | TEXT NOT NULL DEFAULT '' | name of the spec this epic belongs to |
| `status` | TEXT NOT NULL DEFAULT 'open' | `open` \| `closed` |
| `parallelism` | TEXT NOT NULL DEFAULT '' | free-form parallelism notes; populated by `squad analyze` |
| `path` | TEXT NOT NULL | absolute path to the .md file |
| `updated_at` | INTEGER NOT NULL | last sync from disk |

PK: `(repo_id, name)`. Strict mode (`STRICT`).

Index: `idx_epics_spec ON epics(repo_id, spec)`.

## Why no migrations directory yet

The schema is small and forward-compatible: every column added so far has been to a new table or a new column with a default. `CREATE TABLE IF NOT EXISTS ... CREATE INDEX IF NOT EXISTS ...` is replayed at every startup, which is idempotent for additions. When a destructive change is needed (column drop, type change), a numbered `internal/store/migrations/` directory will land alongside it.
