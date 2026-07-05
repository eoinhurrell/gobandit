-- Migration 002: Add experiment key and status fields
-- +migrate Up

ALTER TABLE tests ADD COLUMN key VARCHAR(255);
ALTER TABLE tests ADD COLUMN status VARCHAR(50) DEFAULT 'active';

-- Create unique index on key (allowing NULL for existing records)
CREATE UNIQUE INDEX CONCURRENTLY tests_key_unique ON tests(key) WHERE key IS NOT NULL;

-- +migrate Down

DROP INDEX IF EXISTS tests_key_unique;
ALTER TABLE tests DROP COLUMN IF EXISTS status;
ALTER TABLE tests DROP COLUMN IF EXISTS key;