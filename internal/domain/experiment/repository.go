// Package experiment defines repository interfaces for experiment domain
package experiment

import "context"

// TestRepository defines the interface for test persistence
type TestRepository interface {
	// CreateTest creates a new test with its arms
	CreateTest(ctx context.Context, test *Test) error

	// GetTest retrieves a test by ID
	GetTest(ctx context.Context, id string) (*Test, error)

	// GetAllTests retrieves all tests
	GetAllTests(ctx context.Context) ([]Test, error)

	// UpdateTest updates an existing test
	UpdateTest(ctx context.Context, test *Test) error

	// DeleteTest deletes a test by ID
	DeleteTest(ctx context.Context, id string) error
}

// ArmRepository defines the interface for arm persistence
type ArmRepository interface {
	// GetArm retrieves an arm by ID
	GetArm(ctx context.Context, id string) (*Arm, error)

	// GetArmsByTestID retrieves all arms for a given test
	GetArmsByTestID(ctx context.Context, testID string) ([]Arm, error)

	// UpdateArmStats updates the success/failure counts for an arm
	UpdateArmStats(ctx context.Context, armID string, success bool) (*Arm, error)

	// CreateArm creates a new arm
	CreateArm(ctx context.Context, arm *Arm) error

	// UpdateArm updates an existing arm
	UpdateArm(ctx context.Context, arm *Arm) error

	// DeleteArm deletes an arm by ID
	DeleteArm(ctx context.Context, id string) error
}

// ExperimentRepository defines the interface for experiment persistence
type ExperimentRepository interface {
	// CreateExperiment creates a new experiment with its variants
	CreateExperiment(ctx context.Context, experiment *Experiment) error

	// GetExperiment retrieves an experiment by ID
	GetExperiment(ctx context.Context, id string) (*Experiment, error)

	// GetExperimentByKey retrieves an experiment by its key
	GetExperimentByKey(ctx context.Context, key string) (*Experiment, error)

	// GetAllExperiments retrieves all experiments
	GetAllExperiments(ctx context.Context) ([]Experiment, error)

	// GetExperimentsByStatus retrieves experiments filtered by status
	GetExperimentsByStatus(ctx context.Context, status string) ([]Experiment, error)

	// UpdateExperiment updates an existing experiment
	UpdateExperiment(ctx context.Context, experiment *Experiment) error

	// UpdateExperimentStatus updates the status of an experiment
	UpdateExperimentStatus(ctx context.Context, id string, status string) error

	// DeleteExperiment deletes an experiment by ID
	DeleteExperiment(ctx context.Context, id string) error

	// KeyExists checks if an experiment key already exists
	KeyExists(ctx context.Context, key string) (bool, error)
}

// VariantRepository defines the interface for variant persistence
type VariantRepository interface {
	// GetVariant retrieves a variant by ID
	GetVariant(ctx context.Context, id string) (*Variant, error)

	// GetVariantByKey retrieves a variant by its key
	GetVariantByKey(ctx context.Context, key string) (*Variant, error)

	// GetVariantsByExperimentID retrieves all variants for a given experiment
	GetVariantsByExperimentID(ctx context.Context, experimentID string) ([]Variant, error)

	// UpdateVariantStats updates the success/failure counts for a variant
	UpdateVariantStats(ctx context.Context, variantID string, success bool) (*Variant, error)

	// CreateVariant creates a new variant
	CreateVariant(ctx context.Context, variant *Variant) error

	// UpdateVariant updates an existing variant
	UpdateVariant(ctx context.Context, variant *Variant) error

	// DeleteVariant deletes a variant by ID
	DeleteVariant(ctx context.Context, id string) error

	// KeyExists checks if a variant key already exists within an experiment
	KeyExists(ctx context.Context, experimentID, key string) (bool, error)

	// GetVariantAllocationTotal returns the total allocation percentage for an experiment
	GetVariantAllocationTotal(ctx context.Context, experimentID string) (int, error)
}
