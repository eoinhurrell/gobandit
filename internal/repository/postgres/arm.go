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

// ArmRepository implements the ArmRepository interface using PostgreSQL
type ArmRepository struct {
	db *sql.DB
}

// NewArmRepository creates a new PostgreSQL arm repository
func NewArmRepository(db *sql.DB) *ArmRepository {
	return &ArmRepository{db: db}
}

// GetArm retrieves an arm by ID
func (r *ArmRepository) GetArm(ctx context.Context, id string) (*experiment.Arm, error) {
	var arm experiment.Arm
	var createdAt, updatedAt pq.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, test_id, name, description, successes, failures, created_at, updated_at
		FROM arms
		WHERE id = $1
	`, id).Scan(
		&arm.ID, &arm.TestID, &arm.Name, &arm.Description,
		&arm.Successes, &arm.Failures, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("arm %s not found", id)
		}
		return nil, fmt.Errorf("failed to get arm: %w", err)
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

	return &arm, nil
}

// GetArmsByTestID retrieves all arms for a given test
func (r *ArmRepository) GetArmsByTestID(ctx context.Context, testID string) ([]experiment.Arm, error) {
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

// UpdateArmStats updates the success/failure counts for an arm
func (r *ArmRepository) UpdateArmStats(ctx context.Context, armID string, success bool) (*experiment.Arm, error) {
	query := `
		UPDATE arms
		SET 
			successes = CASE WHEN $1 THEN successes + 1 ELSE successes END,
			failures = CASE WHEN $1 THEN failures ELSE failures + 1 END,
			updated_at = NOW()
		WHERE id = $2
		RETURNING id, test_id, name, description, successes, failures, created_at, updated_at
	`

	var arm experiment.Arm
	var createdAt, updatedAt pq.NullTime

	err := r.db.QueryRowContext(ctx, query, success, armID).Scan(
		&arm.ID, &arm.TestID, &arm.Name, &arm.Description,
		&arm.Successes, &arm.Failures, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("arm %s not found", armID)
		}
		return nil, fmt.Errorf("failed to update arm stats: %w", err)
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

	return &arm, nil
}

// CreateArm creates a new arm
func (r *ArmRepository) CreateArm(ctx context.Context, arm *experiment.Arm) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO arms (id, test_id, name, description, successes, failures, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, arm.ID, arm.TestID, arm.Name, arm.Description, arm.Successes, arm.Failures, arm.CreatedAt, arm.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create arm: %w", err)
	}

	return nil
}

// UpdateArm updates an existing arm
func (r *ArmRepository) UpdateArm(ctx context.Context, arm *experiment.Arm) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE arms 
		SET name = $3, description = $4, updated_at = $5
		WHERE id = $1 AND test_id = $2
	`, arm.ID, arm.TestID, arm.Name, arm.Description, arm.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update arm: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("arm %s not found", arm.ID)
	}

	return nil
}

// DeleteArm deletes an arm by ID
func (r *ArmRepository) DeleteArm(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM arms WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete arm: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("arm %s not found", id)
	}

	return nil
}
