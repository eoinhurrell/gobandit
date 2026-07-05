-- Remove indexes
DROP INDEX IF EXISTS idx_arms_key;
DROP INDEX IF EXISTS idx_tests_status;  
DROP INDEX IF EXISTS idx_tests_key;

-- Remove added columns from arms table
ALTER TABLE arms DROP COLUMN IF EXISTS allocation_percent;
ALTER TABLE arms DROP COLUMN IF EXISTS key;

-- Remove added columns from tests table
ALTER TABLE tests DROP COLUMN IF EXISTS status;
ALTER TABLE tests DROP COLUMN IF EXISTS key;