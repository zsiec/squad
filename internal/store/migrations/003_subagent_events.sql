CREATE TABLE IF NOT EXISTS subagent_events (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id       TEXT NOT NULL,
  agent_id      TEXT NOT NULL,
  subagent_id   TEXT NOT NULL,
  subagent_type TEXT,
  event         TEXT NOT NULL,
  ts            INTEGER NOT NULL,
  duration_ms   INTEGER
);
CREATE INDEX IF NOT EXISTS idx_subagent_events_agent_ts ON subagent_events(agent_id, ts);
CREATE INDEX IF NOT EXISTS idx_subagent_events_subagent ON subagent_events(subagent_id);
