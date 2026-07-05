// Package main provides the assignment API server
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"gobandit/internal/handler/assignment"
	"gobandit/internal/repository/postgres"
	"gobandit/internal/service"
)

func main() {
	// Connect to database
	db, err := sql.Open("postgres", "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Create repositories
	testRepo := postgres.NewTestRepository(db)
	armRepo := postgres.NewArmRepository(db)

	// Create services
	thompsonSvc := service.NewThompsonSampling()
	experimentSvc := service.NewExperimentService(testRepo, armRepo, thompsonSvc)

	// Create handler
	assignmentHandler := assignment.NewHandler(experimentSvc)

	// Setup router
	router := mux.NewRouter()
	assignmentHandler.RegisterRoutes(router)

	// Create server
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Assignment API server starting on port 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down assignment API server...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Assignment API server forced to shutdown:", err)
	}

	log.Println("Assignment API server exited")
}
