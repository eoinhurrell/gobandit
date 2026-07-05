package migrations

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigration_AddKeyAndStatusFields_Up tests adding key and status fields to tests table
func TestMigration_AddKeyAndStatusFields_Up(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	runner := NewRunner(db)

	// Create initial schema
	initialMigration := Migration{
		ID:   1,
		Name: "initial_schema",
		UpSQL: `
			CREATE TABLE tests (
				id VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				description TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP NOT NULL DEFAULT NOW()
			);
		`,
		DownSQL: "DROP TABLE tests;",
	}

	err := runner.ApplyMigration(initialMigration)
	require.NoError(t, err)

	// Apply migration to add new fields
	migration := Migration{
		ID:   2,
		Name: "add_experiment_fields",
		UpSQL: `
			ALTER TABLE tests ADD COLUMN key VARCHAR(255);
			ALTER TABLE tests ADD COLUMN status VARCHAR(50) DEFAULT 'active';
			CREATE UNIQUE INDEX tests_key_unique ON tests(key) WHERE key IS NOT NULL;
		`,
		DownSQL: `
			DROP INDEX IF EXISTS tests_key_unique;
			ALTER TABLE tests DROP COLUMN IF EXISTS status;
			ALTER TABLE tests DROP COLUMN IF EXISTS key;
		`,
	}

	err = runner.ApplyMigration(migration)
	require.NoError(t, err)

	// Verify columns exist
	var columnCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM information_schema.columns 
		WHERE table_name = 'tests' AND column_name IN ('key', 'status')
	`).Scan(&columnCount)
	require.NoError(t, err)
	assert.Equal(t, 2, columnCount, "Both key and status columns should exist")

	// Verify index exists
	var indexCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE tablename = 'tests' AND indexname = 'tests_key_unique'
	`).Scan(&indexCount)
	require.NoError(t, err)
	assert.Equal(t, 1, indexCount, "Unique index on key should exist")
}

