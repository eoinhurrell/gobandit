// Package ui provides HTTP handlers for the web UI
package ui

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"gobandit/internal/domain/experiment"
	"gobandit/models"
	"gobandit/templates"
)

// Handler handles web UI requests
type Handler struct {
	experimentSvc experiment.Service
}

// NewHandler creates a new UI handler
func NewHandler(experimentSvc experiment.Service) *Handler {
	return &Handler{
		experimentSvc: experimentSvc,
	}
}

// RegisterRoutes registers the web UI routes
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.Dashboard).Methods("GET")
	router.HandleFunc("/tests", h.CreateTest).Methods("POST")
	router.HandleFunc("/tests/{testID}/arms", h.GetArmStats).Methods("GET")
}

// Dashboard renders the main dashboard
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	tests, err := h.experimentSvc.GetAllTests(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert domain tests to models.Test for template compatibility
	modelTests := make([]models.Test, len(tests))
	for i, test := range tests {
		modelTests[i] = convertDomainTestToModel(test)
	}

	fmt.Printf("Dashboard tests: %d\n", len(modelTests))
	if err := templates.Dashboard(modelTests).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// CreateTest creates a new test with the specified arms
func (h *Handler) CreateTest(w http.ResponseWriter, r *http.Request) {
	// Parse the form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	numArmsStr := r.FormValue("numArms")

	numArms, err := strconv.Atoi(numArmsStr)
	if err != nil {
		http.Error(w, "Invalid number of arms", http.StatusBadRequest)
		return
	}

	test, err := h.experimentSvc.CreateTest(r.Context(), name, description, numArms)
	if err != nil {
		fmt.Printf("Error creating test: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Created test with ID: %s, Name: %s\n", test.ID, test.Name)

	// Set appropriate headers for HTMX response
	w.Header().Set("Content-Type", "text/html")

	// Convert to models.Test for template compatibility
	modelTest := convertDomainTestToModel(*test)

	// Render the template component
	if err := templates.TestCard(modelTest).Render(r.Context(), w); err != nil {
		fmt.Printf("Error rendering template: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetArmStats returns stats for all arms in a test
func (h *Handler) GetArmStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["testID"]

	arms, err := h.experimentSvc.GetArmStats(r.Context(), testID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert domain arms to models.Arm for template compatibility
	modelArms := make([]models.Arm, len(arms))
	for i, arm := range arms {
		modelArms[i] = convertDomainArmToModel(arm)
	}

	if err := templates.ArmStats(modelArms).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Helper functions to convert between domain and models for template compatibility

func convertDomainTestToModel(domainTest experiment.Test) models.Test {
	modelArms := make([]models.Arm, len(domainTest.Arms))
	for i, arm := range domainTest.Arms {
		modelArms[i] = convertDomainArmToModel(arm)
	}

	return models.Test{
		ID:          domainTest.ID,
		Name:        domainTest.Name,
		Description: domainTest.Description,
		Arms:        modelArms,
		CreatedAt:   domainTest.CreatedAt,
		UpdatedAt:   domainTest.UpdatedAt,
	}
}

func convertDomainArmToModel(domainArm experiment.Arm) models.Arm {
	return models.Arm{
		ID:          domainArm.ID,
		TestID:      domainArm.TestID,
		Name:        domainArm.Name,
		Description: domainArm.Description,
		Successes:   domainArm.Successes,
		Failures:    domainArm.Failures,
		CreatedAt:   domainArm.CreatedAt,
		UpdatedAt:   domainArm.UpdatedAt,
	}
}
