package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"gobandit/internal/domain/experiment"
)

// VariantRepository implements the VariantRepository interface using PostgreSQL
type VariantRepository struct {
	db     *sql.DB
	keyGen experiment.KeyGenerator
}

// NewVariantRepository creates a new PostgreSQL variant repository
func NewVariantRepository(db *sql.DB, keyGen experiment.KeyGenerator) *VariantRepository {
	return &VariantRepository{
		db:     db,
		keyGen: keyGen,
	}
}

// GetVariant retrieves a variant by ID
func (r *VariantRepository) GetVariant(ctx context.Context, id string) (*experiment.Variant, error) {
	var variant experiment.Variant
	var createdAt, updatedAt pq.NullTime
	var key sql.NullString
	var allocationPercent sql.NullInt64

	query := `
		SELECT id, test_id, name, description, key, allocation_percent, successes, failures, created_at, updated_at
		FROM arms
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&variant.ID, &variant.ExperimentID, &variant.Name, &variant.Description,
		&key, &allocationPercent, &variant.Successes, &variant.Failures,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("variant not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get variant: %w", err)
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

	return &variant, nil
}

// GetVariantByKey retrieves a variant by its key
func (r *VariantRepository) GetVariantByKey(ctx context.Context, key string) (*experiment.Variant, error) {
	var variant experiment.Variant
	var createdAt, updatedAt pq.NullTime
	var keyField sql.NullString
	var allocationPercent sql.NullInt64

	query := `
		SELECT id, test_id, name, description, key, allocation_percent, successes, failures, created_at, updated_at
		FROM arms
		WHERE key = $1
	`

	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&variant.ID, &variant.ExperimentID, &variant.Name, &variant.Description,
		&keyField, &allocationPercent, &variant.Successes, &variant.Failures,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("variant not found with key: %s", key)
		}
		return nil, fmt.Errorf("failed to get variant by key: %w", err)
	}

	// Handle nullable fields
	if keyField.Valid {
		variant.Key = keyField.String
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

	return &variant, nil
}

// GetVariantsByExperimentID retrieves all variants for a given experiment
func (r *VariantRepository) GetVariantsByExperimentID(ctx context.Context, experimentID string) ([]experiment.Variant, error) {
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

// UpdateVariantStats updates the success/failure counts for a variant
func (r *VariantRepository) UpdateVariantStats(ctx context.Context, variantID string, success bool) (*experiment.Variant, error) {
	query := `
		UPDATE arms
		SET 
			successes = CASE WHEN $1 THEN successes + 1 ELSE successes END,
			failures = CASE WHEN $1 THEN failures ELSE failures + 1 END,
			updated_at = NOW()
		WHERE id = $2
		RETURNING id, test_id, name, description, key, allocation_percent, successes, failures, created_at, updated_at
	`

	var variant experiment.Variant
	var createdAt, updatedAt pq.NullTime
	var key sql.NullString
	var allocationPercent sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, success, variantID).Scan(
		&variant.ID, &variant.ExperimentID, &variant.Name, &variant.Description,
		&key, &allocationPercent, &variant.Successes, &variant.Failures,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("variant not found: %s", variantID)
		}
		return nil, fmt.Errorf("failed to update variant stats: %w", err)
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

	return &variant, nil
}

// CreateVariant creates a new variant
func (r *VariantRepository) CreateVariant(ctx context.Context, variant *experiment.Variant) error {
	// Generate key if not provided
	if variant.Key == "" {
		variant.Key = r.keyGen.GenerateVariantKey("", variant.Name)
	}

	// Validate key
	if err := r.keyGen.ValidateKey(variant.Key); err != nil {
		return fmt.Errorf("invalid variant key: %w", err)
	}

	// Check for key conflicts within the experiment
	exists, err := r.KeyExists(ctx, variant.ExperimentID, variant.Key)
	if err != nil {
		return fmt.Errorf("failed to check key existence: %w", err)
	}

	if exists {
		// Resolve conflict by trying alternatives
		for attempt := 1; attempt <= 10; attempt++ {
			resolver := experiment.NewConflictResolver(r.keyGen)
			alternativeKey := resolver.ResolveVariantKeyConflict(variant.Key, attempt)
			exists, err := r.KeyExists(ctx, variant.ExperimentID, alternativeKey)
			if err != nil {
				return fmt.Errorf("failed to check alternative key existence: %w", err)
			}
			if !exists {
				variant.Key = alternativeKey
				break
			}
			if attempt == 10 {
				return fmt.Errorf("unable to resolve variant key conflict after 10 attempts")
			}
		}
	}

	query := `
		INSERT INTO arms (id, test_id, name, description, key, allocation_percent, successes, failures, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.ExecContext(ctx, query,
		variant.ID, variant.ExperimentID, variant.Name, variant.Description, variant.Key,
		variant.AllocationPercent, variant.Successes, variant.Failures,
		variant.CreatedAt, variant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create variant: %w", err)
	}

	return nil
}

// UpdateVariant updates an existing variant
func (r *VariantRepository) UpdateVariant(ctx context.Context, variant *experiment.Variant) error {
	// Validate key if it's being changed
	if err := r.keyGen.ValidateKey(variant.Key); err != nil {
		return fmt.Errorf("invalid variant key: %w", err)
	}

	query := `
		UPDATE arms
		SET name = $2, description = $3, key = $4, allocation_percent = $5, successes = $6, failures = $7, updated_at = $8
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		variant.ID, variant.Name, variant.Description, variant.Key, variant.AllocationPercent,
		variant.Successes, variant.Failures, variant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update variant: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("variant not found: %s", variant.ID)
	}

	return nil
}

// DeleteVariant deletes a variant by ID
func (r *VariantRepository) DeleteVariant(ctx context.Context, id string) error {
	query := `DELETE FROM arms WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete variant: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("variant not found: %s", id)
	}

	return nil
}

// KeyExists checks if a variant key already exists within an experiment
func (r *VariantRepository) KeyExists(ctx context.Context, experimentID, key string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM arms WHERE test_id = $1 AND key = $2`

	err := r.db.QueryRowContext(ctx, query, experimentID, key).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check variant key existence: %w", err)
	}

	return count > 0, nil
}

// GetVariantAllocationTotal returns the total allocation percentage for an experiment
func (r *VariantRepository) GetVariantAllocationTotal(ctx context.Context, experimentID string) (int, error) {
	var total sql.NullInt64
	query := `SELECT SUM(allocation_percent) FROM arms WHERE test_id = $1`

	err := r.db.QueryRowContext(ctx, query, experimentID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get allocation total: %w", err)
	}

	if total.Valid {
		return int(total.Int64), nil
	}
	return 0, nil
}
