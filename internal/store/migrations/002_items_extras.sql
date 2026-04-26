ALTER TABLE items ADD COLUMN epic_id TEXT;
ALTER TABLE items ADD COLUMN parallel INTEGER NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN conflicts_with TEXT NOT NULL DEFAULT '[]';
ALTER TABLE attestations ADD COLUMN review_disagreements INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_items_epic ON items(repo_id, epic_id);
