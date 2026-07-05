-- Migration 003: Add variant key and allocation fields
-- +migrate Up

ALTER TABLE arms ADD COLUMN key VARCHAR(255);
ALTER TABLE arms ADD COLUMN allocation_percent INTEGER DEFAULT 0;

-- Create unique index on key within test scope
CREATE UNIQUE INDEX CONCURRENTLY arms_key_test_unique ON arms(test_id, key) WHERE key IS NOT NULL;

-- Add constraint for allocation percentage (0-100)
ALTER TABLE arms ADD CONSTRAINT arms_allocation_percent_range CHECK (allocation_percent >= 0 AND allocation_percent <= 100);

-- +migrate Down

ALTER TABLE arms DROP CONSTRAINT IF EXISTS arms_allocation_percent_range;
DROP INDEX IF EXISTS arms_key_test_unique;
ALTER TABLE arms DROP COLUMN IF EXISTS allocation_percent;
ALTER TABLE arms DROP COLUMN IF EXISTS key;