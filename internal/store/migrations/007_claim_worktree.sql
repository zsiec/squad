-- Per-claim worktree path: when an agent claims with --worktree, the path
-- of the isolated git worktree is recorded here so done/handoff can tear
-- it down without re-deriving from convention. Default empty preserves
-- existing flow for users who don't opt in.

ALTER TABLE claims ADD COLUMN worktree TEXT NOT NULL DEFAULT '';
