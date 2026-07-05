// Package postgres implements PostgreSQL repositories
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"gobandit/internal/domain/experiment"
)

// TestRepository implements the TestRepository interface using PostgreSQL
type TestRepository struct {
	db *sql.DB
}

// NewTestRepository creates a new PostgreSQL test repository
func NewTestRepository(db *sql.DB) *TestRepository {
	return &TestRepository{db: db}
}

// CreateTest creates a new test with its arms in a transaction
func (r *TestRepository) CreateTest(ctx context.Context, test *experiment.Test) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert test
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tests (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, test.ID, test.Name, test.Description, test.CreatedAt, test.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert test: %w", err)
	}

	// Insert arms
	for _, arm := range test.Arms {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO arms (id, test_id, name, description, successes, failures, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, arm.ID, arm.TestID, arm.Name, arm.Description, arm.Successes, arm.Failures, arm.CreatedAt, arm.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert arm %s: %w", arm.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTest retrieves a test by ID with its arms
func (r *TestRepository) GetTest(ctx context.Context, id string) (*experiment.Test, error) {
	var test experiment.Test
	var createdAt, updatedAt pq.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM tests
		WHERE id = $1
	`, id).Scan(&test.ID, &test.Name, &test.Description, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("test %s not found", id)
		}
		return nil, fmt.Errorf("failed to get test: %w", err)
	}

	// Handle nullable timestamps
	if createdAt.Valid {
		test.CreatedAt = createdAt.Time
	} else {
		test.CreatedAt = time.Now()
	}
	if updatedAt.Valid {
		test.UpdatedAt = updatedAt.Time
	} else {
		test.UpdatedAt = time.Now()
	}

	// Get arms for this test
	arms, err := r.getArmsByTestID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get arms for test %s: %w", id, err)
	}
	test.Arms = arms

	return &test, nil
}

// GetAllTests retrieves all tests
func (r *TestRepository) GetAllTests(ctx context.Context) ([]experiment.Test, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM tests
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tests: %w", err)
	}
	defer rows.Close()

	var tests []experiment.Test
	for rows.Next() {
		var test experiment.Test
		var createdAt, updatedAt pq.NullTime

		err := rows.Scan(&test.ID, &test.Name, &test.Description, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan test: %w", err)
		}

		// Handle nullable timestamps
		if createdAt.Valid {
			test.CreatedAt = createdAt.Time
		} else {
			test.CreatedAt = time.Now()
		}
		if updatedAt.Valid {
			test.UpdatedAt = updatedAt.Time
		} else {
			test.UpdatedAt = time.Now()
		}

		tests = append(tests, test)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate test rows: %w", err)
	}

	return tests, nil
}

// UpdateTest updates an existing test
func (r *TestRepository) UpdateTest(ctx context.Context, test *experiment.Test) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE tests 
		SET name = $2, description = $3, updated_at = $4
		WHERE id = $1
	`, test.ID, test.Name, test.Description, test.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update test: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("test %s not found", test.ID)
	}

	return nil
}

// DeleteTest deletes a test by ID
func (r *TestRepository) DeleteTest(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete arms first (foreign key constraint)
	_, err = tx.ExecContext(ctx, "DELETE FROM arms WHERE test_id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete arms for test %s: %w", id, err)
	}

	// Delete test
	result, err := tx.ExecContext(ctx, "DELETE FROM tests WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete test: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("test %s not found", id)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// getArmsByTestID is a helper method to get arms for a test
func (r *TestRepository) getArmsByTestID(ctx context.Context, testID string) ([]experiment.Arm, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, successes, failures, created_at, updated_at
		FROM arms
		WHERE test_id = $1
		ORDER BY name
	`, testID)
	if err != nil {
		return nil, fmt.Errorf("failed to query arms: %w", err)
	}
	defer rows.Close()

	var arms []experiment.Arm
	for rows.Next() {
		var arm experiment.Arm
		var createdAt, updatedAt pq.NullTime

		arm.TestID = testID
		err := rows.Scan(
			&arm.ID, &arm.Name, &arm.Description,
			&arm.Successes, &arm.Failures,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan arm: %w", err)
		}

		// Handle nullable timestamps
		if createdAt.Valid {
			arm.CreatedAt = createdAt.Time
		} else {
			arm.CreatedAt = time.Now()
		}
		if updatedAt.Valid {
			arm.UpdatedAt = updatedAt.Time
		} else {
			arm.UpdatedAt = time.Now()
		}

		arms = append(arms, arm)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate arm rows: %w", err)
	}

	return arms, nil
}
