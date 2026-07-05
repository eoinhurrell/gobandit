package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"gobandit/internal/handler/assignment"
	"gobandit/internal/handler/ui"
	"gobandit/internal/repository/postgres"
	"gobandit/internal/service"
)

// Server encapsulates the MAB server state using the new architecture
type Server struct {
	db     *sql.DB
	router *mux.Router
}

// NewServer creates a new MAB server instance using refactored components
func NewServer(db *sql.DB) *Server {
	// Create repositories
	testRepo := postgres.NewTestRepository(db)
	armRepo := postgres.NewArmRepository(db)

	// Create services
	thompsonSvc := service.NewThompsonSampling()
	experimentSvc := service.NewExperimentService(testRepo, armRepo, thompsonSvc)

	// Create handlers
	assignmentHandler := assignment.NewHandler(experimentSvc)
	uiHandler := ui.NewHandler(experimentSvc)

	// Setup router
	router := mux.NewRouter()

	// Register routes from both handlers
	assignmentHandler.RegisterRoutes(router)
	uiHandler.RegisterRoutes(router)

	return &Server{
		db:     db,
		router: router,
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
