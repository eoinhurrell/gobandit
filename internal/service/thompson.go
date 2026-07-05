// Package service contains business logic implementations
package service

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"gobandit/internal/domain/experiment"
)

var (
	rng    *rand.Rand
	rngMux sync.Mutex
)

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// ThompsonSampling implements the Thompson Sampling service
type ThompsonSampling struct{}

// NewThompsonSampling creates a new Thompson Sampling service
func NewThompsonSampling() *ThompsonSampling {
	return &ThompsonSampling{}
}

// SelectArm selects the best arm using Thompson Sampling algorithm
func (ts *ThompsonSampling) SelectArm(arms []experiment.Arm) *experiment.Arm {
	if len(arms) == 0 {
		return nil
	}

	rngMux.Lock()
	defer rngMux.Unlock()

	var (
		maxSample float64
		selected  *experiment.Arm
	)

	for i := range arms {
		// Sample from beta distribution using conjugate priors
		sample := betaDistribution(float64(arms[i].Successes+1), float64(arms[i].Failures+1))

		if sample > maxSample {
			maxSample = sample
			selected = &arms[i]
		}
	}

	return selected
}

// betaDistribution generates a random sample from a beta distribution
// Using the more robust method with gamma distributions
func betaDistribution(alpha, beta float64) float64 {
	// Handle edge cases
	if alpha <= 0 || beta <= 0 {
		return 0.0
	}

	x := gammaDistribution(alpha)
	y := gammaDistribution(beta)

	// Prevent division by zero
	if x+y == 0 {
		return 0.5 // Default to neutral value
	}

	return x / (x + y)
}

// gammaDistribution generates a random sample from a gamma distribution using Marsaglia and Tsang's method
func gammaDistribution(alpha float64) float64 {
	if alpha <= 0 {
		return 0.0
	}

	if alpha < 1 {
		// For alpha < 1, use Ahrens-Dieter acceptance-rejection method
		return gammaDistribution(1+alpha) * math.Pow(rng.Float64(), 1/alpha)
	}

	// Marsaglia and Tsang's method for alpha >= 1
	d := alpha - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		var x, v float64

		for {
			x = rng.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}

		v = v * v * v
		u := rng.Float64()

		if u < 1.0-0.331*x*x*x*x {
			return d * v
		}

		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}
