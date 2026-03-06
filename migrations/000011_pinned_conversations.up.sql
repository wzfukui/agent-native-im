-- Add pinned_at column to participants for per-user conversation pinning
ALTER TABLE participants ADD COLUMN pinned_at TIMESTAMPTZ;
