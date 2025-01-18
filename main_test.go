package main

import (
	"bytes"
	"context"
	"encoding/json"
	"gobandit/models"
	"gobandit/templates"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateTest(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	server := NewServer(db)

	// Test data
	test := models.Test{
		Name:        "Test Campaign",
		Description: "A/B Test for button color",
		Arms: []models.Arm{
			{Name: "Blue Button", Description: "Control"},
			{Name: "Red Button", Description: "Variant"},
		},
	}

	// DB expectations
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO tests`).
		WithArgs(test.Name, test.Description).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow("test-123", time.Now(), time.Now()))

	for range test.Arms {
		mock.ExpectQuery(`INSERT INTO arms`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
				AddRow("arm-123", time.Now(), time.Now()))
	}

	mock.ExpectCommit()

	// Execute request
	body, _ := json.Marshal(test)
	req := httptest.NewRequest("POST", "/tests", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	server.handleCreateTest(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())

	var response models.Test
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, test.Name, response.Name)
	assert.Len(t, response.Arms, len(test.Arms))
}

func TestGetArm(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	server := NewServer(db)

	// Test data
	testID := "test-123"
	arms := []models.Arm{
		{ID: "arm-1", Name: "Control", Successes: 10, Failures: 5},
		{ID: "arm-2", Name: "Variant", Successes: 15, Failures: 8},
	}

	// DB expectations
	mock.ExpectQuery(`SELECT .+ FROM arms`).
		WithArgs(testID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "successes", "failures"}).
			AddRow(arms[0].ID, arms[0].Name, arms[0].Successes, arms[0].Failures).
			AddRow(arms[1].ID, arms[1].Name, arms[1].Successes, arms[1].Failures))

	// Execute request
	req := httptest.NewRequest("GET", "/tests/"+testID+"/arm", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())

	var response models.Arm
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Contains(t, []string{arms[0].ID, arms[1].ID}, response.ID)
}

func TestRecordResult(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	server := NewServer(db)

	// Test data
	armID := "arm-123"
	result := map[string]bool{"success": true}
	newSuccesses := 11
	newFailures := 5

	// DB expectations
	mock.ExpectQuery(`UPDATE arms SET`).
		WithArgs(true, armID).
		WillReturnRows(sqlmock.NewRows([]string{"successes", "failures"}).
			AddRow(newSuccesses, newFailures))

	// Execute request
	body, _ := json.Marshal(result)
	req := httptest.NewRequest("POST", "/tests/test-123/arms/"+armID+"/result", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, float64(newSuccesses), response["successes"])
	assert.Equal(t, float64(newFailures), response["failures"])
}

func TestThompsonSampling(t *testing.T) {
	arms := []models.Arm{
		{ID: "1", Successes: 100, Failures: 0},
		{ID: "2", Successes: 0, Failures: 100},
		{ID: "3", Successes: 50, Failures: 50},
	}

	// Run multiple times to ensure probabilistic behavior
	selected := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		arm := thompsonSampling(arms)
		selected[arm.ID]++
	}

	// The arm with highest success rate should be selected more often
	assert.Greater(t, selected["1"], selected["2"])
	assert.Greater(t, selected["1"], selected["3"])
}

func TestBetaDistribution(t *testing.T) {
	testCases := []struct {
		alpha, beta float64
	}{
		{1.0, 1.0},    // Uniform distribution
		{10.0, 1.0},   // Strongly skewed towards 1
		{1.0, 10.0},   // Strongly skewed towards 0
		{100.0, 50.0}, // Sharp peak
	}

	for _, tc := range testCases {
		sample := betaDistribution(tc.alpha, tc.beta)
		assert.GreaterOrEqual(t, sample, 0.0)
		assert.LessOrEqual(t, sample, 1.0)
	}
}

// TestDashboardRendering tests the complete rendering of the dashboard template
func TestDashboardRendering(t *testing.T) {
	// Setup test data
	tests := []models.Test{
		{
			ID:        "1",
			Name:      "Test Campaign 1",
			CreatedAt: time.Date(2024, time.January, 14, 15, 30, 45, 0, time.UTC),
		},
		{
			ID:        "2",
			Name:      "Test Campaign 2",
			CreatedAt: time.Date(2024, time.January, 14, 15, 30, 45, 0, time.UTC),
		},
	}

	// Create a string builder to capture the rendered HTML
	var builder strings.Builder

	// Render the dashboard template
	err := templates.Dashboard(tests).Render(context.Background(), &builder)
	assert.NoError(t, err, "Template should render without error")

	// Get the rendered HTML
	result := builder.String()

	// Test for expected content
	assert.Contains(t, result, "A/B Test Dashboard", "Should contain the dashboard title")
	assert.Contains(t, result, "Test Campaign 1", "Should contain the first test name")
	assert.Contains(t, result, "Test Campaign 2", "Should contain the second test name")
	assert.Contains(t, result, "htmx.org", "Should contain HTMX script")
	assert.Contains(t, result, "tailwindcss", "Should contain Tailwind script")
}

// TestEmptyDashboard tests rendering with no tests
func TestEmptyDashboard(t *testing.T) {
	var builder strings.Builder
	err := templates.Dashboard([]models.Test{}).Render(context.Background(), &builder)
	assert.NoError(t, err, "Empty dashboard should render without error")

	result := builder.String()
	assert.Contains(t, result, "A/B Test Dashboard", "Should still contain the dashboard title")
	assert.NotContains(t, result, "Test Campaign", "Should not contain any test campaigns")
}

// TestDashboardSanitization tests that the template properly escapes HTML
func TestDashboardSanitization(t *testing.T) {
	tests := []models.Test{
		{
			ID:        "1",
			Name:      "<script>alert('xss')</script>",
			CreatedAt: time.Date(2024, time.January, 14, 15, 30, 45, 0, time.UTC),
		},
	}

	var builder strings.Builder
	err := templates.Dashboard(tests).Render(context.Background(), &builder)
	assert.NoError(t, err, "Template should render without error")

	result := builder.String()
	assert.Contains(t, result, "&lt;script&gt;", "HTML in test name should be escaped")
	assert.NotContains(t, result, "<script>alert('xss')</script>", "Raw HTML should not appear in output")
}

// TestLayoutStructure tests the basic structure of the layout
func TestLayoutStructure(t *testing.T) {
	var builder strings.Builder
	// We'll use Dashboard with empty tests to test the layout
	err := templates.Dashboard([]models.Test{}).Render(context.Background(), &builder)
	assert.NoError(t, err, "Layout should render without error")

	result := builder.String()

	// Test for required HTML structure
	assert.Contains(t, result, "<!doctype html>", "Should contain DOCTYPE declaration")
	assert.Contains(t, result, "<html lang=\"en\">", "Should contain HTML tag with lang attribute")
	assert.Contains(t, result, "<meta charset=\"UTF-8\"", "Should contain UTF-8 charset meta tag")
	assert.Contains(t, result, "<meta name=\"viewport\"", "Should contain viewport meta tag")
	assert.Contains(t, result, "<body class=\"bg-gray-100\">", "Should contain body tag with correct class")
}
