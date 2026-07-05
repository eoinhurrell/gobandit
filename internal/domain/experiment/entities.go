// Package experiment contains the domain models and business logic for A/B testing experiments
package experiment

import "time"

// Arm represents a single variant in an A/B test
type Arm struct {
	ID          string    `json:"id"`
	TestID      string    `json:"test_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Successes   int       `json:"successes"`
	Failures    int       `json:"failures"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SuccessRate calculates the success rate of the arm
func (a *Arm) SuccessRate() float64 {
	total := a.Successes + a.Failures
	if total == 0 {
		return 0.0
	}
	return float64(a.Successes) / float64(total)
}

// TotalPulls returns the total number of pulls for this arm
func (a *Arm) TotalPulls() int {
	return a.Successes + a.Failures
}

// Test represents an A/B test with multiple arms
type Test struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Arms        []Arm     `json:"arms"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetArm returns an arm by ID, or nil if not found
func (t *Test) GetArm(armID string) *Arm {
	for i := range t.Arms {
		if t.Arms[i].ID == armID {
			return &t.Arms[i]
		}
	}
	return nil
}

// Result represents the outcome of an arm pull
type Result struct {
	Success bool `json:"success"`
}

// RecordResultRequest represents a request to record an arm result
type RecordResultRequest struct {
	ArmID  string
	Result Result
}

// ArmAssignment represents an arm assignment response
type ArmAssignment struct {
	Arm Arm `json:"arm"`
}

// Experiment represents the new experiment model with enhanced features
type Experiment struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"` // New: URL-safe identifier for API access
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // New: draft/active/paused/completed
	Variants    []Variant `json:"variants"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetVariant returns a variant by ID, or nil if not found
func (e *Experiment) GetVariant(variantID string) *Variant {
	for i := range e.Variants {
		if e.Variants[i].ID == variantID {
			return &e.Variants[i]
		}
	}
	return nil
}

// GetVariantByKey returns a variant by key, or nil if not found
func (e *Experiment) GetVariantByKey(variantKey string) *Variant {
	for i := range e.Variants {
		if e.Variants[i].Key == variantKey {
			return &e.Variants[i]
		}
	}
	return nil
}

// IsActive returns true if the experiment is in active status
func (e *Experiment) IsActive() bool {
	return e.Status == "active"
}

// TotalAllocation returns the total allocation percentage across all variants
func (e *Experiment) TotalAllocation() int {
	total := 0
	for _, variant := range e.Variants {
		total += variant.AllocationPercent
	}
	return total
}

// Variant represents the new variant model with enhanced features
type Variant struct {
	ID                string    `json:"id"`
	ExperimentID      string    `json:"experiment_id"`
	Key               string    `json:"key"` // New: URL-safe identifier
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	AllocationPercent int       `json:"allocation_percent"` // New: traffic allocation percentage
	Successes         int       `json:"successes"`
	Failures          int       `json:"failures"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SuccessRate calculates the success rate of the variant
func (v *Variant) SuccessRate() float64 {
	total := v.Successes + v.Failures
	if total == 0 {
		return 0.0
	}
	return float64(v.Successes) / float64(total)
}

// TotalPulls returns the total number of pulls for this variant
func (v *Variant) TotalPulls() int {
	return v.Successes + v.Failures
}

// VariantAssignment represents a variant assignment response
type VariantAssignment struct {
	Variant Variant `json:"variant"`
}

// RecordVariantResultRequest represents a request to record a variant result
type RecordVariantResultRequest struct {
	VariantID string
	Result    Result
}
