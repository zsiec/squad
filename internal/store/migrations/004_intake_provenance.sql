ALTER TABLE items ADD COLUMN captured_by  TEXT;
ALTER TABLE items ADD COLUMN captured_at  INTEGER;
ALTER TABLE items ADD COLUMN accepted_by  TEXT;
ALTER TABLE items ADD COLUMN accepted_at  INTEGER;
ALTER TABLE items ADD COLUMN parent_spec  TEXT;
CREATE INDEX IF NOT EXISTS idx_items_status_capture
  ON items(repo_id, status) WHERE status='captured';
