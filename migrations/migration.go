package migrations

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Migration represents a database migration
type Migration struct {
	ID      int
	Name    string
	UpSQL   string
	DownSQL string
}

// Runner handles database migrations
type Runner struct {
	db *sql.DB
}

// NewRunner creates a new migration runner
func NewRunner(db *sql.DB) *Runner {
	return &Runner{db: db}
}

// CreateMigrationsTable creates the schema_migrations table
func (r *Runner) CreateMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`
	_, err := r.db.Exec(query)
	return err
}

// GetAppliedMigrations returns list of applied migration versions
func (r *Runner) GetAppliedMigrations() ([]int, error) {
	query := `SELECT version FROM schema_migrations ORDER BY version`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

// ApplyMigration applies a single migration
func (r *Runner) ApplyMigration(migration Migration) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the migration SQL
	_, err = tx.Exec(migration.UpSQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration %d: %w", migration.ID, err)
	}

	// Record the migration as applied
	_, err = tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, migration.ID)
	if err != nil {
		return fmt.Errorf("failed to record migration %d: %w", migration.ID, err)
	}

	return tx.Commit()
}

// RollbackMigration rolls back a single migration
func (r *Runner) RollbackMigration(migration Migration) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the rollback SQL
	_, err = tx.Exec(migration.DownSQL)
	if err != nil {
		return fmt.Errorf("failed to rollback migration %d: %w", migration.ID, err)
	}

	// Remove the migration record
	_, err = tx.Exec(`DELETE FROM schema_migrations WHERE version = $1`, migration.ID)
	if err != nil {
		return fmt.Errorf("failed to remove migration record %d: %w", migration.ID, err)
	}

	return tx.Commit()
}

// ParseMigrationFile parses a migration file and extracts up/down SQL
func ParseMigrationFile(filename, content string) (Migration, error) {
	// Extract ID from filename (e.g., "001_initial_schema.sql" -> 1)
	parts := strings.Split(filename, "_")
	if len(parts) < 2 {
		return Migration{}, fmt.Errorf("invalid migration filename format: %s", filename)
	}

	id, err := strconv.Atoi(strings.TrimSuffix(parts[0], ".sql"))
	if err != nil {
		return Migration{}, fmt.Errorf("invalid migration ID in filename %s: %w", filename, err)
	}

	// Extract name from filename
	name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".sql")

	// Split content into up and down migrations
	upMarker := "-- +migrate Up"
	downMarker := "-- +migrate Down"

	upIndex := strings.Index(content, upMarker)
	downIndex := strings.Index(content, downMarker)

	if upIndex == -1 {
		return Migration{}, fmt.Errorf("missing '-- +migrate Up' marker in %s", filename)
	}

	var upSQL, downSQL string

	if downIndex == -1 {
		// No down migration
		upSQL = strings.TrimSpace(content[upIndex+len(upMarker):])
	} else {
		upSQL = strings.TrimSpace(content[upIndex+len(upMarker) : downIndex])
		downSQL = strings.TrimSpace(content[downIndex+len(downMarker):])
	}

	return Migration{
		ID:      id,
		Name:    name,
		UpSQL:   upSQL,
		DownSQL: downSQL,
	}, nil
}

// MigrateUp applies all pending migrations
func (r *Runner) MigrateUp(migrations []Migration) error {
	if err := r.CreateMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	applied, err := r.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedSet := make(map[int]bool)
	for _, version := range applied {
		appliedSet[version] = true
	}

	// Sort migrations by ID
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	// Apply pending migrations
	for _, migration := range migrations {
		if !appliedSet[migration.ID] {
			fmt.Printf("Applying migration %d: %s\n", migration.ID, migration.Name)
			if err := r.ApplyMigration(migration); err != nil {
				return err
			}
		}
	}

	return nil
}

// MigrateDown rolls back the most recent migration
func (r *Runner) MigrateDown(migrations []Migration) error {
	applied, err := r.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(applied) == 0 {
		fmt.Println("No migrations to rollback")
		return nil
	}

	// Find the most recent migration to rollback
	latestVersion := applied[len(applied)-1]

	var targetMigration *Migration
	for _, migration := range migrations {
		if migration.ID == latestVersion {
			targetMigration = &migration
			break
		}
	}

	if targetMigration == nil {
		return fmt.Errorf("migration %d not found in migration files", latestVersion)
	}

	if targetMigration.DownSQL == "" {
		return fmt.Errorf("migration %d has no down migration", latestVersion)
	}

	fmt.Printf("Rolling back migration %d: %s\n", targetMigration.ID, targetMigration.Name)
	return r.RollbackMigration(*targetMigration)
}
