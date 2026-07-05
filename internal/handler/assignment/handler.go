// Package assignment provides HTTP handlers for the assignment API
package assignment

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"gobandit/internal/domain/experiment"
)

// Handler handles assignment API requests
type Handler struct {
	experimentSvc experiment.Service
}

// NewHandler creates a new assignment handler
func NewHandler(experimentSvc experiment.Service) *Handler {
	return &Handler{
		experimentSvc: experimentSvc,
	}
}

// RegisterRoutes registers the assignment API routes
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/tests/{testID}/arm", h.GetArm).Methods("GET")
	router.HandleFunc("/tests/{testID}/arms/{armID}/result", h.RecordResult).Methods("POST")
}

// GetArm returns the next arm to test using Thompson Sampling
func (h *Handler) GetArm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	testID := vars["testID"]

	arm, err := h.experimentSvc.GetArmAssignment(r.Context(), testID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(arm); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

// RecordResult records the result of an arm pull
func (h *Handler) RecordResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	armID := vars["armID"]

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	arm, err := h.experimentSvc.RecordResult(r.Context(), armID, result.Success)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"successes": arm.Successes,
		"failures":  arm.Failures,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
