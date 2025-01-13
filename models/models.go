package models

import "time"

// Arm represents a single variant in an A/B test
type Arm struct {
	ID          string `json:"id"`
	TestID      string `json:"test_id"`
	Name        string `json:"name"`
	Successes   int    `json:"successes"`
	Failures    int    `json:"failures"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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
