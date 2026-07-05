package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"gobandit/internal/domain/experiment"
)

// ExperimentRepository implements the ExperimentRepository interface using PostgreSQL
type ExperimentRepository struct {
	db     *sql.DB
	keyGen experiment.KeyGenerator
}

// NewExperimentRepository creates a new PostgreSQL experiment repository
func NewExperimentRepository(db *sql.DB, keyGen experiment.KeyGenerator) *ExperimentRepository {
	return &ExperimentRepository{
		db:     db,
		keyGen: keyGen,
	}
}

// CreateExperiment creates a new experiment with its variants in a transaction
func (r *ExperimentRepository) CreateExperiment(ctx context.Context, exp *experiment.Experiment) error {
	// Generate key if not provided
	if exp.Key == "" {
		exp.Key = r.keyGen.GenerateExperimentKey(exp.Name)
	}

	// Validate key
	if err := r.keyGen.ValidateKey(exp.Key); err != nil {
		return fmt.Errorf("invalid experiment key: %w", err)
	}

	// Check for key conflicts and resolve if necessary
	exists, err := r.KeyExists(ctx, exp.Key)
	if err != nil {
		return fmt.Errorf("failed to check key existence: %w", err)
	}

	if exists {
		// Resolve conflict by trying alternatives
		for attempt := 1; attempt <= 10; attempt++ {
			resolver := experiment.NewConflictResolver(r.keyGen)
			alternativeKey := resolver.ResolveExperimentKeyConflict(exp.Key, attempt)
			exists, err := r.KeyExists(ctx, alternativeKey)
			if err != nil {
				return fmt.Errorf("failed to check alternative key existence: %w", err)
			}
			if !exists {
				exp.Key = alternativeKey
				break
			}
			if attempt == 10 {
				return fmt.Errorf("unable to resolve key conflict after 10 attempts")
			}
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert experiment (using tests table with new columns)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tests (id, name, description, key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, exp.ID, exp.Name, exp.Description, exp.Key, exp.Status, exp.CreatedAt, exp.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert experiment: %w", err)
	}

	// Insert variants (using arms table with new columns)
	for _, variant := range exp.Variants {
		// Generate variant key if not provided
		if variant.Key == "" {
			variant.Key = r.keyGen.GenerateVariantKey(exp.Key, variant.Name)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO arms (id, test_id, name, description, key, allocation_percent, successes, failures, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, variant.ID, variant.ExperimentID, variant.Name, variant.Description, variant.Key,
			variant.AllocationPercent, variant.Successes, variant.Failures, variant.CreatedAt, variant.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert variant %s: %w", variant.ID, err)
		}
	}

	return tx.Commit()
}

// GetExperiment retrieves an experiment by ID
func (r *ExperimentRepository) GetExperiment(ctx context.Context, id string) (*experiment.Experiment, error) {
	var exp experiment.Experiment
	var createdAt, updatedAt pq.NullTime
	var key, status sql.NullString

	query := `
		SELECT id, name, description, key, status, created_at, updated_at
		FROM tests
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&exp.ID, &exp.Name, &exp.Description, &key, &status, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("experiment not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get experiment: %w", err)
	}

	// Handle nullable fields
	if key.Valid {
		exp.Key = key.String
	}
	if status.Valid {
		exp.Status = status.String
	} else {
		exp.Status = "active" // Default status for legacy records
	}
	if createdAt.Valid {
		exp.CreatedAt = createdAt.Time
	} else {
		exp.CreatedAt = time.Now()
	}
	if updatedAt.Valid {
		exp.UpdatedAt = updatedAt.Time
	} else {
		exp.UpdatedAt = time.Now()
	}

	// Get variants
	variants, err := r.getVariantsByExperimentID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get variants: %w", err)
	}
	exp.Variants = variants

	return &exp, nil
}

// GetExperimentByKey retrieves an experiment by its key
func (r *ExperimentRepository) GetExperimentByKey(ctx context.Context, key string) (*experiment.Experiment, error) {
	var exp experiment.Experiment
	var createdAt, updatedAt pq.NullTime
	var keyField, status sql.NullString

	query := `
		SELECT id, name, description, key, status, created_at, updated_at
		FROM tests
		WHERE key = $1
	`

	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&exp.ID, &exp.Name, &exp.Description, &keyField, &status, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("experiment not found with key: %s", key)
		}
		return nil, fmt.Errorf("failed to get experiment by key: %w", err)
	}

	// Handle nullable fields
	if keyField.Valid {
		exp.Key = keyField.String
	}
	if status.Valid {
		exp.Status = status.String
	} else {
		exp.Status = "active"
	}
	if createdAt.Valid {
		exp.CreatedAt = createdAt.Time
	} else {
		exp.CreatedAt = time.Now()
	}
	if updatedAt.Valid {
		exp.UpdatedAt = updatedAt.Time
	} else {
		exp.UpdatedAt = time.Now()
	}

	// Get variants
	variants, err := r.getVariantsByExperimentID(ctx, exp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get variants: %w", err)
	}
	exp.Variants = variants

	return &exp, nil
}

// GetAllExperiments retrieves all experiments
func (r *ExperimentRepository) GetAllExperiments(ctx context.Context) ([]experiment.Experiment, error) {
	query := `
		SELECT id, name, description, key, status, created_at, updated_at
		FROM tests
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query experiments: %w", err)
	}
	defer rows.Close()

	var experiments []experiment.Experiment
	for rows.Next() {
		var exp experiment.Experiment
		var createdAt, updatedAt pq.NullTime
		var key, status sql.NullString

		err := rows.Scan(
			&exp.ID, &exp.Name, &exp.Description, &key, &status, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan experiment: %w", err)
		}

		// Handle nullable fields
		if key.Valid {
			exp.Key = key.String
		}
		if status.Valid {
			exp.Status = status.String
		} else {
			exp.Status = "active"
		}
		if createdAt.Valid {
			exp.CreatedAt = createdAt.Time
		} else {
			exp.CreatedAt = time.Now()
		}
		if updatedAt.Valid {
			exp.UpdatedAt = updatedAt.Time
		} else {
			exp.UpdatedAt = time.Now()
		}

		// Get variants for this experiment
		variants, err := r.getVariantsByExperimentID(ctx, exp.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get variants for experiment %s: %w", exp.ID, err)
		}
		exp.Variants = variants

		experiments = append(experiments, exp)
	}

	return experiments, rows.Err()
}

// GetExperimentsByStatus retrieves experiments filtered by status
func (r *ExperimentRepository) GetExperimentsByStatus(ctx context.Context, status string) ([]experiment.Experiment, error) {
	query := `
		SELECT id, name, description, key, status, created_at, updated_at
		FROM tests
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query experiments by status: %w", err)
	}
	defer rows.Close()

	var experiments []experiment.Experiment
	for rows.Next() {
		var exp experiment.Experiment
		var createdAt, updatedAt pq.NullTime
		var key, statusField sql.NullString

		err := rows.Scan(
			&exp.ID, &exp.Name, &exp.Description, &key, &statusField, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan experiment: %w", err)
		}

		// Handle nullable fields
		if key.Valid {
			exp.Key = key.String
		}
		if statusField.Valid {
			exp.Status = statusField.String
		}
		if createdAt.Valid {
			exp.CreatedAt = createdAt.Time
		} else {
			exp.CreatedAt = time.Now()
		}
		if updatedAt.Valid {
			exp.UpdatedAt = updatedAt.Time
		} else {
			exp.UpdatedAt = time.Now()
		}

		// Get variants for this experiment
		variants, err := r.getVariantsByExperimentID(ctx, exp.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get variants for experiment %s: %w", exp.ID, err)
		}
		exp.Variants = variants

		experiments = append(experiments, exp)
	}

	return experiments, rows.Err()
}

// UpdateExperiment updates an existing experiment
func (r *ExperimentRepository) UpdateExperiment(ctx context.Context, exp *experiment.Experiment) error {
	query := `
		UPDATE tests
		SET name = $2, description = $3, key = $4, status = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		exp.ID, exp.Name, exp.Description, exp.Key, exp.Status, exp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update experiment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("experiment not found: %s", exp.ID)
	}

	return nil
}

// UpdateExperimentStatus updates the status of an experiment
func (r *ExperimentRepository) UpdateExperimentStatus(ctx context.Context, id string, status string) error {
	query := `
		UPDATE tests
		SET status = $2, updated_at = $3
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update experiment status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("experiment not found: %s", id)
	}

	return nil
}

// DeleteExperiment deletes an experiment by ID
func (r *ExperimentRepository) DeleteExperiment(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete variants (arms) first due to foreign key constraint
	_, err = tx.ExecContext(ctx, `DELETE FROM arms WHERE test_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete variants: %w", err)
	}

	// Delete experiment
	result, err := tx.ExecContext(ctx, `DELETE FROM tests WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete experiment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("experiment not found: %s", id)
	}

	return tx.Commit()
}

// KeyExists checks if an experiment key already exists
func (r *ExperimentRepository) KeyExists(ctx context.Context, key string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM tests WHERE key = $1`

	err := r.db.QueryRowContext(ctx, query, key).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return count > 0, nil
}

// getVariantsByExperimentID is a helper method to get variants for an experiment
func (r *ExperimentRepository) getVariantsByExperimentID(ctx context.Context, experimentID string) ([]experiment.Variant, error) {
	query := `
		SELECT id, test_id, name, description, key, allocation_percent, successes, failures, created_at, updated_at
		FROM arms
		WHERE test_id = $1
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, experimentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query variants: %w", err)
	}
	defer rows.Close()

	var variants []experiment.Variant
	for rows.Next() {
		var variant experiment.Variant
		var createdAt, updatedAt pq.NullTime
		var key sql.NullString
		var allocationPercent sql.NullInt64

		err := rows.Scan(
			&variant.ID, &variant.ExperimentID, &variant.Name, &variant.Description,
			&key, &allocationPercent, &variant.Successes, &variant.Failures,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan variant: %w", err)
		}

		// Handle nullable fields
		if key.Valid {
			variant.Key = key.String
		}
		if allocationPercent.Valid {
			variant.AllocationPercent = int(allocationPercent.Int64)
		}
		if createdAt.Valid {
			variant.CreatedAt = createdAt.Time
		} else {
			variant.CreatedAt = time.Now()
		}
		if updatedAt.Valid {
			variant.UpdatedAt = updatedAt.Time
		} else {
			variant.UpdatedAt = time.Now()
		}

		variants = append(variants, variant)
	}

	return variants, rows.Err()
}
