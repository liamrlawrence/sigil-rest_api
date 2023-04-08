package routes

import (
	"fmt"
	"github.com/liamrlawrence/sigil-rest_api/internal/logging"
	"github.com/liamrlawrence/sigil-rest_api/internal/server"
	"net/http"
)

func SetupEndpoints(s *server.Server) {
	// Services
	s.Router.Get("/api/heartbeat", HandlerRouteHeartbeat(s))
	s.Router.Post("/api/heartbeat", HandlerRouteHeartbeat(s))

	// auth
	//s.Router.Post("/api/auth/sign-up", HandlerRouteAuthSignUp(s)) // in: username, email, 2xpassword
	s.Router.Post("/api/auth/login", HandlerRouteAuthLogIn(s))
	s.Router.Post("/api/auth/refresh", HandlerRouteAuthRefresh(s))

	// ai
	s.Router.Get("/api/ai/bills", HandlerRouteAIBills(s))
	s.Router.Post("/api/ai/gpt3", HandlerRouteChatGPT35_Turbo(s))
	s.Router.Post("/api/ai/gpt4", HandlerRouteChatGPT4(s))

	// docker
	s.Router.Get("/api/boto/logs", HandlerRouteBotoLogs(s))

	// 404
	s.Router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Page not found", http.StatusNotFound)
	})
}

func HandlerRouteHeartbeat(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logging.APIEndpoint(r, r.Method, "/api/heartbeat")

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, `[{
	"status": "success",
	"message": "doki doki"
}]`)
		return
	}
}
