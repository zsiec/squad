-- Commits made by an agent during a claim, captured at squad-done time so
-- the dashboard can show what shipped without shelling to git log per
-- request. Append-only; (repo_id, sha) PK so re-running done is a no-op.

CREATE TABLE IF NOT EXISTS commits (
  repo_id   TEXT NOT NULL,
  item_id   TEXT NOT NULL,
  sha       TEXT NOT NULL,
  subject   TEXT NOT NULL,
  ts        INTEGER NOT NULL,
  agent_id  TEXT NOT NULL,
  PRIMARY KEY (repo_id, sha)
);

CREATE INDEX IF NOT EXISTS idx_commits_item ON commits(repo_id, item_id);
