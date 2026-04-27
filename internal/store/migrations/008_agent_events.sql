-- No foreign keys: agents may be GC'd before their events, and we don't
-- want cascade deletes wiping the audit trail.

CREATE TABLE IF NOT EXISTS agent_events (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id      TEXT NOT NULL,
  agent_id     TEXT NOT NULL,
  session_id   TEXT NOT NULL DEFAULT '',
  ts           INTEGER NOT NULL,
  event_kind   TEXT NOT NULL,
  tool         TEXT NOT NULL DEFAULT '',
  target       TEXT NOT NULL DEFAULT '',
  exit_code    INTEGER NOT NULL DEFAULT 0,
  duration_ms  INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE INDEX IF NOT EXISTS idx_agent_events_agent_ts ON agent_events(repo_id, agent_id, ts);
CREATE INDEX IF NOT EXISTS idx_agent_events_repo_ts  ON agent_events(repo_id, ts);
