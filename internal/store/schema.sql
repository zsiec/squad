CREATE TABLE IF NOT EXISTS repos (
  id          TEXT PRIMARY KEY,
  root_path   TEXT,
  remote_url  TEXT,
  name        TEXT,
  created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
  id            TEXT PRIMARY KEY,
  repo_id       TEXT NOT NULL,
  display_name  TEXT NOT NULL,
  worktree      TEXT,
  pid           INTEGER,
  started_at    INTEGER NOT NULL,
  last_tick_at  INTEGER NOT NULL,
  status        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agents_repo ON agents(repo_id);

CREATE TABLE IF NOT EXISTS claims (
  item_id     TEXT PRIMARY KEY,
  repo_id     TEXT NOT NULL,
  agent_id    TEXT NOT NULL,
  claimed_at  INTEGER NOT NULL,
  last_touch  INTEGER NOT NULL,
  intent      TEXT,
  long        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_claims_repo ON claims(repo_id);

CREATE TABLE IF NOT EXISTS claim_history (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id      TEXT NOT NULL,
  item_id      TEXT NOT NULL,
  agent_id     TEXT NOT NULL,
  claimed_at   INTEGER NOT NULL,
  released_at  INTEGER NOT NULL,
  outcome      TEXT
);
CREATE INDEX IF NOT EXISTS idx_claim_history_repo ON claim_history(repo_id);

CREATE TABLE IF NOT EXISTS messages (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id   TEXT NOT NULL,
  ts        INTEGER NOT NULL,
  agent_id  TEXT NOT NULL,
  thread    TEXT NOT NULL,
  kind      TEXT NOT NULL,
  body      TEXT,
  mentions  TEXT,
  priority  TEXT NOT NULL DEFAULT 'normal'
);
CREATE INDEX IF NOT EXISTS idx_messages_thread_ts ON messages(thread, ts);
CREATE INDEX IF NOT EXISTS idx_messages_ts ON messages(ts);
CREATE INDEX IF NOT EXISTS idx_messages_repo_ts ON messages(repo_id, ts);

CREATE TABLE IF NOT EXISTS touches (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id      TEXT NOT NULL,
  agent_id     TEXT NOT NULL,
  item_id      TEXT,
  path         TEXT NOT NULL,
  started_at   INTEGER NOT NULL,
  released_at  INTEGER
);
CREATE INDEX IF NOT EXISTS idx_touches_path_active ON touches(path) WHERE released_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_touches_agent_active ON touches(agent_id) WHERE released_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_touches_repo_active ON touches(repo_id) WHERE released_at IS NULL;

CREATE TABLE IF NOT EXISTS reads (
  agent_id     TEXT NOT NULL,
  thread       TEXT NOT NULL,
  last_msg_id  INTEGER NOT NULL,
  PRIMARY KEY (agent_id, thread)
);

CREATE TABLE IF NOT EXISTS progress (
  item_id      TEXT NOT NULL,
  pct          INTEGER NOT NULL,
  reported_at  INTEGER NOT NULL,
  reported_by  TEXT NOT NULL,
  note         TEXT
);
CREATE INDEX IF NOT EXISTS idx_progress_item_ts ON progress(item_id, reported_at);
