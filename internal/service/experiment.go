// Package service contains business logic implementations
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gobandit/internal/domain/experiment"
)

// ExperimentService implements the experiment business logic
type ExperimentService struct {
	testRepo    experiment.TestRepository
	armRepo     experiment.ArmRepository
	thompsonSvc experiment.ThompsonSamplingService
}

// NewExperimentService creates a new experiment service
func NewExperimentService(
	testRepo experiment.TestRepository,
	armRepo experiment.ArmRepository,
	thompsonSvc experiment.ThompsonSamplingService,
) *ExperimentService {
	return &ExperimentService{
		testRepo:    testRepo,
		armRepo:     armRepo,
		thompsonSvc: thompsonSvc,
	}
}

// CreateTest creates a new test with specified number of arms
func (s *ExperimentService) CreateTest(ctx context.Context, name, description string, numArms int) (*experiment.Test, error) {
	if numArms < 2 {
		return nil, fmt.Errorf("test must have at least 2 arms, got %d", numArms)
	}

	now := time.Now()
	test := &experiment.Test{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create arms for the test
	test.Arms = make([]experiment.Arm, numArms)
	for i := 0; i < numArms; i++ {
		test.Arms[i] = experiment.Arm{
			ID:          uuid.New().String(),
			TestID:      test.ID,
			Name:        fmt.Sprintf("Arm %d", i+1),
			Description: fmt.Sprintf("Description for arm %d", i+1),
			Successes:   0,
			Failures:    0,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	// Save test to repository
	if err := s.testRepo.CreateTest(ctx, test); err != nil {
		return nil, fmt.Errorf("failed to create test: %w", err)
	}

	return test, nil
}

// GetTest retrieves a test by ID
func (s *ExperimentService) GetTest(ctx context.Context, id string) (*experiment.Test, error) {
	test, err := s.testRepo.GetTest(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get test: %w", err)
	}
	return test, nil
}

// GetAllTests retrieves all tests
func (s *ExperimentService) GetAllTests(ctx context.Context) ([]experiment.Test, error) {
	tests, err := s.testRepo.GetAllTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tests: %w", err)
	}
	return tests, nil
}

// GetArmAssignment returns the next arm to test using Thompson Sampling
func (s *ExperimentService) GetArmAssignment(ctx context.Context, testID string) (*experiment.Arm, error) {
	arms, err := s.armRepo.GetArmsByTestID(ctx, testID)
	if err != nil {
		return nil, fmt.Errorf("failed to get arms for test %s: %w", testID, err)
	}

	if len(arms) == 0 {
		return nil, fmt.Errorf("test %s not found or has no arms", testID)
	}

	selectedArm := s.thompsonSvc.SelectArm(arms)
	if selectedArm == nil {
		return nil, fmt.Errorf("failed to select arm for test %s", testID)
	}

	return selectedArm, nil
}

// RecordResult records the result of an arm pull
func (s *ExperimentService) RecordResult(ctx context.Context, armID string, success bool) (*experiment.Arm, error) {
	arm, err := s.armRepo.UpdateArmStats(ctx, armID, success)
	if err != nil {
		return nil, fmt.Errorf("failed to record result for arm %s: %w", armID, err)
	}
	return arm, nil
}

// GetArmStats retrieves statistics for all arms in a test
func (s *ExperimentService) GetArmStats(ctx context.Context, testID string) ([]experiment.Arm, error) {
	arms, err := s.armRepo.GetArmsByTestID(ctx, testID)
	if err != nil {
		return nil, fmt.Errorf("failed to get arm stats for test %s: %w", testID, err)
	}
	return arms, nil
}
