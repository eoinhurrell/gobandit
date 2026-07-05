-- Add key and status columns to tests table
ALTER TABLE tests ADD COLUMN key VARCHAR(255) UNIQUE;
ALTER TABLE tests ADD COLUMN status VARCHAR(50) DEFAULT 'active';

-- Add key and allocation_percent columns to arms table  
ALTER TABLE arms ADD COLUMN key VARCHAR(255);
ALTER TABLE arms ADD COLUMN allocation_percent INTEGER DEFAULT 0;

-- Create indexes for performance
CREATE INDEX idx_tests_key ON tests(key) WHERE key IS NOT NULL;
CREATE INDEX idx_tests_status ON tests(status);
CREATE INDEX idx_arms_key ON arms(key) WHERE key IS NOT NULL;