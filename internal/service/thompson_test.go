package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gobandit/internal/domain/experiment"
)

func TestThompsonSamplingService(t *testing.T) {
	ts := NewThompsonSampling()

	arms := []experiment.Arm{
		{ID: "1", Successes: 6, Failures: 4}, // 60% success rate
		{ID: "2", Successes: 3, Failures: 7}, // 30% success rate
		{ID: "3", Successes: 5, Failures: 5}, // 50% success rate
	}

	// Run sampling multiple times to verify probabilistic behavior
	results := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		selected := ts.SelectArm(arms)
		assert.NotNil(t, selected, "SelectArm should not return nil")
		results[selected.ID]++
	}

	// Verify probabilistic behavior - arm 1 should be selected most often
	assert.Greater(t, results["1"], results["2"], "Arm 1 should be selected more often than arm 2")
	assert.Greater(t, results["1"], results["3"], "Arm 1 should be selected more often than arm 3")

	// All arms should be selected at least once given enough iterations
	assert.Greater(t, results["1"], 0, "Arm 1 should be selected at least once")
	assert.Greater(t, results["2"], 0, "Arm 2 should be selected at least once")
	assert.Greater(t, results["3"], 0, "Arm 3 should be selected at least once")

	t.Logf("Selection results: %v", results)
}

func TestThompsonSamplingEmptyArms(t *testing.T) {
	ts := NewThompsonSampling()
	selected := ts.SelectArm([]experiment.Arm{})
	assert.Nil(t, selected, "SelectArm should return nil for empty arms")
}

func TestThompsonSamplingSingleArm(t *testing.T) {
	ts := NewThompsonSampling()
	arms := []experiment.Arm{
		{ID: "only", Successes: 10, Failures: 5},
	}

	selected := ts.SelectArm(arms)
	assert.NotNil(t, selected)
	assert.Equal(t, "only", selected.ID)
}
