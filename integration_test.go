package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gobandit/models"
)

// TestRefactoredServerIntegration tests that the refactored server maintains the same API behavior
func TestRefactoredServerIntegration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	server := NewServer(db)

	t.Run("routes are properly configured", func(t *testing.T) {
		// Test that expected routes exist
		err := server.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			pathTemplate, _ := route.GetPathTemplate()
			methods, _ := route.GetMethods()

			switch pathTemplate {
			case "/tests":
				assert.Contains(t, methods, "POST", "POST /tests should exist")
			case "/tests/{testID}/arm":
				assert.Contains(t, methods, "GET", "GET /tests/{testID}/arm should exist")
			case "/tests/{testID}/arms/{armID}/result":
				assert.Contains(t, methods, "POST", "POST /tests/{testID}/arms/{armID}/result should exist")
			case "/":
				assert.Contains(t, methods, "GET", "GET / should exist")
			case "/tests/{testID}/arms":
				assert.Contains(t, methods, "GET", "GET /tests/{testID}/arms should exist")
			}
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("assignment API endpoint works", func(t *testing.T) {
		testID := "test-123"

		// Mock arms data for Thompson Sampling
		mock.ExpectQuery(`SELECT .+ FROM arms WHERE test_id`).
			WithArgs(testID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "successes", "failures", "created_at", "updated_at"}).
				AddRow("arm-1", "Control", "Control arm", 10, 5, "2024-01-01", "2024-01-01").
				AddRow("arm-2", "Variant", "Variant arm", 15, 8, "2024-01-01", "2024-01-01"))

		req := httptest.NewRequest("GET", "/tests/"+testID+"/arm", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, mock.ExpectationsWereMet())

		var response models.Arm
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Contains(t, []string{"arm-1", "arm-2"}, response.ID)
	})

	t.Run("record result endpoint works", func(t *testing.T) {
		armID := "arm-123"

		// Mock result recording
		mock.ExpectQuery(`UPDATE arms SET`).
			WithArgs(true, armID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "test_id", "name", "description", "successes", "failures", "created_at", "updated_at"}).
				AddRow(armID, "test-123", "Test Arm", "Description", 11, 5, "2024-01-01", "2024-01-01"))

		result := map[string]bool{"success": true}
		body, _ := json.Marshal(result)
		req := httptest.NewRequest("POST", "/tests/test-123/arms/"+armID+"/result", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, mock.ExpectationsWereMet())

		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, float64(11), response["successes"])
		assert.Equal(t, float64(5), response["failures"])
	})

	t.Run("dashboard endpoint works", func(t *testing.T) {
		// Mock tests data
		mock.ExpectQuery(`SELECT .+ FROM tests`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
				AddRow("test-1", "Test 1", "Description 1", "2024-01-01", "2024-01-01"))

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, mock.ExpectationsWereMet())

		// Verify HTML content is returned
		body := w.Body.String()
		assert.Contains(t, body, "A/B Test Dashboard")
	})

	t.Run("create test endpoint works", func(t *testing.T) {
		// Mock test creation
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO tests`).
			WithArgs(sqlmock.AnyArg(), "Test Campaign", "A/B Test for button color", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Mock arm creation
		mock.ExpectExec(`INSERT INTO arms`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Arm 1", "Description for arm 1", 0, 0, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`INSERT INTO arms`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "Arm 2", "Description for arm 2", 0, 0, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectCommit()

		// Create form data
		form := url.Values{}
		form.Set("name", "Test Campaign")
		form.Set("description", "A/B Test for button color")
		form.Set("numArms", "2")

		req := httptest.NewRequest("POST", "/tests", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
