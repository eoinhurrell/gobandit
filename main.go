package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	. "gobandit/models"

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
}

// handleCreateTest creates a new test with the specified arms
func (s *Server) handleCreateTest(w http.ResponseWriter, r *http.Request) {
	var test Test
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

	var arms []Arm
	for rows.Next() {
		var arm Arm
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
func thompsonSampling(arms []Arm) Arm {
	rand.Seed(time.Now().UnixNano())

	var (
		maxSample float64
		selected  Arm
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
