-- claims.item_id was declared PRIMARY KEY alone, but item IDs are scoped per
-- repo (every repo's `squad init` seeds EXAMPLE-001). The bare PK caused
-- claims in repo A to silently block claims of the same item_id in repo B,
-- because the INSERT hit the global PK constraint and surfaced as
-- ErrClaimTaken.
--
-- This rescopes the PK to (repo_id, item_id), matching the rest of the
-- repo-scoped tables (items, specs, epics).

ALTER TABLE claims RENAME TO claims_old;

CREATE TABLE claims (
  item_id     TEXT NOT NULL,
  repo_id     TEXT NOT NULL,
  agent_id    TEXT NOT NULL,
  claimed_at  INTEGER NOT NULL,
  last_touch  INTEGER NOT NULL,
  intent      TEXT,
  long        INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (repo_id, item_id)
);

INSERT INTO claims (item_id, repo_id, agent_id, claimed_at, last_touch, intent, long)
SELECT item_id, repo_id, agent_id, claimed_at, last_touch, intent, long FROM claims_old;

DROP TABLE claims_old;

CREATE INDEX IF NOT EXISTS idx_claims_repo ON claims(repo_id);
