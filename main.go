package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"gobandit/models"
	"gobandit/templates"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Server encapsulates the MAB server state
type Server struct {
	db     *sql.DB
	router *mux.Router
}

// NewServer creates a new MAB server instance
func NewServer(db *sql.DB) *Server {
	s := &Server{
		db:     db,
		router: mux.NewRouter(),
	}
	s.routes()
	return s
}

// routes sets up the HTTP routing
func (s *Server) routes() {
	s.router.HandleFunc("/tests", s.handleCreateTest).Methods("POST")
	s.router.HandleFunc("/tests/{testID}/arm", s.handleGetArm).Methods("GET")
	s.router.HandleFunc("/tests/{testID}/arms/{armID}/result", s.handleRecordResult).Methods("POST")

	// Dashboard routes
	s.router.HandleFunc("/", s.handleDashboard).Methods("GET")
	s.router.HandleFunc("/tests/{testID}/arms", s.handleGetArmStats).Methods("GET")
}

// handleDashboard renders the main dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tests, err := s.getAllTests()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.Dashboard(tests).Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleCreateTest creates a new test with the specified arms
func (s *Server) handleCreateTest(w http.ResponseWriter, r *http.Request) {
	var test models.Test
	if err := json.NewDecoder(r.Body).Decode(&test); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Insert test
	err = tx.QueryRow(`
		INSERT INTO tests (name, description)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`, test.Name, test.Description).Scan(&test.ID, &test.CreatedAt, &test.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Insert arms
	for i := range test.Arms {
		test.Arms[i].TestID = test.ID
		err = tx.QueryRow(`
			INSERT INTO arms (test_id, name, description)
			VALUES ($1, $2, $3)
			RETURNING id, created_at, updated_at
		`, test.ID, test.Arms[i].Name, test.Arms[i].Description).
			Scan(&test.Arms[i].ID, &test.Arms[i].CreatedAt, &test.Arms[i].UpdatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(test)
}

// handleGetArmStats returns stats for all arms in a test
func (s *Server) handleGetArmStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["testID"]

	arms, err := s.getTestArms(testID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.ArmStats(arms).Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getAllTests retrieves all tests from the database
func (s *Server) getAllTests() ([]models.Test, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, created_at, updated_at
		FROM tests
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []models.Test
	for rows.Next() {
		var test models.Test
		err := rows.Scan(&test.ID, &test.Name, &test.Description, &test.CreatedAt, &test.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tests = append(tests, test)
	}
	return tests, nil
}

// getTestArms retrieves all arms for a specific test
func (s *Server) getTestArms(testID string) ([]models.Arm, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, successes, failures, created_at, updated_at
		FROM arms
		WHERE test_id = $1
		ORDER BY name
	`, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var arms []models.Arm
	for rows.Next() {
		var arm models.Arm
		err := rows.Scan(
			&arm.ID, &arm.Name, &arm.Description,
			&arm.Successes, &arm.Failures,
			&arm.CreatedAt, &arm.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		arms = append(arms, arm)
	}
	return arms, nil
}

// handleGetArm returns the next arm to test using Thompson Sampling
func (s *Server) handleGetArm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["testID"]

	// Get all arms for the test
	rows, err := s.db.Query(`
		SELECT id, name, successes, failures
		FROM arms
		WHERE test_id = $1
	`, testID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var arms []models.Arm
	for rows.Next() {
		var arm models.Arm
		err := rows.Scan(&arm.ID, &arm.Name, &arm.Successes, &arm.Failures)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		arms = append(arms, arm)
	}

	if len(arms) == 0 {
		http.Error(w, "test not found", http.StatusNotFound)
		return
	}

	// Thompson Sampling
	selectedArm := thompsonSampling(arms)
	json.NewEncoder(w).Encode(selectedArm)
}

// handleRecordResult records the result of an arm pull
func (s *Server) handleRecordResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	armID := vars["armID"]

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update arm statistics
	query := `
		UPDATE arms
		SET 
			successes = CASE WHEN $1 THEN successes + 1 ELSE successes END,
			failures = CASE WHEN $1 THEN failures ELSE failures + 1 END,
			updated_at = NOW()
		WHERE id = $2
		RETURNING successes, failures`

	var successes, failures int
	err := s.db.QueryRow(query, result.Success, armID).Scan(&successes, &failures)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "arm not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"successes": successes,
		"failures":  failures,
	})
}

// thompsonSampling implements the Thompson Sampling algorithm
func thompsonSampling(arms []models.Arm) models.Arm {
	rand.Seed(time.Now().UnixNano())

	var (
		maxSample float64
		selected  models.Arm
	)

	for _, arm := range arms {
		// Sample from beta distribution using conjugate priors
		sample := betaDistribution(float64(arm.Successes+1), float64(arm.Failures+1))

		if sample > maxSample {
			maxSample = sample
			selected = arm
		}
	}

	return selected
}

// betaDistribution generates a random sample from a beta distribution
func betaDistribution(alpha, beta float64) float64 {
	x := gammaDistribution(alpha)
	y := gammaDistribution(beta)
	return x / (x + y)
}

// gammaDistribution generates a random sample from a gamma distribution
func gammaDistribution(alpha float64) float64 {
	if alpha < 1 {
		return gammaDistribution(1+alpha) * math.Pow(rand.Float64(), 1/alpha)
	}

	d := alpha - 1/3
	c := 1 / math.Sqrt(9*d)

	for {
		x := rand.NormFloat64()
		v := 1 + c*x
		v = v * v * v
		u := rand.Float64()

		if u < 1-0.331*math.Pow(x, 4) ||
			math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

func main() {
	// Connect to database
	db, err := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create server
	server := NewServer(db)

	// Start server
	log.Fatal(http.ListenAndServe(":8080", server.router))
}