// TestMigration_AddKeyAndStatusFields_Down tests rollback of key and status fields
func TestMigration_AddKeyAndStatusFields_Down(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	runner := NewRunner(db)

	// Apply up migration first
	migration := Migration{
		ID:   2,
		Name: "add_experiment_fields",
		UpSQL: `
			CREATE TABLE tests (
				id VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				description TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP NOT NULL DEFAULT NOW()
			);
			ALTER TABLE tests ADD COLUMN key VARCHAR(255);
			ALTER TABLE tests ADD COLUMN status VARCHAR(50) DEFAULT 'active';
			CREATE UNIQUE INDEX tests_key_unique ON tests(key) WHERE key IS NOT NULL;
		`,
		DownSQL: `
			DROP INDEX IF EXISTS tests_key_unique;
			ALTER TABLE tests DROP COLUMN IF EXISTS status;
			ALTER TABLE tests DROP COLUMN IF EXISTS key;
		`,
	}

	err := runner.ApplyMigration(migration)
	require.NoError(t, err)

	// Test rollback
	err = runner.RollbackMigration(migration)
	require.NoError(t, err)

	// Verify columns are gone
	var columnCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM information_schema.columns 
		WHERE table_name = 'tests' AND column_name IN ('key', 'status')
	`).Scan(&columnCount)
	require.NoError(t, err)
	assert.Equal(t, 0, columnCount, "key and status columns should be removed")
}

// TestMigration_AddKeyAndAllocationPercent_Up tests adding variant fields
func TestMigration_AddKeyAndAllocationPercent_Up(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	runner := NewRunner(db)

	// Create initial schema with both tables
	initialMigration := Migration{
		ID:   1,
		Name: "initial_schema",
		UpSQL: `
			CREATE TABLE tests (
				id VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				description TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP NOT NULL DEFAULT NOW()
			);
			CREATE TABLE arms (
				id VARCHAR(255) PRIMARY KEY,
				test_id VARCHAR(255) NOT NULL REFERENCES tests(id),
				name VARCHAR(255) NOT NULL,
				description TEXT,
				successes INTEGER NOT NULL DEFAULT 0,
				failures INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP NOT NULL DEFAULT NOW()
			);
		`,
		DownSQL: `
			DROP TABLE arms;
			DROP TABLE tests;
		`,
	}

	err := runner.ApplyMigration(initialMigration)
	require.NoError(t, err)

	// Apply migration to add variant fields
	migration := Migration{
		ID:   3,
		Name: "add_variant_fields",
		UpSQL: `
			ALTER TABLE arms ADD COLUMN key VARCHAR(255);
			ALTER TABLE arms ADD COLUMN allocation_percent INTEGER DEFAULT 0;
			CREATE UNIQUE INDEX arms_key_test_unique ON arms(test_id, key) WHERE key IS NOT NULL;
			ALTER TABLE arms ADD CONSTRAINT arms_allocation_percent_range CHECK (allocation_percent >= 0 AND allocation_percent <= 100);
		`,
		DownSQL: `
			ALTER TABLE arms DROP CONSTRAINT IF EXISTS arms_allocation_percent_range;
			DROP INDEX IF EXISTS arms_key_test_unique;
			ALTER TABLE arms DROP COLUMN IF EXISTS allocation_percent;
			ALTER TABLE arms DROP COLUMN IF EXISTS key;
		`,
	}

	err = runner.ApplyMigration(migration)
	require.NoError(t, err)

	// Verify columns exist
	var columnCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM information_schema.columns 
		WHERE table_name = 'arms' AND column_name IN ('key', 'allocation_percent')
	`).Scan(&columnCount)
	require.NoError(t, err)
	assert.Equal(t, 2, columnCount, "Both key and allocation_percent columns should exist")

	// Verify constraint exists
	var constraintCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.table_constraints
		WHERE table_name = 'arms' AND constraint_name = 'arms_allocation_percent_range'
	`).Scan(&constraintCount)
	require.NoError(t, err)
	assert.Equal(t, 1, constraintCount, "Allocation percentage constraint should exist")
}

// TestMigration_DataIntegrity_AfterMigration tests existing data remains intact
func TestMigration_DataIntegrity_AfterMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create initial schema and insert test data
	_, err := db.Exec(`
		CREATE TABLE tests (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		CREATE TABLE arms (
			id VARCHAR(255) PRIMARY KEY,
			test_id VARCHAR(255) NOT NULL REFERENCES tests(id),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			successes INTEGER NOT NULL DEFAULT 0,
			failures INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
	`)
	require.NoError(t, err)

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO tests (id, name, description) VALUES ('test1', 'Test 1', 'Description 1');
		INSERT INTO arms (id, test_id, name, successes, failures) VALUES ('arm1', 'test1', 'Arm 1', 10, 5);
	`)
	require.NoError(t, err)

	// Apply migrations
	runner := NewRunner(db)

	experimentMigration := Migration{
		ID:   2,
		Name: "add_experiment_fields",
		UpSQL: `
			ALTER TABLE tests ADD COLUMN key VARCHAR(255);
			ALTER TABLE tests ADD COLUMN status VARCHAR(50) DEFAULT 'active';
		`,
		DownSQL: "",
	}

	variantMigration := Migration{
		ID:   3,
		Name: "add_variant_fields",
		UpSQL: `
			ALTER TABLE arms ADD COLUMN key VARCHAR(255);
			ALTER TABLE arms ADD COLUMN allocation_percent INTEGER DEFAULT 0;
		`,
		DownSQL: "",
	}

	err = runner.ApplyMigration(experimentMigration)
	require.NoError(t, err)

	err = runner.ApplyMigration(variantMigration)
	require.NoError(t, err)

	// Verify existing data is intact with new default values
	var name, status string
	var successes, failures, allocationPercent int
	err = db.QueryRow(`
		SELECT t.name, t.status, a.successes, a.failures, a.allocation_percent
		FROM tests t
		JOIN arms a ON t.id = a.test_id
		WHERE t.id = 'test1'
	`).Scan(&name, &status, &successes, &failures, &allocationPercent)

	require.NoError(t, err)
	assert.Equal(t, "Test 1", name, "Original test name should be preserved")
	assert.Equal(t, "active", status, "Default status should be applied")
	assert.Equal(t, 10, successes, "Original successes should be preserved")
	assert.Equal(t, 5, failures, "Original failures should be preserved")
	assert.Equal(t, 0, allocationPercent, "Default allocation percent should be applied")
}

// setupTestDB creates a test database connection, skipping the test when
// Postgres is unavailable so `go test ./...` stays green without Docker.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Skipf("skipping migration tests: postgres unreachable at %q: %v", dsn, err)
	}

	// Clean up any existing test tables
	db.Exec("DROP TABLE IF EXISTS arms")
	db.Exec("DROP TABLE IF EXISTS tests")
	db.Exec("DROP TABLE IF EXISTS schema_migrations")

	return db
}
