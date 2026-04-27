CREATE TABLE IF NOT EXISTS intake_sessions (
  id              TEXT PRIMARY KEY,
  repo_id         TEXT NOT NULL,
  agent_id        TEXT NOT NULL,
  mode            TEXT NOT NULL,
  refine_item_id  TEXT,
  idea_seed       TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'open',
  shape           TEXT,
  bundle_json     TEXT,
  created_at      INTEGER NOT NULL,
  updated_at      INTEGER NOT NULL,
  committed_at    INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_intake_sessions_open
  ON intake_sessions(repo_id, agent_id) WHERE status='open';

CREATE TABLE IF NOT EXISTS intake_turns (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id    TEXT NOT NULL REFERENCES intake_sessions(id),
  seq           INTEGER NOT NULL,
  role          TEXT NOT NULL,
  content       TEXT NOT NULL,
  fields_filled TEXT,
  created_at    INTEGER NOT NULL,
  UNIQUE(session_id, seq)
);

ALTER TABLE items ADD COLUMN intake_session_id TEXT;
