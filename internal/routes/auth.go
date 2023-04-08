package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/liamrlawrence/sigil-rest_api/internal/logging"
	"github.com/liamrlawrence/sigil-rest_api/internal/server"
	"log"
	"net/http"
)

func AuthTokenMiddleware(s *server.Server) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication on these routes
			switch r.URL.Path {
			case "/api/auth/login":
				next.ServeHTTP(w, r)
				return
			}

			// Authenticate
			sessionID := r.Header.Get("X-Grimoire-Token")
			if sessionID == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprintf(w, `{
	"status": "failed",
	"message": "missing session id"
}`)
				return
			}

			row, err := s.DBPool.Query(context.Background(), "SELECT * FROM Auth.FN_Valid_Session($1::UUID);", sessionID)
			if err != nil {
				log.Fatalf("Failed to execute query: %v", err)
			}
			valid := row.Next()
			row.Close()

			if !valid {
				if err := row.Err(); err != nil {
					log.Printf("/api/auth/refresh | %v\n", err)

					switch err.Error() {
					case "ERROR: invalid session id (SQLSTATE P0001)":
						w.WriteHeader(http.StatusBadRequest)
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						fmt.Fprint(w, `{
	"status": "failed",
	"message": "invalid session id"
}`)
						return

					case "ERROR: session id is expired (SQLSTATE P0001)":
						if r.URL.Path == "/api/auth/refresh" {
							next.ServeHTTP(w, r)
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						fmt.Fprint(w, `{
	"status": "failed",
	"message": "session id is expired"
}`)
						return

					case "ERROR: refresh token is expired (SQLSTATE P0001)":
						w.WriteHeader(http.StatusBadRequest)
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						fmt.Fprint(w, `{
	"status": "failed",
	"message": "refresh token is expired"
}`)
						return

					default:
						w.WriteHeader(http.StatusBadRequest)
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						fmt.Fprint(w, `{
	"status": "failed",
	"message": "Unknown error"
}`)
						return
					}
				}
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func HandlerRouteAuthRefresh(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			SessionID    string `json:"session_id"`
			RefreshToken string `json:"refresh_token"`
		}

		var requestBody RequestBody
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Authenticate
		sessionID := r.Header.Get("X-Grimoire-Token")
		if sessionID == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}
		logging.APIEndpoint(r, "POST", fmt.Sprintf("/api/auth/refresh"))
		refreshToken := requestBody.RefreshToken

		row, err := s.DBPool.Query(context.Background(), "SELECT Auth.FN_Refresh_Session($1::UUID, $2::UUID);", sessionID, refreshToken)
		if err != nil {
			log.Fatalf("database query failed: %v", err)
		}
		defer row.Close()

		if row.Next() {
			var newRefreshToken string
			err = row.Scan(&newRefreshToken)
			if err != nil {
				log.Fatalf("Failed to scan row: %v", err)
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprintf(w, `{
	"status": "success",
	"message": "session refreshed",
	"data": {
		"refresh_token": "%v"
	}
}`,
				newRefreshToken)
		}

		if err := row.Err(); err != nil {
			log.Printf("/api/auth/login | %v\n", err)
			switch err.Error() {
			case "ERROR: invalid refresh token (SQLSTATE P0001)":
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprint(w, `{
	"status": "failed",
	"message": "invalid refresh token"
}`)
				return

			default:
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprint(w, `{
	"status": "failed",
	"message": "Unknown error"
}`)
				return
			}
		}
	}
}

func HandlerRouteAuthLogIn(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		var requestBody RequestBody
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		username := requestBody.Username
		password := requestBody.Password
		logging.APIEndpoint(r, "POST", fmt.Sprintf("/api/auth/login - %s", username))

		row, err := s.DBPool.Query(context.Background(), "SELECT session_id, refresh_token FROM Auth.FN_User_LogIn($1, $2);", username, password)
		if err != nil {
			log.Fatalf("database query failed: %v", err)
		}
		defer row.Close()

		if row.Next() {
			var sessionID string
			var refreshToken string
			err = row.Scan(&sessionID, &refreshToken)
			if err != nil {
				log.Fatalf("Failed to scan row: %v", err)
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprintf(w, `{
	"status": "success",
	"message": "signed in",
	"data": {
		"session_id": "%v",
		"refresh_token": "%v"
	}
}`,
				sessionID, refreshToken)
		}

		if err := row.Err(); err != nil {
			log.Printf("/api/auth/login | %v\n", err)
			switch err.Error() {
			case "ERROR: incorrect username and password (SQLSTATE P0001)":
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprint(w, `{
	"status": "failed",
	"message": "incorrect username or password"
}`)
			default:
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprint(w, `{
	"status": "failed",
	"message": "Unknown error"
}`)
			}
		}
	}
}
