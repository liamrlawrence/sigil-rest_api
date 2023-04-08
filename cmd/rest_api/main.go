package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/liamrlawrence/sigil-rest_api/internal/database"
	"github.com/liamrlawrence/sigil-rest_api/internal/routes"
	"github.com/liamrlawrence/sigil-rest_api/internal/server"
	"net/http"
	"path/filepath"
)

func initialize(s *server.Server) error {
	// load environment variables
	var envFiles = [...]string{"database.env", "jenkins.env", "openai.env"}

	var err error
	for _, ef := range envFiles {
		err = godotenv.Load(filepath.Join("envs", ef))
		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
	}

	// connect to databases
	s.DBPool, err = database.ConnectToDatabase()
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// setup middleware to check for session tokens
	s.Router.Use(routes.AuthTokenMiddleware(s))

	// setup endpoints for routes
	routes.SetupEndpoints(s)
	return nil
}

func run(s *server.Server) error {
	// Start the HTTPS server
	fmt.Println("Starting the server on :8000...")
	return http.ListenAndServeTLS(
		":8000",
		"/app/certs/live/necronomicon.network/fullchain.pem",
		"/app/certs/live/necronomicon.network/privkey.pem",
		s.Router)
}

func main() {
	s := server.Server{
		DBPool: nil,
		Router: chi.NewRouter(),
	}

	err := initialize(&s)
	if err != nil {
		panic(err)
	}

	err = run(&s)
	if err != nil {
		panic(err)
	}
}
