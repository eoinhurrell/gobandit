// Package experiment defines service interfaces for experiment domain
package experiment

import "context"

// Service defines the interface for experiment business logic
type Service interface {
	// CreateTest creates a new test with specified number of arms
	CreateTest(ctx context.Context, name, description string, numArms int) (*Test, error)

	// GetTest retrieves a test by ID
	GetTest(ctx context.Context, id string) (*Test, error)

	// GetAllTests retrieves all tests
	GetAllTests(ctx context.Context) ([]Test, error)

	// GetArmAssignment returns the next arm to test using Thompson Sampling
	GetArmAssignment(ctx context.Context, testID string) (*Arm, error)

	// RecordResult records the result of an arm pull
	RecordResult(ctx context.Context, armID string, success bool) (*Arm, error)

	// GetArmStats retrieves statistics for all arms in a test
	GetArmStats(ctx context.Context, testID string) ([]Arm, error)
}

// ThompsonSamplingService defines the interface for Thompson Sampling algorithm
type ThompsonSamplingService interface {
	// SelectArm selects the best arm using Thompson Sampling
	SelectArm(arms []Arm) *Arm
}
